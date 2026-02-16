package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hra42/go-web-fetch-mcp/internal/config"
)

type Response struct {
	StatusCode  int
	Headers     http.Header
	Body        []byte
	ContentType string
}

type Fetcher struct {
	client    *http.Client
	userAgent string
}

func NewFetcher(cfg *config.Config) *Fetcher {
	transport := &http.Transport{}

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &Fetcher{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		userAgent: cfg.UserAgent,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", f.userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return &Response{
		StatusCode:  resp.StatusCode,
		Headers:     resp.Header,
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}
