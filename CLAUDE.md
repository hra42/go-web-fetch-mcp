# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go implementation of a Model Context Protocol (MCP) server that fetches web pages, extracts readable content via readability algorithms, converts HTML to markdown, and supports byte-offset pagination. Communicates over stdio using JSON-RPC.

## Build & Development Commands

```bash
make build          # Build to bin/web-fetch-mcp
make test           # go test ./... -race
make lint           # go vet + golangci-lint (if installed)
make install        # go install ./cmd/web-fetch-mcp
make clean          # rm -rf bin/

# Run directly
go run ./cmd/web-fetch-mcp [--user-agent <ua>] [--ignore-robots-txt] [--proxy-url <url>]

# Run a single test
go test ./internal/server/ -run TestHandleFetch -race

# Run tests for a specific package
go test ./internal/processor/ -race
```

## Architecture

```
MCP Client (Claude Desktop, etc.)
    ↕ stdio JSON-RPC (mcp.StdioTransport)
cmd/web-fetch-mcp/main.go        — CLI flags → Config → wires Fetcher, Processor, Server, Checker
    ↓
internal/server/server.go         — MCP server; registers "fetch" tool; handleFetch orchestrates pipeline
internal/fetcher/fetcher.go       — HTTP GET with 30s timeout, proxy support, User-Agent
internal/processor/processor.go   — readability extraction → HTML-to-Markdown → UTF-8-safe pagination
internal/robots/robots.go         — robots.txt checker with 1h in-memory cache (per-origin)
internal/config/config.go         — Config struct and defaults
```

**Request flow:** `server.handleFetch` validates the URL → optional robots.txt check → `fetcher.Fetch` → `processor.Process` (readability + html→md + paginate) → returns `mcp.TextContent`.

## Key Patterns

- All library code in `internal/` — not importable externally
- Module: `github.com/hra42/go-web-fetch-mcp`, Go 1.25.5
- `server.RobotsChecker` is an interface; tests use a mock implementation
- Tests use `httptest.NewServer` for integration-style testing against real HTTP handlers
- `processor.paginate` works on byte offsets and snaps to UTF-8 rune boundaries
- Errors in tool handlers return `mcp.CallToolResult` with `IsError: true` (not Go errors) — the MCP SDK expects this pattern
- Logging goes to stderr (`log.SetOutput(os.Stderr)`) since stdout is the MCP transport
- Development plan docs live in `dev-plan/` (gitignored)

## Dependencies

- `github.com/modelcontextprotocol/go-sdk` — MCP server SDK (Server, Tool registration, StdioTransport)
- `github.com/go-shiori/go-readability` — article extraction from HTML
- `github.com/JohannesKaufmann/html-to-markdown/v2` — HTML→Markdown conversion
- `github.com/jimsmart/grobotstxt` — robots.txt rule matching
