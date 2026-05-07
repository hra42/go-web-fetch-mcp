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

The server supports two transports:

- **stdio** (default) — launched as a subprocess by an MCP client (e.g. Claude Desktop) and speaks JSON-RPC over stdin/stdout.
- **http** — listens for the [streamable HTTP](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#streamable-http) MCP transport, suitable for self-hosting one shared instance behind a network endpoint. Requires a Bearer token.

```bash
web-fetch-mcp [flags]
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--user-agent` | `ModelContextProtocol/1.0 (Autonomous; ...)` | User-Agent header for HTTP requests |
| `--ignore-robots-txt` | `false` | Skip robots.txt compliance checks |
| `--proxy-url` | _(none)_ | HTTP/HTTPS proxy URL |
| `--transport` | `stdio` | MCP transport: `stdio` or `http` |
| `--listen` | `:8080` | Bind address for `http` transport |

### Environment

| Variable | Required when | Description |
|----------|---------------|-------------|
| `WEB_FETCH_MCP_TOKEN` | `--transport=http` | Bearer token clients must send via `Authorization: Bearer <token>` |

### Remote (HTTP) Mode

```bash
export WEB_FETCH_MCP_TOKEN=$(openssl rand -hex 32)
web-fetch-mcp --transport=http --listen=:8080
```

Clients must include `Authorization: Bearer $WEB_FETCH_MCP_TOKEN` on every request; missing or wrong tokens return `401 Unauthorized`. For Claude Desktop or other clients that only speak stdio, bridge with [`mcp-remote`](https://www.npmjs.com/package/mcp-remote) pointing at the endpoint.

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

## Self-Hosting via Docker Compose

For running the HTTP transport on a VPS behind a reverse proxy (Caddy, Traefik, Cloudflare Tunnel, …) which terminates TLS.

**Prerequisites:** Docker + Compose plugin, an existing reverse proxy attached to a shared external Docker network. Create one if you don't have it yet:

```bash
docker network create web
```

**Setup:**

```bash
cp .env.example .env
sed -i "s/replace-me/$(openssl rand -hex 32)/" .env   # generate token
docker compose up -d --build
```

The container exposes port `8080` only on the internal `web` network — never publish it to the host. Routes:

- `GET /healthz` → `200 ok` (no auth, for health probes)
- `POST /` → MCP streamable HTTP, requires `Authorization: Bearer <token>`

**Example Caddyfile entry** (Caddy on the same `web` network):

```caddy
fetch.example.com {
    reverse_proxy web-fetch-mcp:8080
}
```

**Verify:**

```bash
curl -fsS https://fetch.example.com/healthz   # → ok
curl -i -X POST https://fetch.example.com/ \
     -H "Authorization: Bearer $WEB_FETCH_MCP_TOKEN" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}'
```

Logs are emitted as JSON to stderr in HTTP mode — pipe `docker compose logs` into your log aggregator if needed.

To bridge a stdio-only client (like Claude Desktop) to the remote endpoint, use [`mcp-remote`](https://www.npmjs.com/package/mcp-remote).

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

This project is released into the public domain under the [Unlicense](LICENSE).
