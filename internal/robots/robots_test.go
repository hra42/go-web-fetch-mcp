package robots

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hra42/go-web-fetch-mcp/internal/config"
	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
)

const testUA = "TestBot/1.0"

func newTestFetcher() *fetcher.Fetcher {
	return fetcher.NewFetcher(&config.Config{UserAgent: testUA})
}

func TestAllowedURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer ts.Close()

	c := NewChecker(newTestFetcher())
	allowed, err := c.IsAllowed(context.Background(), ts.URL+"/page", testUA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected URL to be allowed")
	}
}

func TestDisallowedURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /secret\n"))
	}))
	defer ts.Close()

	c := NewChecker(newTestFetcher())
	allowed, err := c.IsAllowed(context.Background(), ts.URL+"/secret/data", testUA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected URL to be disallowed")
	}
}

func TestMissingRobotsTxt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	c := NewChecker(newTestFetcher())
	allowed, err := c.IsAllowed(context.Background(), ts.URL+"/anything", testUA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected URL to be allowed when robots.txt is missing")
	}
}

func TestFetchError(t *testing.T) {
	// Use an unreachable server
	c := NewChecker(newTestFetcher())
	allowed, err := c.IsAllowed(context.Background(), "http://127.0.0.1:1/path", testUA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected URL to be allowed on fetch error")
	}
}

func TestCacheHit(t *testing.T) {
	var fetchCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer ts.Close()

	c := NewChecker(newTestFetcher())

	// First call — fetches robots.txt
	_, _ = c.IsAllowed(context.Background(), ts.URL+"/a", testUA)
	// Second call — should use cache
	_, _ = c.IsAllowed(context.Background(), ts.URL+"/b", testUA)

	// robots.txt is at /robots.txt, but the test server serves everything the same.
	// Both calls target the same origin, so only 1 fetch for robots.txt should occur.
	if got := fetchCount.Load(); got != 1 {
		t.Fatalf("expected 1 fetch, got %d", got)
	}
}

func TestCacheExpiry(t *testing.T) {
	var fetchCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer ts.Close()

	c := NewChecker(newTestFetcher())
	c.ttl = 10 * time.Millisecond

	_, _ = c.IsAllowed(context.Background(), ts.URL+"/a", testUA)
	time.Sleep(20 * time.Millisecond)
	_, _ = c.IsAllowed(context.Background(), ts.URL+"/b", testUA)

	if got := fetchCount.Load(); got != 2 {
		t.Fatalf("expected 2 fetches after expiry, got %d", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer ts.Close()

	c := NewChecker(newTestFetcher())

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			allowed, err := c.IsAllowed(context.Background(), ts.URL+"/page", testUA)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", i, err)
			}
			if !allowed {
				t.Errorf("goroutine %d: expected allowed", i)
			}
		}(i)
	}
	wg.Wait()
}
