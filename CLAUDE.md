# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go implementation of the Python fetch MCP (Model Context Protocol) server. Exposes a `fetch` tool over stdio (JSON-RPC) that retrieves web pages, extracts readable content, converts HTML to markdown, and supports pagination.

## Build & Development Commands

No Makefile yet. Use standard Go commands:

```bash
# Build
go build -o bin/web-fetch-mcp ./cmd/web-fetch-mcp

# Run
go run ./cmd/web-fetch-mcp [--user-agent <ua>] [--ignore-robots-txt] [--proxy-url <url>]

# Test
go test ./... -race

# Vet/Lint
go vet ./...
```

## Architecture

```
MCP Client (Claude, etc.)
    ↕ stdio (JSON-RPC)
cmd/web-fetch-mcp/main.go    — CLI entry point, flag parsing, config init
    ↓
internal/config/config.go     — Config struct, defaults (user-agent, proxy, robots.txt)
internal/fetcher/fetcher.go   — HTTP client (30s timeout, proxy support, redirects)
```

**Planned but not yet implemented:** content processor (readability + HTML→MD + pagination), MCP server wiring, robots.txt compliance.

## Key Patterns

- All library code lives under `internal/` (not importable externally)
- CLI entry point at `cmd/web-fetch-mcp/main.go`
- Module path: `github.com/hra42/go-web-fetch-mcp`
- Go 1.25.5, currently stdlib-only (no external deps yet)
- Context-aware HTTP requests with error wrapping (`%w`)
- Development plan docs in `dev-plan/` (gitignored)

## Planned Dependencies (not yet added)

- `github.com/modelcontextprotocol/go-sdk` — MCP server SDK
- `github.com/go-shiori/go-readability` — article extraction
- `github.com/JohannesKaufmann/html-to-markdown/v2` — HTML→Markdown
- `github.com/jimsmart/grobotstxt` — robots.txt parsing
