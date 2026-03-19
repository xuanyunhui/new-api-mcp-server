# New API MCP Server — 设计文档

## 概述

一个 Go 语言实现的 MCP (Model Context Protocol) Server，为 [New API](https://github.com/QuantumNous/new-api) 提供 MCP 工具接口。支持 stdio 和 HTTP (Streamable HTTP) 两种传输模式，具备完整的微服务可观测性（Metrics、Tracing、Structured Logging）。

## 核心决策

| 决策项 | 选择 | 理由 |
|--------|------|------|
| MCP SDK | modelcontextprotocol/go-sdk (官方) | 官方维护，协议兼容性有保障 |
| Tool 注册方式 | 混合模式：go:embed + 运行时解析 | 单二进制部署 + ~160 端点手写不现实 |
| HTTP 传输 | Streamable HTTP | MCP 新版推荐 |
| 配置方式 | 环境变量 | 适合容器化部署 |
| 模块路径 | github.com/QuantumNous/new-api-mcp-server | 与 new-api 同组织 |

## 架构

```
┌─────────────────────────────────────────────────┐
│                  MCP Server                      │
│                                                  │
│  ┌──────────┐  ┌──────────┐                     │
│  │  stdio   │  │Streamable│  ← 传输层            │
│  │ transport │  │  HTTP    │    (二选一启动)       │
│  └────┬─────┘  └────┬─────┘                     │
│       └──────┬──────┘                            │
│              ▼                                   │
│  ┌───────────────────┐                          │
│  │   MCP Protocol    │  ← go-sdk 核心           │
│  │   (Tool Router)   │                          │
│  └────────┬──────────┘                          │
│           ▼                                      │
│  ┌───────────────────┐                          │
│  │  Tool Registry    │  ← 启动时从 OpenAPI 注册  │
│  │                   │                          │
│  │  ┌─────────────┐  │                          │
│  │  │ API Tools   │  │  ← System API Key        │
│  │  │ (整体开关)   │  │    默认关闭              │
│  │  └─────────────┘  │                          │
│  │  ┌─────────────┐  │                          │
│  │  │ Relay Tools │  │  ← API Key               │
│  │  │ (按tag分组)  │  │    默认开启              │
│  │  └─────────────┘  │                          │
│  └────────┬──────────┘                          │
│           ▼                                      │
│  ┌───────────────────┐                          │
│  │  HTTP Client      │  ← 调用 New API          │
│  │  (带 auth header) │                          │
│  └───────────────────┘                          │
│                                                  │
│  ┌───────────────────────────────────────────┐  │
│  │ Observability                              │  │
│  │ Prometheus Metrics │ OTel Tracing │ slog   │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

## API 来源

两个 OpenAPI 3.0 spec 文件通过 `go:embed` 编译进二进制：

- **api.json**（后台管理接口）：17 个 tag 分组，~120 个端点。涵盖系统管理、用户、渠道、令牌、日志、模型管理、兑换码、供应商等。
- **relay.json**（AI 模型接口）：21 个 tag 分组，~40 个端点。涵盖 OpenAI Chat/Completions/Embeddings、Claude Messages、Gemini、图片/视频/音频生成、Rerank 等。

## Tool 注册与分组

### OpenAPI → MCP Tool 映射

- 每个 OpenAPI endpoint 映射为一个 MCP tool
- **Relay tools** 命名：直接使用 operationId，如 `create_chat_completion`、`list_models`
- **API tools** 命名：加 `api_` 前缀，如 `api_get_all_channels`（避免与 relay 侧同名端点冲突）
- Tool description：取 OpenAPI 的 `summary` + `description`
- Tool inputSchema：从 OpenAPI 的 `parameters`（path/query/header）+ `requestBody` 自动生成 JSON Schema

### 分组控制

**Relay tools：**
- 默认全部开启
- 可通过 `MCP_RELAY_DISABLED_GROUPS` 按 tag 禁用某些分组
- 未配置 `NEW_API_KEY` 时，全部不注册

**API tools：**
- 默认全部关闭
- 通过 `MCP_API_TOOLS_ENABLED=true` 整体开启（全开或全关，不支持逐个控制）
- 未配置 `NEW_API_SYSTEM_KEY` 时，全部不注册

### Tool 调用流程

1. MCP client 调用 tool，传入参数
2. Tool handler 根据映射关系构造 HTTP 请求（method、path、query、body）
3. 根据 tool 来源（api/relay）注入对应的 API Key header
4. 发送请求到 New API base URL
5. 返回响应体作为 tool result

## 配置（环境变量）

```bash
# 必填 — New API 基础地址
NEW_API_BASE_URL=https://api.example.com

# Relay 侧 API Key（用于模型调用）
NEW_API_KEY=sk-xxx

# System API Key（用于后台管理）
NEW_API_SYSTEM_KEY=sk-sys-xxx

# 传输模式：stdio | http（默认 stdio）
MCP_TRANSPORT=stdio

# HTTP 模式监听地址（仅 http 模式生效）
MCP_HTTP_ADDR=:8080

# Relay tools 禁用的 tag 分组（逗号分隔）
MCP_RELAY_DISABLED_GROUPS=未实现

# API tools 总开关（默认 false）
MCP_API_TOOLS_ENABLED=false

# 上游请求超时（默认 30s）
NEW_API_TIMEOUT=30s

# Logging
MCP_LOG_LEVEL=info              # debug | info | warn | error
MCP_LOG_FORMAT=json             # json | text
MCP_LOG_CONSOLE_ENABLED=true    # 配置 OTLP 后可设为 false 关闭 console 输出

# OTLP（Tracing + Logging 共用）
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
OTEL_SERVICE_NAME=new-api-mcp-server

# Metrics
MCP_METRICS_ADDR=:9090          # 默认 :9090
MCP_METRICS_PATH=/metrics       # 默认 /metrics
```

### 配置规则

- `NEW_API_KEY` 未设置 → Relay tools 全部不注册
- `NEW_API_SYSTEM_KEY` 未设置或 `MCP_API_TOOLS_ENABLED=false` → API tools 全部不注册
- `OTEL_EXPORTER_OTLP_ENDPOINT` 未设置 → Tracing 使用 noop exporter，日志不走 OTLP
- `MCP_METRICS_ADDR` 始终生效，默认 `:9090`

## 可观测性

### Structured Logging (slog)

- 使用 Go 标准库 `log/slog`
- 支持 JSON / text 格式输出
- 日志中包含 trace_id、span_id 实现 log-trace 关联
- 每次 tool 调用记录：tool 名称、耗时、状态码、错误信息
- 默认输出到 console (stdout)
- 配置 `OTEL_EXPORTER_OTLP_ENDPOINT` 后支持 OTLP 导出
- `MCP_LOG_CONSOLE_ENABLED=false` 可在 OTLP 开启时关闭 console 输出

### Metrics (Prometheus)

- 默认开启，监听地址和路径可配置
- 核心指标：
  - `mcp_tool_requests_total{tool, status}` — tool 调用计数
  - `mcp_tool_request_duration_seconds{tool}` — tool 调用耗时直方图
  - `mcp_upstream_requests_total{method, path, status_code}` — 上游 API 调用计数
  - `mcp_upstream_request_duration_seconds{method, path}` — 上游 API 耗时直方图
  - Go runtime 默认指标（goroutines、GC、内存等）

### Tracing (OpenTelemetry)

- 使用 OTLP HTTP exporter，遵循 OTel 标准环境变量
- 未配置 endpoint 时使用 noop exporter，零开销
- 每次 tool 调用创建一个 span，包含：
  - tool 名称、参数名（不记录参数值，防止泄露敏感信息）
  - 子 span：上游 HTTP 请求（method、url、status_code）
- trace_id 注入到 slog，实现 log-trace 关联

## 错误处理

- 上游 API 返回非 2xx 时，将状态码和响应体透传给 MCP client
- 上游超时/不可达时，返回明确的错误信息，包含 tool 名称和上游 URL
- HTTP client 默认超时 30s，可通过 `NEW_API_TIMEOUT` 配置

## 安全

- API Key 不在日志中输出，slog 对 auth header 自动脱敏
- Tracing span 中只记录参数名不记录参数值
- HTTP 模式下不内置认证（由前置网关/反向代理负责）

## 项目结构

```
new-api-mcp-server/
├── cmd/
│   └── server/
│       └── main.go              # 入口，配置加载，启动
├── internal/
│   ├── config/
│   │   └── config.go            # 环境变量解析，配置结构体
│   ├── registry/
│   │   ├── registry.go          # OpenAPI 解析 → MCP tool 注册
│   │   └── openapi.go           # OpenAPI spec 解析逻辑
│   ├── handler/
│   │   └── handler.go           # 统一 tool 调用处理，构造 HTTP 请求
│   ├── client/
│   │   └── client.go            # 上游 HTTP client，auth 注入
│   └── observability/
│       ├── metrics.go           # Prometheus metrics 定义与注册
│       ├── tracing.go           # OTel tracing 初始化
│       └── logging.go           # slog + OTLP 日志配置
├── openapi/
│   ├── embed.go                 # go:embed 声明
│   ├── api.json                 # 后台管理 OpenAPI spec
│   └── relay.json               # AI 模型 OpenAPI spec
├── go.mod
├── go.sum
└── Makefile
```
