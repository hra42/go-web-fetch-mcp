# web-fetch-mcp

A Go implementation of a Model Context Protocol (MCP) server that fetches web pages, extracts readable content, converts HTML to markdown, and supports pagination.

## Features

- Fetch web pages over HTTP/HTTPS
- Extract readable article content using readability algorithms
- Convert HTML to clean Markdown
- Paginate large content with byte-offset continuation
- Respect robots.txt rules (with opt-out flag)
- HTTP/HTTPS proxy support
- Configurable User-Agent

## Installation

```bash
go install github.com/hra42/go-web-fetch-mcp/cmd/web-fetch-mcp@latest
```

### Building from Source

```bash
git clone https://github.com/hra42/go-web-fetch-mcp.git
cd go-web-fetch-mcp
go build -o bin/web-fetch-mcp ./cmd/web-fetch-mcp
```

## Usage

The server communicates over stdio using JSON-RPC (MCP protocol). It is designed to be launched by an MCP client such as Claude Desktop.

```bash
web-fetch-mcp [flags]
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--user-agent` | `ModelContextProtocol/1.0 (Autonomous; ...)` | User-Agent header for HTTP requests |
| `--ignore-robots-txt` | `false` | Skip robots.txt compliance checks |
| `--proxy-url` | _(none)_ | HTTP/HTTPS proxy URL |

### Claude Desktop Configuration

Add the following to your Claude Desktop MCP config (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fetch": {
      "command": "web-fetch-mcp",
      "args": []
    }
  }
}
```

With custom options:

```json
{
  "mcpServers": {
    "fetch": {
      "command": "web-fetch-mcp",
      "args": ["--ignore-robots-txt", "--user-agent", "Mozilla/5.0 (compatible)"]
    }
  }
}
```

## MCP Tool

The server exposes a single `fetch` tool with the following parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `url` | string | _(required)_ | The URL to fetch |
| `max_length` | int | 5000 | Maximum content length in bytes |
| `start_index` | int | 0 | Byte offset for pagination |
| `raw` | bool | false | Skip readability, return raw converted content |

### Pagination

When content exceeds `max_length`, the response includes a continuation hint with the `start_index` to use for the next page.

## Development

```bash
# Run tests
make test

# Build
make build

# Lint
make lint

# Install locally
make install

# Clean build artifacts
make clean
```

## Docker

```bash
docker build -t web-fetch-mcp .
docker run -i web-fetch-mcp
```

## Architecture

```
MCP Client (Claude, etc.)
    ↕ stdio (JSON-RPC)
cmd/web-fetch-mcp/main.go       — CLI entry point
    ↓
internal/config/config.go        — Configuration and defaults
internal/fetcher/fetcher.go      — HTTP client with proxy and redirect support
internal/processor/processor.go  — Readability extraction, HTML→Markdown, pagination
internal/robots/robots.go        — robots.txt compliance with caching
internal/server/server.go        — MCP server and tool handler
```

## License

MIT
