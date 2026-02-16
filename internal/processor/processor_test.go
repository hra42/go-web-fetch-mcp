package processor

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
)

func makeResponse(contentType string, body string) *fetcher.Response {
	return &fetcher.Response{
		StatusCode:  200,
		Headers:     http.Header{"Content-Type": {contentType}},
		Body:        []byte(body),
		ContentType: contentType,
	}
}

func TestContentTypeRouting(t *testing.T) {
	p := NewProcessor()

	tests := []struct {
		name        string
		contentType string
		body        string
		wantInBody  string
		wantMIME    string
	}{
		{
			name:        "HTML is processed",
			contentType: "text/html; charset=utf-8",
			body:        "<html><body><article><h1>Hello</h1><p>World</p></article></body></html>",
			wantInBody:  "Hello",
			wantMIME:    "text/html",
		},
		{
			name:        "plain text passed through",
			contentType: "text/plain",
			body:        "just some text",
			wantInBody:  "just some text",
			wantMIME:    "text/plain",
		},
		{
			name:        "JSON passed through",
			contentType: "application/json",
			body:        `{"key":"value"}`,
			wantInBody:  `{"key":"value"}`,
			wantMIME:    "application/json",
		},
		{
			name:        "binary content placeholder",
			contentType: "image/png",
			body:        "\x89PNG\r\n",
			wantInBody:  "[Binary content: image/png,",
			wantMIME:    "image/png",
		},
		{
			name:        "empty content type defaults",
			contentType: "",
			body:        "\x00\x01\x02",
			wantInBody:  "[Binary content: application/octet-stream,",
			wantMIME:    "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := makeResponse(tt.contentType, tt.body)
			result, err := p.Process(resp, "https://example.com", 0, 10000, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(result.Content, tt.wantInBody) {
				t.Errorf("content = %q, want it to contain %q", result.Content, tt.wantInBody)
			}
			if result.MIMEType != tt.wantMIME {
				t.Errorf("MIMEType = %q, want %q", result.MIMEType, tt.wantMIME)
			}
		})
	}
}

