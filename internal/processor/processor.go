package processor

import (
	"bytes"
	"fmt"
	"mime"
	"net/url"
	"strings"
	"unicode/utf8"

	readability "codeberg.org/readeck/go-readability/v2"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"

	"github.com/hra42/go-web-fetch-mcp/internal/fetcher"
)

// ProcessedContent holds the result of processing a fetched response.
type ProcessedContent struct {
	Title       string
	Content     string
	MIMEType    string
	TotalLength int
	StartIndex  int
	HasMore     bool
}

// Processor transforms raw HTTP responses into clean, readable content.
type Processor struct {
	defaultMaxLength int
}

// NewProcessor creates a Processor with default settings.
func NewProcessor() *Processor {
	return &Processor{
		defaultMaxLength: 5000,
	}
}

// Process transforms a fetcher.Response into ProcessedContent with pagination.
// pageURL is used for resolving relative links. If maxLength <= 0, the default (5000) is used.
// When raw is true and the content is HTML, readability extraction is skipped and the full HTML
// is converted directly to markdown.
func (p *Processor) Process(resp *fetcher.Response, pageURL string, startIndex, maxLength int, raw bool) (*ProcessedContent, error) {
	if maxLength <= 0 {
		maxLength = p.defaultMaxLength
	}

	mediaType := parseMediaType(resp.ContentType)

	var content, title string
	var err error

	switch {
	case mediaType == "text/html":
		if raw {
			content, err = convertHTMLToMarkdown(resp.Body, pageURL)
		} else {
			content, title, err = processHTML(resp.Body, pageURL)
		}
		if err != nil {
			return nil, err
		}
	case isTextType(mediaType):
		content = string(resp.Body)
	default:
		content = fmt.Sprintf("[Binary content: %s, %d bytes]", mediaType, len(resp.Body))
	}

	result := paginate(content, startIndex, maxLength)
	result.Title = title
	result.MIMEType = mediaType

	return result, nil
}

// parseMediaType extracts the clean MIME type, stripping charset and other params.
func parseMediaType(contentType string) string {
	if contentType == "" {
		return "application/octet-stream"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType
	}
	return mediaType
}

// isTextType returns true for MIME types that should be treated as text.
func isTextType(mediaType string) bool {
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	textLikeTypes := map[string]bool{
		"application/json":       true,
		"application/xml":        true,
		"application/javascript": true,
		"application/xhtml+xml":  true,
		"application/rss+xml":    true,
		"application/atom+xml":   true,
		"application/yaml":       true,
		"application/toml":       true,
		"application/x-yaml":     true,
	}
	return textLikeTypes[mediaType]
}

// processHTML extracts readable content from HTML and converts it to markdown.
func processHTML(body []byte, pageURL string) (content, title string, err error) {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing page URL: %w", err)
	}

	article, readErr := readability.FromReader(bytes.NewReader(body), parsedURL)

	html := ""
	if readErr != nil || article.Node == nil {
		// Fallback to raw HTML
		html = string(body)
	} else {
		title = article.Title()
		var buf bytes.Buffer
		if err := article.RenderHTML(&buf); err != nil {
			html = string(body)
		} else {
			html = buf.String()
		}
	}

	md, err := htmltomarkdown.ConvertString(html, converter.WithDomain(pageURL))
	if err != nil {
		return "", "", fmt.Errorf("converting HTML to markdown: %w", err)
	}

	return strings.TrimSpace(md), title, nil
}

// convertHTMLToMarkdown converts raw HTML to markdown without readability extraction.
func convertHTMLToMarkdown(body []byte, pageURL string) (string, error) {
	md, err := htmltomarkdown.ConvertString(string(body), converter.WithDomain(pageURL))
	if err != nil {
		return "", fmt.Errorf("converting HTML to markdown: %w", err)
	}
	return strings.TrimSpace(md), nil
}

// paginate slices content by byte position, respecting UTF-8 boundaries.
func paginate(content string, startIndex, maxLength int) *ProcessedContent {
	totalLength := len(content)

	// Clamp startIndex
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex >= totalLength {
		return &ProcessedContent{
			Content:     "",
			TotalLength: totalLength,
			StartIndex:  totalLength,
			HasMore:     false,
		}
	}

	// Snap startIndex forward to UTF-8 rune boundary
	for startIndex < totalLength && !utf8.RuneStart(content[startIndex]) {
		startIndex++
	}

	end := startIndex + maxLength
	if end >= totalLength {
		return &ProcessedContent{
			Content:     content[startIndex:],
			TotalLength: totalLength,
			StartIndex:  startIndex,
			HasMore:     false,
		}
	}

	// Snap end backward to UTF-8 rune boundary
	for end > startIndex && !utf8.RuneStart(content[end]) {
		end--
	}

	truncated := content[startIndex:end]
	hint := fmt.Sprintf("\n\n---\nContent truncated. Showing bytes %d-%d of %d total. Use start_index=%d to continue reading.", startIndex, end, totalLength, end)

	return &ProcessedContent{
		Content:     truncated + hint,
		TotalLength: totalLength,
		StartIndex:  startIndex,
		HasMore:     true,
	}
}
