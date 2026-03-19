# New API MCP Server

A Go MCP ([Model Context Protocol](https://modelcontextprotocol.io/)) server for [New API](https://github.com/QuantumNous/new-api). Automatically maps ~195 OpenAPI endpoints into MCP tools, enabling AI assistants to interact with New API through a standardized protocol.

## Features

- **~195 MCP tools** auto-generated from OpenAPI specs (157 admin + 38 relay)
- **Dual transport** — stdio and Streamable HTTP
- **Dual API key** — separate keys for model relay and admin management
- **Tag-based group control** — enable/disable tool groups by OpenAPI tags
- **Full observability** — Prometheus metrics, OpenTelemetry tracing, structured logging (slog)
- **Single binary** — OpenAPI specs embedded via `go:embed`, zero external files

## Quick Start

### Build

```bash
go build -o bin/new-api-mcp-server ./cmd/server
```

### Run (stdio mode)

```bash
export NEW_API_BASE_URL=https://your-new-api.example.com
export NEW_API_KEY=sk-your-relay-key

./bin/new-api-mcp-server
```

### Run (HTTP mode)

```bash
export NEW_API_BASE_URL=https://your-new-api.example.com
export NEW_API_KEY=sk-your-relay-key
export MCP_TRANSPORT=http
export MCP_HTTP_ADDR=:8080

./bin/new-api-mcp-server
```

### Use with Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "new-api": {
      "command": "/path/to/new-api-mcp-server",
      "env": {
        "NEW_API_BASE_URL": "https://your-new-api.example.com",
        "NEW_API_KEY": "sk-your-relay-key"
      }
    }
  }
}
```

### Use with Claude Code

```bash
claude mcp add new-api /path/to/new-api-mcp-server \
  -e NEW_API_BASE_URL=https://your-new-api.example.com \
  -e NEW_API_KEY=sk-your-relay-key
