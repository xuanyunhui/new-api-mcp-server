# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

A Go MCP (Model Context Protocol) server that wraps New API's OpenAPI endpoints (~160) as MCP tools. It supports stdio and HTTP (Streamable HTTP) transport modes with full observability (Prometheus, OpenTelemetry, slog).

## Build & Run

```bash
make build          # Build binary to bin/new-api-mcp-server
make test           # Run all tests with race detector
make lint           # Run golangci-lint
make run            # go run ./cmd/server
```

Single test: `go test ./internal/config/ -v -run TestLoad_Defaults`

Integration test: `go test -tags=integration -v -run TestIntegration`

## Architecture

- `openapi/` — Embedded OpenAPI specs (api.json, relay.json) via `go:embed`
- `internal/openapi/` — Parses OpenAPI JSON into `[]ToolDef` using kin-openapi
- `internal/registry/` — Filters tools by config and registers them on `mcp.Server` via `server.AddTool()`
- `internal/handler/` — Creates `mcp.ToolHandler` functions that map MCP tool calls to upstream HTTP requests
- `internal/client/` — HTTP client for upstream New API calls with API key injection
- `internal/observability/` — Logging (slog), Metrics (Prometheus), Tracing (OTel)
- `cmd/server/` — Entry point wiring everything together

## Key Design Decisions

- Tools are registered dynamically at startup using `server.AddTool(tool, handler)` (low-level API, not generic `AddTool[In,Out]`)
- Two API key types: `NEW_API_KEY` (relay/model tools) and `NEW_API_SYSTEM_KEY` (admin tools)
- API tools use `api_` name prefix; relay tools have no prefix
- API tools default OFF (whole group toggle); relay tools default ON (disable by tag)
- All config via environment variables
- Tool names are sanitized to `[a-zA-Z0-9_\-.]` per MCP SDK requirement
- Non-JSON upstream responses are base64 encoded
- Metrics use OpenAPI path templates (e.g., `/api/channel/{id}`) not resolved paths
