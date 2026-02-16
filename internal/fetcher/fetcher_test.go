package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hra42/go-web-fetch-mcp/internal/config"
)

func TestSuccessfulFetch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<h1>Hello</h1>"))
	}))
	defer ts.Close()

	f := NewFetcher(&config.Config{UserAgent: "TestAgent/1.0"})
	resp, err := f.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if string(resp.Body) != "<h1>Hello</h1>" {
		t.Errorf("Body = %q, want %q", string(resp.Body), "<h1>Hello</h1>")
	}
	if !strings.HasPrefix(resp.ContentType, "text/html") {
		t.Errorf("ContentType = %q, want text/html prefix", resp.ContentType)
	}
	if resp.Headers.Get("X-Custom") != "test-value" {
		t.Errorf("X-Custom header = %q, want %q", resp.Headers.Get("X-Custom"), "test-value")
	}
}

func TestCustomUserAgent(t *testing.T) {
	var receivedUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	customUA := "MyCustomAgent/2.0"
	f := NewFetcher(&config.Config{UserAgent: customUA})
	_, err := f.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedUA != customUA {
		t.Errorf("User-Agent = %q, want %q", receivedUA, customUA)
	}
}

func TestRedirects(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("final page"))
	}))
	defer ts.Close()

	f := NewFetcher(&config.Config{UserAgent: "TestAgent/1.0"})
	resp, err := f.Fetch(context.Background(), ts.URL+"/redirect")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200 after redirect", resp.StatusCode)
	}
	if string(resp.Body) != "final page" {
		t.Errorf("Body = %q, want %q", string(resp.Body), "final page")
	}
}

func TestHTTPErrorStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"403 Forbidden", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("error body"))
			}))
			defer ts.Close()

			f := NewFetcher(&config.Config{UserAgent: "TestAgent/1.0"})
			_, err := f.Fetch(context.Background(), ts.URL)
			if err == nil {
				t.Fatal("expected error for HTTP error status")
			}
			if !strings.Contains(err.Error(), "HTTP") {
				t.Errorf("error = %q, want it to contain 'HTTP'", err.Error())
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &config.Config{UserAgent: "TestAgent/1.0"}
	f := NewFetcher(cfg)
	// Override the client timeout for the test
	f.client.Timeout = 50 * time.Millisecond

	_, err := f.Fetch(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestProxyConfiguration(t *testing.T) {
	cfg := &config.Config{
		UserAgent: "TestAgent/1.0",
		ProxyURL:  "http://proxy.example.com:8080",
	}
	f := NewFetcher(cfg)

	transport, ok := f.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if transport.Proxy == nil {
		t.Fatal("expected proxy function to be set")
	}

	// Verify the proxy function returns the configured URL
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("proxy function error: %v", err)
	}
	if proxyURL.String() != "http://proxy.example.com:8080" {
		t.Errorf("proxy URL = %q, want %q", proxyURL.String(), "http://proxy.example.com:8080")
	}
}

func TestContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f := NewFetcher(&config.Config{UserAgent: "TestAgent/1.0"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := f.Fetch(ctx, ts.URL)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
