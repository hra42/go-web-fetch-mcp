package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hra42/go-web-fetch-mcp/internal/config"
	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
	"github.com/hra42/go-web-fetch-mcp/internal/processor"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockRobotsChecker implements RobotsChecker for testing.
type mockRobotsChecker struct {
	allowed bool
	err     error
}

func (m *mockRobotsChecker) IsAllowed(_ context.Context, _, _ string) (bool, error) {
	return m.allowed, m.err
}

// newTestServer creates a Server backed by an httptest server.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*Server, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)

	cfg := config.DefaultConfig()
	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := NewServer(cfg, f, p)

	return srv, ts
}

func callFetch(t *testing.T, srv *Server, args FetchArgs) *mcp.CallToolResult {
	t.Helper()
	result, _, err := srv.handleFetch(context.Background(), &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("handleFetch returned unexpected error: %v", err)
	}
	return result
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestValidFetch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Test Page</title></head>
<body>
<article>
<h1>Test Heading</h1>
<p>This is a test paragraph with enough content for readability to extract it.
We need multiple sentences to ensure the article extraction works properly.
Adding more text here to make the content substantial enough for processing.</p>
<p>A second paragraph helps readability identify this as article content.
More text is always better for the extraction algorithm to work correctly.</p>
</article>
</body></html>`))
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := NewServer(cfg, f, p)

	result := callFetch(t, srv, FetchArgs{URL: ts.URL})
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Title:") {
		t.Errorf("expected title in output, got: %s", text)
	}
	if !strings.Contains(text, "Test") {
		t.Errorf("expected content in output, got: %s", text)
	}
}

func TestEmptyURL(t *testing.T) {
	srv, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer ts.Close()

	result := callFetch(t, srv, FetchArgs{URL: ""})
	if !result.IsError {
		t.Fatal("expected error result for empty URL")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "URL is required") {
		t.Errorf("error = %q, want it to contain 'URL is required'", text)
	}
}

func TestInvalidURL(t *testing.T) {
	srv, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer ts.Close()

	tests := []struct {
		name string
		url  string
	}{
		{"no scheme", "example.com"},
		{"no host", "http://"},
		{"just path", "/path/to/something"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := callFetch(t, srv, FetchArgs{URL: tt.url})
			if !result.IsError {
				t.Fatal("expected error result for invalid URL")
			}
			text := resultText(t, result)
			if !strings.Contains(text, "Invalid URL") {
				t.Errorf("error = %q, want it to contain 'Invalid URL'", text)
			}
		})
	}
}

func TestRobotsBlocked(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := NewServer(cfg, f, p)
	srv.SetRobotsChecker(&mockRobotsChecker{allowed: false})

	result := callFetch(t, srv, FetchArgs{URL: ts.URL + "/secret"})
	if !result.IsError {
		t.Fatal("expected error result for robots-blocked URL")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "robots.txt") {
		t.Errorf("error = %q, want it to mention robots.txt", text)
	}
}

func TestRawMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Raw Test</title></head>
<body>
<nav><a href="/a">Link A</a><a href="/b">Link B</a></nav>
<article>
<h1>Article Title</h1>
<p>This is a substantial article paragraph that readability should extract.
It has enough content for the readability algorithm to identify it as main content.
We add extra sentences to ensure this block is considered substantial.</p>
<p>Second paragraph with more text so readability treats this as the article.
The more content here, the better the extraction works for our testing.</p>
</article>
<footer>Footer content here</footer>
</body></html>`))
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := NewServer(cfg, f, p)

	// Fetch with raw=true — should include nav/footer content
	rawResult := callFetch(t, srv, FetchArgs{URL: ts.URL, Raw: true})
	if rawResult.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, rawResult))
	}

	// Fetch with raw=false — readability should strip nav/footer
	normalResult := callFetch(t, srv, FetchArgs{URL: ts.URL, Raw: false})
	if normalResult.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, normalResult))
	}

	rawText := resultText(t, rawResult)
	normalText := resultText(t, normalResult)

	// Raw mode should include nav content that readability strips
	if !strings.Contains(rawText, "Link A") {
		t.Error("raw mode should include nav links")
	}
	// Normal mode should have a title extracted by readability
	if !strings.Contains(normalText, "Title:") {
		t.Error("normal mode should include extracted title")
	}
}

func TestFetchError(t *testing.T) {
	cfg := config.DefaultConfig()
	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := NewServer(cfg, f, p)

	// Use an unreachable URL
	result := callFetch(t, srv, FetchArgs{URL: "http://127.0.0.1:1/unreachable"})
	if !result.IsError {
		t.Fatal("expected error result for unreachable URL")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Failed to fetch") {
		t.Errorf("error = %q, want it to contain 'Failed to fetch'", text)
	}
}

func TestPlainTextFetch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Hello, plain text!"))
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	f := fetcher.NewFetcher(cfg)
	p := processor.NewProcessor()
	srv := NewServer(cfg, f, p)

	result := callFetch(t, srv, FetchArgs{URL: ts.URL})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Hello, plain text!") {
		t.Errorf("expected plain text in output, got: %s", text)
	}
}