func TestHTMLProcessing(t *testing.T) {
	p := NewProcessor()

	t.Run("well-formed article", func(t *testing.T) {
		html := `<!DOCTYPE html>
<html><head><title>Test Article</title></head>
<body>
<article>
<h1>Main Heading</h1>
<p>This is a paragraph with enough content for readability to consider it an article.
It needs to be long enough so that readability does not discard it as too short.
Let us add more text here to make sure the extraction works properly.
This paragraph is quite important for the content extraction algorithm.</p>
<p>Another paragraph with additional detail to ensure the article is substantial.
We want to make sure readability picks this up as the main content.</p>
</article>
<nav><a href="/other">Other</a></nav>
</body></html>`

		resp := makeResponse("text/html", html)
		result, err := p.Process(resp, "https://example.com", 0, 10000, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result.Content, "Main Heading") {
			t.Errorf("content should contain article heading, got: %s", result.Content)
		}
		if !strings.Contains(result.Content, "paragraph") {
			t.Errorf("content should contain article text, got: %s", result.Content)
		}
	})

	t.Run("readability fallback for nav-only page", func(t *testing.T) {
		html := `<html><body><nav><a href="/a">A</a><a href="/b">B</a></nav></body></html>`
		resp := makeResponse("text/html", html)
		result, err := p.Process(resp, "https://example.com", 0, 10000, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should still produce some output (markdown conversion of raw HTML)
		if result.Content == "" {
			t.Error("expected non-empty content from fallback")
		}
	})

	t.Run("relative link resolution", func(t *testing.T) {
		html := `<html><body><article>
<p>This is a long enough article paragraph for readability to extract it properly.
We need enough content here to ensure readability considers this substantial.</p>
<p>Here is a <a href="/page">relative link</a> that should be resolved to an absolute URL.</p>
<p>More content to pad this out so readability picks up this article block.</p>
</article></body></html>`

		resp := makeResponse("text/html", html)
		result, err := p.Process(resp, "https://example.com", 0, 10000, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result.Content, "https://example.com/page") {
			t.Errorf("expected resolved absolute URL, got: %s", result.Content)
		}
	})
}

func TestPagination(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		startIndex int
		maxLength  int
		wantMore   bool
		checkFn    func(t *testing.T, pc *ProcessedContent)
	}{
		{
			name:       "no truncation needed",
			content:    "short",
			startIndex: 0,
			maxLength:  100,
			wantMore:   false,
			checkFn: func(t *testing.T, pc *ProcessedContent) {
				if pc.Content != "short" {
					t.Errorf("content = %q, want %q", pc.Content, "short")
				}
			},
		},
		{
			name:       "truncation with hint",
			content:    "Hello, World! This is a longer piece of content.",
			startIndex: 0,
			maxLength:  13,
			wantMore:   true,
			checkFn: func(t *testing.T, pc *ProcessedContent) {
				if !strings.Contains(pc.Content, "Hello, World!") {
					t.Errorf("should contain beginning of content")
				}
				if !strings.Contains(pc.Content, "start_index=13") {
					t.Errorf("should contain continuation hint, got: %s", pc.Content)
				}
			},
		},
		{
			name:       "start past end",
			content:    "short",
			startIndex: 100,
			maxLength:  50,
			wantMore:   false,
			checkFn: func(t *testing.T, pc *ProcessedContent) {
				if pc.Content != "" {
					t.Errorf("content = %q, want empty", pc.Content)
				}
				if pc.StartIndex != 5 {
					t.Errorf("startIndex = %d, want 5", pc.StartIndex)
				}
			},
		},
		{
			name:       "multi-byte UTF-8 at boundary",
			content:    "abc\xc3\xa9def", // "abcédef" — é is 2 bytes (0xc3 0xa9)
			startIndex: 0,
			maxLength:  4, // would cut in the middle of é
			wantMore:   true,
			checkFn: func(t *testing.T, pc *ProcessedContent) {
				// Should snap back to avoid cutting the multi-byte char
				if strings.Contains(pc.Content, "\xc3") && !strings.Contains(pc.Content, "\xc3\xa9") {
					t.Error("content contains partial UTF-8 sequence")
				}
			},
		},
		{
			name:       "start landing mid-rune",
			content:    "abc\xc3\xa9def", // "abcédef"
			startIndex: 4,                // byte 4 is 0xa9 (continuation byte of é)
			maxLength:  100,
			wantMore:   false,
			checkFn: func(t *testing.T, pc *ProcessedContent) {
				if pc.StartIndex != 5 { // should snap forward past the é
					t.Errorf("startIndex = %d, want 5", pc.StartIndex)
				}
				if pc.Content != "def" {
					t.Errorf("content = %q, want %q", pc.Content, "def")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := paginate(tt.content, tt.startIndex, tt.maxLength)
			if result.HasMore != tt.wantMore {
				t.Errorf("HasMore = %v, want %v", result.HasMore, tt.wantMore)
			}
			if result.TotalLength != len(tt.content) {
				t.Errorf("TotalLength = %d, want %d", result.TotalLength, len(tt.content))
			}
			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestIntegrationPipeline(t *testing.T) {
	p := NewProcessor()

	html := `<!DOCTYPE html>
<html><head><title>Integration Test</title></head>
<body>
<article>
<h1>Test Page</h1>
<p>This is a test page with enough content for the readability algorithm to work.
We need multiple paragraphs of text to ensure that the content extraction produces
meaningful output from the HTML processing pipeline.</p>
<p>Second paragraph adds more text so the readability engine considers this block
substantial enough to be the main article content on the page.</p>
</article>
</body></html>`

	resp := makeResponse("text/html; charset=utf-8", html)
	result, err := p.Process(resp, "https://example.com/test", 0, 50, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MIMEType != "text/html" {
		t.Errorf("MIMEType = %q, want text/html", result.MIMEType)
	}
	if result.TotalLength == 0 {
		t.Error("TotalLength should be > 0")
	}
	if !result.HasMore {
		t.Error("expected HasMore=true with maxLength=50")
	}
	if result.StartIndex != 0 {
		t.Errorf("StartIndex = %d, want 0", result.StartIndex)
	}
}