```

## Tool Categories

### Relay Tools (model calls, default ON)

Powered by `NEW_API_KEY`. Covers AI model endpoints:

| Category | Examples |
|----------|----------|
| OpenAI Chat | `POST /v1/chat/completions` |
| Claude Messages | `POST /v1/messages` |
| Embeddings | `POST /v1/embeddings` |
| Image Generation | `POST /v1/images/generations` |
| Audio (TTS/STT) | `POST /v1/audio/speech`, `POST /v1/audio/transcriptions` |
| Video Generation | `POST /v1/video/generations` |
| Rerank | `POST /v1/rerank` |
| Gemini | `POST /v1beta/models/{model}:generateContent` |

### API Tools (admin management, default OFF)

Powered by `NEW_API_SYSTEM_KEY`. Covers backend admin endpoints with `api_` prefix:

| Category | Examples |
|----------|----------|
| Channel Management | `api_get_all_channels`, `api_create_channel` |
| Token Management | `api_get_all_tokens`, `api_create_token` |
| User Management | `api_get_all_users`, `api_create_user` |
| Logs & Stats | `api_search_logs`, `api_get_log_stats` |
| Model Management | `api_get_all_models`, `api_create_model` |
| System Settings | `api_get_system_options`, `api_update_system_options` |
| Redemption Codes | `api_create_redemption`, `api_search_redemptions` |

Enable with:

```bash
export NEW_API_SYSTEM_KEY=sk-your-system-key
export MCP_API_TOOLS_ENABLED=true
```

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `NEW_API_BASE_URL` | **(required)** | New API base URL |
| `NEW_API_KEY` | | API key for relay (model) tools |
| `NEW_API_SYSTEM_KEY` | | System API key for admin tools |
| `NEW_API_TIMEOUT` | `30s` | Upstream request timeout |
| `MCP_TRANSPORT` | `stdio` | Transport mode: `stdio` or `http` |
| `MCP_HTTP_ADDR` | `:8080` | HTTP listen address (http mode only) |
| `MCP_API_TOOLS_ENABLED` | `false` | Enable admin tools (requires system key) |
| `MCP_RELAY_DISABLED_GROUPS` | | Comma-separated tag groups to disable |
| `MCP_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `MCP_LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `MCP_LOG_CONSOLE_ENABLED` | `true` | Console log output (disable when using OTLP) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | | OTLP endpoint for tracing |
| `OTEL_SERVICE_NAME` | `new-api-mcp-server` | Service name for tracing |
| `MCP_METRICS_ADDR` | `:9090` | Prometheus metrics listen address |
| `MCP_METRICS_PATH` | `/metrics` | Prometheus metrics path |

### Behavior Rules

- `NEW_API_KEY` not set → relay tools not registered
- `NEW_API_SYSTEM_KEY` not set or `MCP_API_TOOLS_ENABLED=false` → admin tools not registered
- `OTEL_EXPORTER_OTLP_ENDPOINT` not set → tracing uses noop (zero overhead)
- `MCP_RELAY_DISABLED_GROUPS=未实现,视频生成` → disables tools tagged with these groups

## Observability

### Prometheus Metrics

Default endpoint: `http://localhost:9090/metrics`

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mcp_tool_requests_total` | Counter | `tool`, `status` | Tool call count |
| `mcp_tool_request_duration_seconds` | Histogram | `tool` | Tool call duration |
| `mcp_upstream_requests_total` | Counter | `method`, `path`, `status_code` | Upstream API call count |
| `mcp_upstream_request_duration_seconds` | Histogram | `method`, `path` | Upstream API duration |

Plus Go runtime metrics (goroutines, GC, memory).

### OpenTelemetry Tracing

Each tool call creates a span with:
- Tool name, HTTP method, path template
- Child span for upstream HTTP request with status code
- `trace_id` and `span_id` injected into structured logs

### Structured Logging

JSON-formatted via `log/slog` with automatic:
- Trace correlation (`trace_id`, `span_id`)
- Sensitive key redaction (`authorization`, `token`, `password`, etc.)
- Per-call logging: tool name, status code, duration

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  MCP Server                      │
│                                                  │
│  ┌──────────┐  ┌──────────┐                     │
│  │  stdio   │  │Streamable│  ← Transport         │
│  │ transport │  │  HTTP    │                      │
│  └────┬─────┘  └────┬─────┘                     │
│       └──────┬──────┘                            │
│              ▼                                   │
│  ┌───────────────────┐                          │
│  │  Tool Registry    │  ← From embedded OpenAPI  │
│  │                   │                          │
│  │  ┌─────────────┐  │                          │
│  │  │ API Tools   │  │  ← System API Key        │
│  │  │ (off by def)│  │                          │
│  │  └─────────────┘  │                          │
│  │  ┌─────────────┐  │                          │
│  │  │ Relay Tools │  │  ← API Key               │
│  │  │ (on by def) │  │                          │
│  │  └─────────────┘  │                          │
│  └────────┬──────────┘                          │
│           ▼                                      │
│  ┌───────────────────┐                          │
│  │  HTTP Client      │  ← Upstream New API       │
│  └───────────────────┘                          │
│                                                  │
│  ┌───────────────────────────────────────────┐  │
│  │ Prometheus │ OpenTelemetry │ slog          │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

## Development

```bash
make build    # Build binary
make test     # Run tests (with race detector)
make lint     # Run golangci-lint
make run      # Run via go run

# Integration test
go test -tags=integration -v
```

## Tech Stack

- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — Official MCP Go SDK
- [getkin/kin-openapi](https://github.com/getkin/kin-openapi) — OpenAPI 3.0 parser
- [prometheus/client_golang](https://github.com/prometheus/client_golang) — Prometheus metrics
- [go.opentelemetry.io/otel](https://opentelemetry.io/docs/languages/go/) — OpenTelemetry tracing

## License

MIT
