package server

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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
