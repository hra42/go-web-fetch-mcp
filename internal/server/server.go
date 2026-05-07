package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hra42/go-web-fetch-mcp/internal/config"
	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
	"github.com/hra42/go-web-fetch-mcp/internal/processor"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RobotsChecker validates whether a URL is allowed by robots.txt.
// When nil on the Server, all URLs are allowed.
type RobotsChecker interface {
	IsAllowed(ctx context.Context, url, userAgent string) (bool, error)
}

// FetchArgs is the input schema for the fetch tool, auto-inferred by the SDK.
type FetchArgs struct {
	URL        string `json:"url" jsonschema:"The URL to fetch"`
	MaxLength  int    `json:"max_length,omitempty" jsonschema:"Maximum content length (default 5000)"`
	StartIndex int    `json:"start_index,omitempty" jsonschema:"Byte offset for pagination (default 0)"`
	Raw        bool   `json:"raw,omitempty" jsonschema:"Skip readability and return raw content (default false)"`
}

// Server wraps the MCP server with fetcher and processor dependencies.
type Server struct {
	mcpServer     *mcp.Server
	fetcher       *fetcher.Fetcher
	processor     *processor.Processor
	cfg           *config.Config
	robotsChecker RobotsChecker
}

// NewServer creates an MCP server with the fetch tool registered.
func NewServer(cfg *config.Config, f *fetcher.Fetcher, p *processor.Processor) *Server {
	s := &Server{
		fetcher:   f,
		processor: p,
		cfg:       cfg,
	}

	s.mcpServer = mcp.NewServer(
		&mcp.Implementation{Name: "web-fetch-mcp", Version: "1.0.0"},
		nil,
	)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "fetch",
		Description: "Fetches a URL from the internet and extracts its contents as markdown",
	}, s.handleFetch)

	return s
}

// SetRobotsChecker sets an optional robots.txt checker. When set, URLs
// disallowed by robots.txt will be rejected.
func (s *Server) SetRobotsChecker(rc RobotsChecker) {
	s.robotsChecker = rc
}

// handleFetch is the tool handler for the fetch tool.
func (s *Server) handleFetch(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args FetchArgs,
) (*mcp.CallToolResult, struct{}, error) {
	var empty struct{}

	// Validate URL
	if strings.TrimSpace(args.URL) == "" {
		return errorResult("URL is required"), empty, nil
	}
	parsedURL, err := url.Parse(args.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errorResult(fmt.Sprintf("Invalid URL: %s", args.URL)), empty, nil
	}

	// Robots.txt check
	if s.robotsChecker != nil {
		allowed, err := s.robotsChecker.IsAllowed(ctx, args.URL, s.cfg.UserAgent)
		if err != nil {
			return errorResult(fmt.Sprintf("robots.txt check failed: %v", err)), empty, nil
		}
		if !allowed {
			return errorResult(fmt.Sprintf("URL blocked by robots.txt: %s", args.URL)), empty, nil
		}
	}

	// Fetch
	resp, err := s.fetcher.Fetch(ctx, args.URL)
	if err != nil {
		return errorResult(fmt.Sprintf("Failed to fetch %s: %v", args.URL, err)), empty, nil
	}

	// Process
	result, err := s.processor.Process(resp, args.URL, args.StartIndex, args.MaxLength, args.Raw)
	if err != nil {
		return errorResult(fmt.Sprintf("Failed to process content: %v", err)), empty, nil
	}

	// Format output
	var text strings.Builder
	if result.Title != "" {
		text.WriteString("Title: ")
		text.WriteString(result.Title)
		text.WriteString("\n\n")
	}
	text.WriteString(result.Content)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text.String()},
		},
	}, empty, nil
}

// errorResult creates a CallToolResult with IsError set.
func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
		IsError: true,
	}
}

// Run starts the MCP server on stdio transport, blocking until the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// HTTPHandler returns an http.Handler serving the MCP streamable HTTP transport,
// wrapped in Bearer-token auth using the provided token. The token must be non-empty.
//
// The returned handler also exposes a GET /healthz endpoint that bypasses auth,
// for liveness probes from reverse proxies / load balancers.
func (s *Server) HTTPHandler(token string) (http.Handler, error) {
	if token == "" {
		return nil, errors.New("auth token must not be empty for HTTP transport")
	}
	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.mcpServer },
		&mcp.StreamableHTTPOptions{Logger: slog.Default()},
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", bearerAuth(token, mcpHandler))
	return mux, nil
}

// RunHTTP starts the MCP server on the streamable HTTP transport at addr,
// authenticated with the given Bearer token. It blocks until ctx is cancelled
// and then performs a graceful shutdown.
func (s *Server) RunHTTP(ctx context.Context, addr, token string) error {
	handler, err := s.HTTPHandler(token)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr, "transport", "http")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// bearerAuth wraps an http.Handler, requiring a matching "Authorization: Bearer <token>" header.
func bearerAuth(token string, next http.Handler) http.Handler {
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="web-fetch-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
