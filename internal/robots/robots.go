package robots

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/jimsmart/grobotstxt"

	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
)

type cacheEntry struct {
	body      string
	fetchedAt time.Time
}

// Checker implements server.RobotsChecker using grobotstxt.
type Checker struct {
	fetcher *fetcher.Fetcher
	cache   map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

// NewChecker creates a robots.txt checker with a 1-hour cache TTL.
func NewChecker(f *fetcher.Fetcher) *Checker {
	return &Checker{
		fetcher: f,
		cache:   make(map[string]*cacheEntry),
		ttl:     1 * time.Hour,
	}
}

// IsAllowed checks whether the given URL is allowed by robots.txt for the specified user agent.
// On fetch errors or missing robots.txt (404), all URLs are allowed.
func (c *Checker) IsAllowed(ctx context.Context, rawURL, userAgent string) (bool, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Errorf("parsing URL: %w", err)
	}

	origin := parsed.Scheme + "://" + parsed.Host
	robotsURL := origin + "/robots.txt"

	// Check cache with read lock
	c.mu.RLock()
	entry, ok := c.cache[origin]
	if ok && time.Since(entry.fetchedAt) < c.ttl {
		body := entry.body
		c.mu.RUnlock()
		return grobotstxt.AgentAllowed(body, userAgent, rawURL), nil
	}
	c.mu.RUnlock()

	// Cache miss or expired — fetch robots.txt
	body, err := c.fetchRobotsTxt(ctx, robotsURL)
	if err != nil {
		// On any fetch error, allow all and cache empty string
		body = ""
	}

	c.mu.Lock()
	c.cache[origin] = &cacheEntry{
		body:      body,
		fetchedAt: time.Now(),
	}
	c.mu.Unlock()

	return grobotstxt.AgentAllowed(body, userAgent, rawURL), nil
}

// fetchRobotsTxt fetches the robots.txt content. Returns empty string on 404 or errors.
func (c *Checker) fetchRobotsTxt(ctx context.Context, robotsURL string) (string, error) {
	resp, err := c.fetcher.Fetch(ctx, robotsURL)
	if err != nil {
		return "", err
	}
	return string(resp.Body), nil
}
