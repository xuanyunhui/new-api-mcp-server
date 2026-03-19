# New API MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go MCP server that dynamically maps New API's OpenAPI endpoints (~160) into MCP tools, with dual API key support, tag-based group controls, and full observability (Prometheus, OpenTelemetry, slog).

**Architecture:** OpenAPI JSON specs are embedded via `go:embed` and parsed at startup using `kin-openapi`. Each endpoint becomes an MCP tool registered via `server.AddTool()` with a raw `ToolHandler`. A unified HTTP client dispatches upstream requests with appropriate auth headers. The server supports stdio and Streamable HTTP transport modes.

**Tech Stack:** Go 1.23+, modelcontextprotocol/go-sdk (mcp package), getkin/kin-openapi, prometheus/client_golang, go.opentelemetry.io/otel, log/slog

**Spec:** `docs/superpowers/specs/2026-03-19-new-api-mcp-server-design.md`

---

## File Structure

```
new-api-mcp-server/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point: config → init observability → register tools → run transport
├── internal/
│   ├── config/
│   │   └── config.go                  # Env var parsing into Config struct
│   │   └── config_test.go
│   ├── openapi/
│   │   ├── parser.go                  # OpenAPI JSON → []ToolDef (parsed tool definitions)
│   │   └── parser_test.go
│   ├── registry/
│   │   ├── registry.go                # ToolDef filtering + MCP server.AddTool registration
│   │   └── registry_test.go
│   ├── handler/
│   │   ├── handler.go                 # ToolHandler: builds HTTP request from tool call args
│   │   └── handler_test.go
│   ├── client/
│   │   ├── client.go                  # Upstream HTTP client with auth injection + tracing
│   │   └── client_test.go
│   └── observability/
│       ├── logging.go                 # slog setup: console + OTLP, trace_id injection
│       ├── logging_test.go
│       ├── metrics.go                 # Prometheus metrics definition + HTTP server
│       ├── metrics_test.go
│       ├── tracing.go                 # OTel tracer provider setup
│       └── tracing_test.go
├── openapi/
│   ├── embed.go                       # go:embed declarations for api.json + relay.json
│   ├── api.json                       # Backend admin OpenAPI spec (from New API repo)
│   └── relay.json                     # AI model relay OpenAPI spec (from New API repo)
├── go.mod
├── go.sum
├── Makefile
└── CLAUDE.md
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `openapi/embed.go`
- Create: `openapi/api.json`
- Create: `openapi/relay.json`

- [ ] **Step 1: Initialize Go module**

```bash
cd /var/home/core/Developer/new-api-mcp-server
go mod init github.com/QuantumNous/new-api-mcp-server
```

- [ ] **Step 2: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: build test lint run clean

BINARY=new-api-mcp-server
VERSION?=dev
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/server

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

run:
	go run ./cmd/server

clean:
	rm -rf bin/
```

- [ ] **Step 3: Download OpenAPI specs**

```bash
gh api repos/QuantumNous/new-api/contents/docs/openapi/api.json --jq '.content' | base64 -d > openapi/api.json
gh api repos/QuantumNous/new-api/contents/docs/openapi/relay.json --jq '.content' | base64 -d > openapi/relay.json
```

- [ ] **Step 4: Create embed.go**

Create `openapi/embed.go`:

```go
package openapi

import _ "embed"

//go:embed api.json
var APISpec []byte

//go:embed relay.json
var RelaySpec []byte
```

- [ ] **Step 5: Install core dependencies**

```bash
go get github.com/modelcontextprotocol/go-sdk/mcp@latest
go get github.com/getkin/kin-openapi/openapi3@latest
go get github.com/prometheus/client_golang/prometheus@latest
go get github.com/prometheus/client_golang/prometheus/promhttp@latest
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
go get go.opentelemetry.io/otel/sdk/trace@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp@latest
go get go.opentelemetry.io/otel/sdk/log@latest
```

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum Makefile openapi/
git commit -m "chore: project scaffolding with go module, Makefile, and embedded OpenAPI specs"
```

---

### Task 2: Config

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear all relevant env vars
	for _, key := range []string{
		"NEW_API_BASE_URL", "NEW_API_KEY", "NEW_API_SYSTEM_KEY",
		"MCP_TRANSPORT", "MCP_HTTP_ADDR", "MCP_RELAY_DISABLED_GROUPS",
		"MCP_API_TOOLS_ENABLED", "NEW_API_TIMEOUT",
		"MCP_LOG_LEVEL", "MCP_LOG_FORMAT", "MCP_LOG_CONSOLE_ENABLED",
		"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_SERVICE_NAME",
		"MCP_METRICS_ADDR", "MCP_METRICS_PATH",
	} {
		t.Setenv(key, "")
	}

	t.Setenv("NEW_API_BASE_URL", "https://api.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://api.example.com")
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport = %q, want %q", cfg.Transport, "stdio")
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.APIToolsEnabled {
		t.Errorf("APIToolsEnabled = true, want false")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
	if !cfg.LogConsoleEnabled {
		t.Errorf("LogConsoleEnabled = false, want true")
	}
	if cfg.MetricsAddr != ":9090" {
		t.Errorf("MetricsAddr = %q, want %q", cfg.MetricsAddr, ":9090")
	}
	if cfg.MetricsPath != "/metrics" {
		t.Errorf("MetricsPath = %q, want %q", cfg.MetricsPath, "/metrics")
	}
	if cfg.ServiceName != "new-api-mcp-server" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "new-api-mcp-server")
	}
}

func TestLoad_MissingBaseURL(t *testing.T) {
	t.Setenv("NEW_API_BASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing base URL")
	}
}

func TestLoad_FullConfig(t *testing.T) {
	t.Setenv("NEW_API_BASE_URL", "https://api.test.com")
	t.Setenv("NEW_API_KEY", "sk-relay")
	t.Setenv("NEW_API_SYSTEM_KEY", "sk-sys")
	t.Setenv("MCP_TRANSPORT", "http")
	t.Setenv("MCP_HTTP_ADDR", ":3000")
	t.Setenv("MCP_RELAY_DISABLED_GROUPS", "视频生成,未实现")
	t.Setenv("MCP_API_TOOLS_ENABLED", "true")
	t.Setenv("NEW_API_TIMEOUT", "60s")
	t.Setenv("MCP_LOG_LEVEL", "debug")
	t.Setenv("MCP_LOG_FORMAT", "text")
	t.Setenv("MCP_LOG_CONSOLE_ENABLED", "false")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("OTEL_SERVICE_NAME", "my-mcp")
	t.Setenv("MCP_METRICS_ADDR", ":8081")
	t.Setenv("MCP_METRICS_PATH", "/prom")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.APIKey != "sk-relay" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "sk-relay")
	}
	if cfg.SystemKey != "sk-sys" {
		t.Errorf("SystemKey = %q, want %q", cfg.SystemKey, "sk-sys")
	}
	if cfg.Transport != "http" {
		t.Errorf("Transport = %q, want %q", cfg.Transport, "http")
	}
	if len(cfg.RelayDisabledGroups) != 2 || cfg.RelayDisabledGroups[0] != "视频生成" {
		t.Errorf("RelayDisabledGroups = %v, want [视频生成 未实现]", cfg.RelayDisabledGroups)
	}
	if !cfg.APIToolsEnabled {
		t.Errorf("APIToolsEnabled = false, want true")
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 60*time.Second)
	}
	if cfg.OTLPEndpoint != "http://otel:4318" {
		t.Errorf("OTLPEndpoint = %q, want %q", cfg.OTLPEndpoint, "http://otel:4318")
	}
	if cfg.MetricsPath != "/prom" {
		t.Errorf("MetricsPath = %q, want %q", cfg.MetricsPath, "/prom")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL (package/types not defined)

- [ ] **Step 3: Write implementation**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	// New API connection
	BaseURL   string
	APIKey    string
	SystemKey string
	Timeout   time.Duration

	// MCP transport
	Transport string
	HTTPAddr  string

	// Tool control
	RelayDisabledGroups []string
	APIToolsEnabled     bool

	// Logging
	LogLevel          string
	LogFormat         string
	LogConsoleEnabled bool

	// OpenTelemetry
	OTLPEndpoint string
	ServiceName  string

	// Metrics
	MetricsAddr string
	MetricsPath string
}

func Load() (*Config, error) {
	baseURL := os.Getenv("NEW_API_BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("NEW_API_BASE_URL is required")
	}

	cfg := &Config{
		BaseURL:           baseURL,
		APIKey:            os.Getenv("NEW_API_KEY"),
		SystemKey:         os.Getenv("NEW_API_SYSTEM_KEY"),
		Timeout:           parseDuration("NEW_API_TIMEOUT", 30*time.Second),
		Transport:         envOrDefault("MCP_TRANSPORT", "stdio"),
		HTTPAddr:          envOrDefault("MCP_HTTP_ADDR", ":8080"),
		APIToolsEnabled:   os.Getenv("MCP_API_TOOLS_ENABLED") == "true",
		LogLevel:          envOrDefault("MCP_LOG_LEVEL", "info"),
		LogFormat:         envOrDefault("MCP_LOG_FORMAT", "json"),
		LogConsoleEnabled: os.Getenv("MCP_LOG_CONSOLE_ENABLED") != "false",
		OTLPEndpoint:      os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:       envOrDefault("OTEL_SERVICE_NAME", "new-api-mcp-server"),
		MetricsAddr:       envOrDefault("MCP_METRICS_ADDR", ":9090"),
		MetricsPath:       envOrDefault("MCP_METRICS_PATH", "/metrics"),
	}

	if groups := os.Getenv("MCP_RELAY_DISABLED_GROUPS"); groups != "" {
		cfg.RelayDisabledGroups = strings.Split(groups, ",")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package for environment variable parsing"
```

---

### Task 3: OpenAPI Parser

**Files:**
- Create: `internal/openapi/parser.go`
- Create: `internal/openapi/parser_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/openapi/parser_test.go`:

```go
package openapi

import (
	"testing"
)

// Minimal valid OpenAPI 3.0 spec for testing
var testSpec = []byte(`{
  "openapi": "3.0.1",
  "info": {"title": "Test", "version": "1.0.0"},
  "paths": {
    "/api/items": {
      "get": {
        "operationId": "listItems",
        "summary": "List all items",
        "description": "Returns a list of items",
        "tags": ["items"],
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "required": false,
            "schema": {"type": "integer"}
          }
        ],
        "responses": {"200": {"description": "OK"}}
      },
      "post": {
        "summary": "Create item",
        "tags": ["items"],
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "name": {"type": "string"}
                },
                "required": ["name"]
              }
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/items/{id}": {
      "get": {
        "operationId": "getItem",
        "summary": "Get item by ID",
        "tags": ["items"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "integer"}
          }
        ],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/health": {
      "get": {
        "operationId": "healthCheck",
        "summary": "Health check",
        "tags": ["system"],
        "parameters": [],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`)

func TestParse_BasicEndpoints(t *testing.T) {
	defs, err := Parse(testSpec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(defs) != 4 {
		t.Fatalf("got %d tool defs, want 4", len(defs))
	}

	// Find listItems
	var listItems *ToolDef
	for i := range defs {
		if defs[i].Name == "listItems" {
			listItems = &defs[i]
			break
		}
	}
	if listItems == nil {
		t.Fatal("listItems tool not found")
	}
	if listItems.Method != "GET" {
		t.Errorf("listItems.Method = %q, want GET", listItems.Method)
	}
	if listItems.Path != "/api/items" {
		t.Errorf("listItems.Path = %q, want /api/items", listItems.Path)
	}
	if listItems.Tags[0] != "items" {
		t.Errorf("listItems.Tags = %v, want [items]", listItems.Tags)
	}
	if listItems.Description != "List all items - Returns a list of items" {
		t.Errorf("listItems.Description = %q", listItems.Description)
	}
}

func TestParse_MissingOperationId(t *testing.T) {
	defs, err := Parse(testSpec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// POST /api/items has no operationId, should generate one
	var createItem *ToolDef
	for i := range defs {
		if defs[i].Method == "POST" && defs[i].Path == "/api/items" {
			createItem = &defs[i]
			break
		}
	}
	if createItem == nil {
		t.Fatal("POST /api/items tool not found")
	}
	if createItem.Name != "post_api_items" {
		t.Errorf("generated name = %q, want %q", createItem.Name, "post_api_items")
	}
}

func TestParse_InputSchema_QueryParams(t *testing.T) {
	defs, err := Parse(testSpec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	var listItems *ToolDef
	for i := range defs {
		if defs[i].Name == "listItems" {
			listItems = &defs[i]
			break
		}
	}
	if listItems == nil {
		t.Fatal("listItems not found")
	}

	schema, ok := listItems.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("InputSchema type = %T, want map[string]any", listItems.InputSchema)
	}
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	if _, ok := props["limit"]; !ok {
		t.Error("schema missing 'limit' property")
	}
}

func TestParse_InputSchema_PathParams(t *testing.T) {
	defs, err := Parse(testSpec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	var getItem *ToolDef
	for i := range defs {
		if defs[i].Name == "getItem" {
			getItem = &defs[i]
			break
		}
	}
	if getItem == nil {
		t.Fatal("getItem not found")
	}

	schema := getItem.InputSchema.(map[string]any)
	props := schema["properties"].(map[string]any)
	if _, ok := props["id"]; !ok {
		t.Error("schema missing 'id' path parameter property")
	}

	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatal("required field missing")
	}
	found := false
	for _, r := range required {
		if r == "id" {
			found = true
		}
	}
	if !found {
		t.Error("'id' should be in required list")
	}
}

func TestParse_InputSchema_RequestBody(t *testing.T) {
	defs, err := Parse(testSpec)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	var createItem *ToolDef
	for i := range defs {
		if defs[i].Method == "POST" && defs[i].Path == "/api/items" {
			createItem = &defs[i]
			break
		}
	}
	if createItem == nil {
		t.Fatal("POST /api/items not found")
	}

	schema := createItem.InputSchema.(map[string]any)
	props := schema["properties"].(map[string]any)
	if _, ok := props["body"]; !ok {
		t.Error("schema missing 'body' property for request body")
	}
}

func TestParse_InvalidSpec(t *testing.T) {
	_, err := Parse([]byte(`{invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/openapi/ -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

Create `internal/openapi/parser.go`:

```go
package openapi

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ToolDef represents a parsed OpenAPI endpoint ready for MCP tool registration.
type ToolDef struct {
	Name        string
	Description string
	Method      string
	Path        string
	Tags        []string
	InputSchema any // map[string]any JSON Schema for MCP tool inputSchema

	// Internal metadata for request construction
	PathParams  []ParamDef
	QueryParams []ParamDef
	HasBody     bool
}

// ParamDef describes a single parameter extracted from the OpenAPI spec.
type ParamDef struct {
	Name     string
	In       string // "path", "query", "header"
	Required bool
	Schema   map[string]any
}

// Parse reads an OpenAPI 3.0 JSON spec and returns a list of ToolDefs.
func Parse(specData []byte) ([]ToolDef, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specData)
	if err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}

	// kin-openapi resolves $ref into .Value fields during LoadFromData,
	// so no additional ref resolution is needed.

	var defs []ToolDef

	for path, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			def := buildToolDef(path, method, op)
			defs = append(defs, def)
		}
	}

	return defs, nil
}

func buildToolDef(path, method string, op *openapi3.Operation) ToolDef {
	name := sanitizeToolName(op.OperationID)
	if name == "" {
		name = generateName(method, path)
	}

	desc := op.Summary
	if op.Description != "" {
		if desc != "" {
			desc += " - " + op.Description
		} else {
			desc = op.Description
		}
	}

	var tags []string
	tags = append(tags, op.Tags...)

	def := ToolDef{
		Name:        name,
		Description: desc,
		Method:      strings.ToUpper(method),
		Path:        path,
		Tags:        tags,
	}

	// Build inputSchema from parameters + requestBody
	properties := map[string]any{}
	var required []any

	// Process parameters (path, query, header)
	for _, paramRef := range op.Parameters {
		param := paramRef.Value
		if param == nil {
			continue
		}

		pd := ParamDef{
			Name:     param.Name,
			In:       param.In,
			Required: param.Required,
		}

		propSchema := map[string]any{}
		if param.Schema != nil && param.Schema.Value != nil {
			propSchema = schemaToMap(param.Schema.Value)
		}
		if param.Description != "" {
			propSchema["description"] = param.Description
		}
		pd.Schema = propSchema

		properties[param.Name] = propSchema

		if param.Required {
			required = append(required, param.Name)
		}

		switch param.In {
		case "path":
			def.PathParams = append(def.PathParams, pd)
		case "query":
			def.QueryParams = append(def.QueryParams, pd)
		}
	}

	// Process requestBody
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		rb := op.RequestBody.Value
		if ct := rb.Content.Get("application/json"); ct != nil && ct.Schema != nil && ct.Schema.Value != nil {
			bodySchema := schemaToMap(ct.Schema.Value)
			properties["body"] = bodySchema
			if rb.Required {
				required = append(required, "body")
			}
			def.HasBody = true
		}
	}

	inputSchema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}

	def.InputSchema = inputSchema

	return def
}

// schemaToMap converts an openapi3.Schema into a map[string]any for JSON Schema.
func schemaToMap(s *openapi3.Schema) map[string]any {
	m := map[string]any{}

	if s.Type != nil {
		types := s.Type.Slice()
		if len(types) == 1 {
			m["type"] = types[0]
		} else if len(types) > 1 {
			m["type"] = types
		}
	}

	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if s.Enum != nil {
		m["enum"] = s.Enum
	}

	// Object properties
	if len(s.Properties) > 0 {
		props := map[string]any{}
		for name, ref := range s.Properties {
			if ref.Value != nil {
				props[name] = schemaToMap(ref.Value)
			}
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		req := make([]any, len(s.Required))
		for i, r := range s.Required {
			req[i] = r
		}
		m["required"] = req
	}

	// Array items
	if s.Items != nil && s.Items.Value != nil {
		m["items"] = schemaToMap(s.Items.Value)
	}

	return m
}

// generateName creates a tool name from method + path when operationId is missing.
// Example: "POST", "/api/items/{id}" → "post_api_items_id"
func generateName(method, path string) string {
	slug := strings.ToLower(path)
	slug = strings.ReplaceAll(slug, "{", "")
	slug = strings.ReplaceAll(slug, "}", "")
	slug = strings.ReplaceAll(slug, "/", "_")
	slug = strings.Trim(slug, "_")
	return sanitizeToolName(strings.ToLower(method) + "_" + slug)
}

// sanitizeToolName ensures the name only contains [a-zA-Z0-9_\-.]
// as required by the MCP SDK's validateToolName.
func sanitizeToolName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/openapi/ -v`
Expected: PASS

- [ ] **Step 5: Test with real OpenAPI specs**

Write a quick integration check to verify parsing works on the actual api.json and relay.json:

```go
// Add to parser_test.go
func TestParse_RealAPISpec(t *testing.T) {
	spec, err := os.ReadFile("../../openapi/api.json")
	if err != nil {
		t.Skip("api.json not found, skipping")
	}
	defs, err := Parse(spec)
	if err != nil {
		t.Fatalf("Parse(api.json) error: %v", err)
	}
	if len(defs) < 100 {
		t.Errorf("expected >100 tool defs from api.json, got %d", len(defs))
	}
	t.Logf("Parsed %d tool definitions from api.json", len(defs))
}

func TestParse_RealRelaySpec(t *testing.T) {
	spec, err := os.ReadFile("../../openapi/relay.json")
	if err != nil {
		t.Skip("relay.json not found, skipping")
	}
	defs, err := Parse(spec)
	if err != nil {
		t.Fatalf("Parse(relay.json) error: %v", err)
	}
	if len(defs) < 30 {
		t.Errorf("expected >30 tool defs from relay.json, got %d", len(defs))
	}
	t.Logf("Parsed %d tool definitions from relay.json", len(defs))
}
```

Run: `go test ./internal/openapi/ -v -run TestParse_Real`
Expected: PASS with logged counts

- [ ] **Step 6: Commit**

```bash
git add internal/openapi/
git commit -m "feat: add OpenAPI spec parser with kin-openapi"
```

---

### Task 4: Upstream HTTP Client

**Files:**
- Create: `internal/client/client.go`
- Create: `internal/client/client_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/client/client_test.go`:

```go
package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Do_RelayAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-relay-key", "sk-sys-key", 5*time.Second)

	resp, err := c.Do(context.Background(), SourceRelay, "POST", "/v1/chat/completions", nil, []byte(`{}`))
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer sk-relay-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-relay-key")
	}
}

func TestClient_Do_APIAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-relay", "sk-sys-key", 5*time.Second)

	resp, err := c.Do(context.Background(), SourceAPI, "GET", "/api/channel/", nil, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer sk-sys-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-sys-key")
	}
}

func TestClient_Do_QueryParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-key", "", 5*time.Second)
	params := map[string]string{"page": "1", "limit": "10"}

	resp, err := c.Do(context.Background(), SourceRelay, "GET", "/api/items", params, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	if gotQuery == "" {
		t.Error("expected query params, got empty")
	}
}

func TestClient_Do_ReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"hello"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-key", "", 5*time.Second)

	resp, err := c.Do(context.Background(), SourceRelay, "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"result":"hello"}` {
		t.Errorf("body = %q, want %q", string(body), `{"result":"hello"}`)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

Create `internal/client/client.go`:

```go
package client

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Source identifies which API key to use.
type Source int

const (
	SourceRelay Source = iota
	SourceAPI
)

var tracer = otel.Tracer("client")

// Client is the upstream HTTP client for New API.
type Client struct {
	baseURL    string
	relayKey   string
	systemKey  string
	httpClient *http.Client
}

// New creates a Client with the given configuration.
func New(baseURL, relayKey, systemKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:   baseURL,
		relayKey:  relayKey,
		systemKey: systemKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Do sends an HTTP request to the upstream New API.
func (c *Client) Do(ctx context.Context, source Source, method, path string, queryParams map[string]string, body []byte) (*http.Response, error) {
	ctx, span := tracer.Start(ctx, "upstream.request",
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
		),
	)
	defer span.End()

	url := c.baseURL + path
	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set auth header based on source
	key := c.relayKey
	if source == SourceAPI {
		key = c.systemKey
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add query parameters
	if len(queryParams) > 0 {
		q := req.URL.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	return resp, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/client/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "feat: add upstream HTTP client with auth injection and tracing"
```

---

### Task 5: Tool Handler

**Files:**
- Create: `internal/handler/handler.go`
- Create: `internal/handler/handler_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/handler/handler_test.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api-mcp-server/internal/client"
	"github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandle_SimpleGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/items" {
			t.Errorf("path = %s, want /api/items", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	c := client.New(srv.URL, "sk-key", "", 5*time.Second)
	def := openapi.ToolDef{
		Name:   "listItems",
		Method: "GET",
		Path:   "/api/items",
	}

	h := New(c, client.SourceRelay, nil)
	handler := h.MakeHandler(def)

	req := &mcp.CallToolRequest{}
	req.Params.Name = "listItems"
	req.Params.Arguments = json.RawMessage(`{}`)

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatal("result.IsError = true")
	}
	if len(result.Content) == 0 {
		t.Fatal("no content in result")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if text != `{"items":[]}` {
		t.Errorf("result text = %q, want %q", text, `{"items":[]}`)
	}
}

func TestHandle_PathParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/42" {
			t.Errorf("path = %s, want /api/items/42", r.URL.Path)
		}
		w.Write([]byte(`{"id":42}`))
	}))
	defer srv.Close()

	c := client.New(srv.URL, "sk-key", "", 5*time.Second)
	def := openapi.ToolDef{
		Name:   "getItem",
		Method: "GET",
		Path:   "/api/items/{id}",
		PathParams: []openapi.ParamDef{
			{Name: "id", In: "path", Required: true},
		},
	}

	h := New(c, client.SourceRelay, nil)
	handler := h.MakeHandler(def)

	req := &mcp.CallToolRequest{}
	req.Params.Arguments = json.RawMessage(`{"id": 42}`)

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if text != `{"id":42}` {
		t.Errorf("result text = %q", text)
	}
}

func TestHandle_QueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("query limit = %q, want 10", r.URL.Query().Get("limit"))
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := client.New(srv.URL, "sk-key", "", 5*time.Second)
	def := openapi.ToolDef{
		Name:   "listItems",
		Method: "GET",
		Path:   "/api/items",
		QueryParams: []openapi.ParamDef{
			{Name: "limit", In: "query"},
		},
	}

	h := New(c, client.SourceRelay, nil)
	handler := h.MakeHandler(def)

	req := &mcp.CallToolRequest{}
	req.Params.Arguments = json.RawMessage(`{"limit": 10}`)

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatal("result.IsError = true")
	}
}

func TestHandle_RequestBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(map[string]string{}) // placeholder
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		_ = b
		w.Write([]byte(`{"created":true}`))
	}))
	defer srv.Close()

	c := client.New(srv.URL, "sk-key", "", 5*time.Second)
	def := openapi.ToolDef{
		Name:    "createItem",
		Method:  "POST",
		Path:    "/api/items",
		HasBody: true,
	}

	h := New(c, client.SourceRelay, nil)
	handler := h.MakeHandler(def)

	req := &mcp.CallToolRequest{}
	req.Params.Arguments = json.RawMessage(`{"body": {"name": "test"}}`)

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if gotBody == "" {
		t.Error("expected request body, got empty")
	}
}

func TestHandle_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := client.New(srv.URL, "sk-key", "", 5*time.Second)
	def := openapi.ToolDef{
		Name:   "getItem",
		Method: "GET",
		Path:   "/api/items/999",
	}

	h := New(c, client.SourceRelay, nil)
	handler := h.MakeHandler(def)

	req := &mcp.CallToolRequest{}
	req.Params.Arguments = json.RawMessage(`{}`)

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for non-2xx response")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

Create `internal/handler/handler.go`:

```go
package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/QuantumNous/new-api-mcp-server/internal/client"
	"github.com/QuantumNous/new-api-mcp-server/internal/observability"
	"github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("handler")

// Handler creates MCP tool handlers from OpenAPI ToolDefs.
type Handler struct {
	client  *client.Client
	source  client.Source
	metrics *observability.Metrics // may be nil
}

// New creates a Handler with the given upstream client, source, and optional metrics.
func New(c *client.Client, source client.Source, metrics *observability.Metrics) *Handler {
	return &Handler{client: c, source: source, metrics: metrics}
}

// MakeHandler returns an mcp.ToolHandler for the given ToolDef.
func (h *Handler) MakeHandler(def openapi.ToolDef) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, span := tracer.Start(ctx, "tool.call",
			trace.WithAttributes(
				attribute.String("tool.name", def.Name),
				attribute.String("tool.method", def.Method),
				attribute.String("tool.path", def.Path),
			),
		)
		defer span.End()

		start := time.Now()

		// Parse arguments
		var args map[string]any
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
		}

		// Build path with path parameters substituted
		path := def.Path
		for _, p := range def.PathParams {
			if v, ok := args[p.Name]; ok {
				path = strings.ReplaceAll(path, "{"+p.Name+"}", fmt.Sprintf("%v", v))
			}
		}

		// Build query parameters
		var queryParams map[string]string
		if len(def.QueryParams) > 0 {
			queryParams = make(map[string]string)
			for _, p := range def.QueryParams {
				if v, ok := args[p.Name]; ok {
					queryParams[p.Name] = fmt.Sprintf("%v", v)
				}
			}
		}

		// Build request body
		var body []byte
		if def.HasBody {
			if bodyData, ok := args["body"]; ok {
				var err error
				body, err = json.Marshal(bodyData)
				if err != nil {
					return errorResult(fmt.Sprintf("marshal body: %v", err)), nil
				}
			}
		}

		// Call upstream
		resp, err := h.client.Do(ctx, h.source, def.Method, path, queryParams, body)
		if err != nil {
			slog.ErrorContext(ctx, "upstream request failed",
				"tool", def.Name,
				"error", err,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return errorResult(fmt.Sprintf("upstream error: %v", err)), nil
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return errorResult(fmt.Sprintf("read response: %v", err)), nil
		}

		duration := time.Since(start)
		status := "success"
		isError := resp.StatusCode < 200 || resp.StatusCode >= 300
		if isError {
			status = "error"
		}

		// Record metrics
		if h.metrics != nil {
			h.metrics.ToolRequestsTotal.WithLabelValues(def.Name, status).Inc()
			h.metrics.ToolRequestDuration.WithLabelValues(def.Name).Observe(duration.Seconds())
			h.metrics.UpstreamRequestsTotal.WithLabelValues(def.Method, def.Path, fmt.Sprintf("%d", resp.StatusCode)).Inc()
			h.metrics.UpstreamRequestDuration.WithLabelValues(def.Method, def.Path).Observe(duration.Seconds())
		}

		slog.InfoContext(ctx, "tool call completed",
			"tool", def.Name,
			"status_code", resp.StatusCode,
			"duration_ms", duration.Milliseconds(),
		)

		// Handle non-JSON response: base64 encode
		contentType := resp.Header.Get("Content-Type")
		var text string
		if strings.HasPrefix(contentType, "application/json") || contentType == "" {
			text = string(respBody)
		} else {
			text = base64.StdEncoding.EncodeToString(respBody)
		}

		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: text},
			},
		}

		if isError {
			result.IsError = true
			span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		}

		return result, nil
	}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/
git commit -m "feat: add tool handler for OpenAPI-to-HTTP request mapping"
```

---

### Task 6: Tool Registry

**Files:**
- Create: `internal/registry/registry.go`
- Create: `internal/registry/registry_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/registry/registry_test.go`:

```go
package registry

import (
	"testing"

	"github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegister_RelayTools(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "list_models", Method: "GET", Path: "/v1/models", Tags: []string{"models"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "create_chat", Method: "POST", Path: "/v1/chat/completions", Tags: []string{"chat"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "create_video", Method: "POST", Path: "/v1/video", Tags: []string{"视频生成"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{
		DisabledGroups: []string{"视频生成"},
		NamePrefix:     "",
	}

	count := RegisterTools(server, defs, opts, nil)

	if count != 2 {
		t.Errorf("registered %d tools, want 2 (视频生成 should be filtered)", count)
	}
}

func TestRegister_APITools_Prefix(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "get_channels", Method: "GET", Path: "/api/channel/", Tags: []string{"渠道管理"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{
		NamePrefix: "api_",
	}

	count := RegisterTools(server, defs, opts, nil)

	if count != 1 {
		t.Errorf("registered %d tools, want 1", count)
	}
}

func TestRegister_DuplicateNames(t *testing.T) {
	defs := []openapi.ToolDef{
		{Name: "list_models", Method: "GET", Path: "/v1/models", Tags: []string{"models"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "list_models", Method: "GET", Path: "/v2/models", Tags: []string{"models"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	err := ValidateUniqueNames(defs, "")
	if err == nil {
		t.Error("expected error for duplicate tool names")
	}
}

func TestRegister_AllGroupsDisabled(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "list_models", Method: "GET", Path: "/v1/models", Tags: []string{"models"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{
		DisabledGroups: []string{"models"},
	}

	count := RegisterTools(server, defs, opts, nil)
	if count != 0 {
		t.Errorf("registered %d tools, want 0", count)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/registry/ -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

Create `internal/registry/registry.go`:

```go
package registry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options controls tool registration behavior.
type Options struct {
	DisabledGroups []string
	NamePrefix     string // "api_" for API tools, "" for relay tools
}

// RegisterTools registers filtered ToolDefs onto the MCP server.
// The handler parameter creates the ToolHandler for each def; if nil, a no-op handler is used.
func RegisterTools(server *mcp.Server, defs []openapi.ToolDef, opts Options, makeHandler func(openapi.ToolDef) mcp.ToolHandler) int {
	disabled := make(map[string]bool)
	for _, g := range opts.DisabledGroups {
		disabled[g] = true
	}

	count := 0
	for _, def := range defs {
		if shouldSkip(def, disabled) {
			continue
		}

		toolName := opts.NamePrefix + def.Name

		var toolHandler mcp.ToolHandler
		if makeHandler != nil {
			toolHandler = makeHandler(def)
		} else {
			toolHandler = func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "not implemented"}},
				}, nil
			}
		}

		tool := &mcp.Tool{
			Name:        toolName,
			Description: def.Description,
			InputSchema: def.InputSchema,
		}

		server.AddTool(tool, toolHandler)
		count++

		slog.Debug("registered tool", "name", toolName, "method", def.Method, "path", def.Path)
	}

	return count
}

// ValidateUniqueNames checks that all tool names (with prefix) are unique.
func ValidateUniqueNames(defs []openapi.ToolDef, prefix string) error {
	seen := make(map[string]bool)
	for _, def := range defs {
		name := prefix + def.Name
		if seen[name] {
			return fmt.Errorf("duplicate tool name: %q", name)
		}
		seen[name] = true
	}
	return nil
}

func shouldSkip(def openapi.ToolDef, disabled map[string]bool) bool {
	if len(disabled) == 0 {
		return false
	}
	for _, tag := range def.Tags {
		if disabled[tag] {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/registry/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat: add tool registry with group filtering and name validation"
```

---

### Task 7: Observability — Logging

**Files:**
- Create: `internal/observability/logging.go`
- Create: `internal/observability/logging_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/observability/logging_test.go`:

```go
package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestSetupLogging_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupLogging("info", "json", true, "", &buf)

	logger.InfoContext(context.Background(), "test message", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON log output: %v\nraw: %s", err, buf.String())
	}
	if entry["msg"] != "test message" {
		t.Errorf("msg = %v, want 'test message'", entry["msg"])
	}
}

func TestSetupLogging_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupLogging("info", "text", true, "", &buf)

	logger.Info("hello text")

	if !bytes.Contains(buf.Bytes(), []byte("hello text")) {
		t.Errorf("log output missing message: %s", buf.String())
	}
}

func TestSetupLogging_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupLogging("warn", "json", true, "", &buf)

	logger.Info("should not appear")
	if buf.Len() > 0 {
		t.Errorf("info message should be filtered at warn level: %s", buf.String())
	}

	logger.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("warn message should appear at warn level")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		got := parseLogLevel(tt.input)
		if got != tt.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/observability/ -v -run TestSetupLogging`
Expected: FAIL

- [ ] **Step 3: Write implementation**

Create `internal/observability/logging.go`:

```go
package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// traceHandler wraps an slog.Handler to inject trace_id and span_id from context.
type traceHandler struct {
	inner slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		r.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}

// SetupLogging configures slog with the specified format and level.
// If writer is nil, os.Stdout is used.
// Returns the configured logger (also sets it as default).
func SetupLogging(level, format string, consoleEnabled bool, otlpEndpoint string, writer io.Writer) *slog.Logger {
	if writer == nil {
		writer = os.Stdout
	}

	if !consoleEnabled && otlpEndpoint != "" {
		writer = io.Discard
	}

	logLevel := parseLogLevel(level)
	opts := &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Redact sensitive values
			key := strings.ToLower(a.Key)
			if strings.Contains(key, "auth") || strings.Contains(key, "key") || strings.Contains(key, "token") || strings.Contains(key, "password") {
				a.Value = slog.StringValue("[REDACTED]")
			}
			return a
		},
	}

	var baseHandler slog.Handler
	if format == "text" {
		baseHandler = slog.NewTextHandler(writer, opts)
	} else {
		baseHandler = slog.NewJSONHandler(writer, opts)
	}

	handler := &traceHandler{inner: baseHandler}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/observability/ -v -run TestSetupLogging`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/observability/logging.go internal/observability/logging_test.go
git commit -m "feat: add structured logging with slog, trace correlation, and OTLP support"
```

---

### Task 8: Observability — Metrics

**Files:**
- Create: `internal/observability/metrics.go`
- Create: `internal/observability/metrics_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/observability/metrics_test.go`:

```go
package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	if m.ToolRequestsTotal == nil {
		t.Error("ToolRequestsTotal is nil")
	}
	if m.ToolRequestDuration == nil {
		t.Error("ToolRequestDuration is nil")
	}
	if m.UpstreamRequestsTotal == nil {
		t.Error("UpstreamRequestsTotal is nil")
	}
	if m.UpstreamRequestDuration == nil {
		t.Error("UpstreamRequestDuration is nil")
	}
}

func TestMetrics_RecordToolRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.ToolRequestsTotal.WithLabelValues("list_models", "success").Inc()

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "mcp_tool_requests_total" {
			found = true
		}
	}
	if !found {
		t.Error("mcp_tool_requests_total metric not found")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/observability/ -v -run TestNewMetrics`
Expected: FAIL

- [ ] **Step 3: Write implementation**

Create `internal/observability/metrics.go`:

```go
package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metric collectors.
type Metrics struct {
	ToolRequestsTotal      *prometheus.CounterVec
	ToolRequestDuration    *prometheus.HistogramVec
	UpstreamRequestsTotal  *prometheus.CounterVec
	UpstreamRequestDuration *prometheus.HistogramVec
}

// NewMetrics creates and registers all metrics with the given registry.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		ToolRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_tool_requests_total",
				Help: "Total number of MCP tool requests",
			},
			[]string{"tool", "status"},
		),
		ToolRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_tool_request_duration_seconds",
				Help:    "Duration of MCP tool requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"tool"},
		),
		UpstreamRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_upstream_requests_total",
				Help: "Total number of upstream API requests",
			},
			[]string{"method", "path", "status_code"},
		),
		UpstreamRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_upstream_request_duration_seconds",
				Help:    "Duration of upstream API requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
	}

	reg.MustRegister(
		m.ToolRequestsTotal,
		m.ToolRequestDuration,
		m.UpstreamRequestsTotal,
		m.UpstreamRequestDuration,
	)

	// Register Go runtime collectors if the registerer supports Gatherer
	if gathererReg, ok := reg.(*prometheus.Registry); ok {
		gathererReg.MustRegister(collectors.NewGoCollector())
	}

	return m
}

// Handler returns an HTTP handler serving metrics in Prometheus exposition format.
func Handler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/observability/ -v -run TestMetrics`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/observability/metrics.go internal/observability/metrics_test.go
git commit -m "feat: add Prometheus metrics for tool calls and upstream requests"
```

---

### Task 9: Observability — Tracing

**Files:**
- Create: `internal/observability/tracing.go`
- Create: `internal/observability/tracing_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/observability/tracing_test.go`:

```go
package observability

import (
	"context"
	"testing"
)

func TestSetupTracing_NoEndpoint(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), "", "test-service")
	if err != nil {
		t.Fatalf("SetupTracing() error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function should not be nil")
	}
	// Should not panic
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestSetupTracing_WithEndpoint(t *testing.T) {
	// Use a non-existent endpoint; we just test setup doesn't error
	shutdown, err := SetupTracing(context.Background(), "http://localhost:4318", "test-service")
	if err != nil {
		t.Fatalf("SetupTracing() error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function should not be nil")
	}
	// Shutdown should work (exporter may fail to connect, but shouldn't error on shutdown)
	shutdown(context.Background())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/observability/ -v -run TestSetupTracing`
Expected: FAIL

- [ ] **Step 3: Write implementation**

Create `internal/observability/tracing.go`:

```go
package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// SetupTracing initializes the OTel tracing provider.
// If endpoint is empty, a noop tracer is used.
// Returns a shutdown function that must be called on application exit.
func SetupTracing(ctx context.Context, endpoint, serviceName string) (func(context.Context) error, error) {
	if endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(stripScheme(endpoint)),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// stripScheme removes http:// or https:// prefix for otlptracehttp.WithEndpoint.
func stripScheme(endpoint string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if len(endpoint) > len(prefix) && endpoint[:len(prefix)] == prefix {
			return endpoint[len(prefix):]
		}
	}
	return endpoint
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/observability/ -v -run TestSetupTracing`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/observability/tracing.go internal/observability/tracing_test.go
git commit -m "feat: add OpenTelemetry tracing with OTLP exporter and noop fallback"
```

---

### Task 10: Main Entry Point

**Files:**
- Create: `cmd/server/main.go`

- [ ] **Step 1: Write main.go**

Create `cmd/server/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api-mcp-server/internal/client"
	"github.com/QuantumNous/new-api-mcp-server/internal/config"
	"github.com/QuantumNous/new-api-mcp-server/internal/handler"
	"github.com/QuantumNous/new-api-mcp-server/internal/observability"
	openapipkg "github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/QuantumNous/new-api-mcp-server/internal/registry"
	embeddedSpecs "github.com/QuantumNous/new-api-mcp-server/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Setup logging
	observability.SetupLogging(cfg.LogLevel, cfg.LogFormat, cfg.LogConsoleEnabled, cfg.OTLPEndpoint, nil)

	slog.Info("starting new-api-mcp-server", "version", version, "transport", cfg.Transport)

	// Setup tracing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTracing, err := observability.SetupTracing(ctx, cfg.OTLPEndpoint, cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("setup tracing: %w", err)
	}
	defer shutdownTracing(ctx)

	// Setup metrics
	promRegistry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(promRegistry)

	// Start metrics HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.Handle(cfg.MetricsPath, observability.Handler(promRegistry))
	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metricsMux,
	}

	go func() {
		slog.Info("metrics server starting", "addr", cfg.MetricsAddr, "path", cfg.MetricsPath)
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("metrics server error", "error", err)
		}
	}()

	// Create upstream clients
	relayClient := client.New(cfg.BaseURL, cfg.APIKey, cfg.SystemKey, cfg.Timeout)

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "new-api-mcp-server",
		Version: version,
	}, nil)

	// Register relay tools
	if cfg.APIKey != "" {
		relayDefs, err := openapipkg.Parse(embeddedSpecs.RelaySpec)
		if err != nil {
			return fmt.Errorf("parse relay spec: %w", err)
		}

		if err := registry.ValidateUniqueNames(relayDefs, ""); err != nil {
			return fmt.Errorf("relay tools: %w", err)
		}

		relayHandler := handler.New(relayClient, client.SourceRelay, metrics)
		count := registry.RegisterTools(server, relayDefs, registry.Options{
			DisabledGroups: cfg.RelayDisabledGroups,
			NamePrefix:     "",
		}, relayHandler.MakeHandler)

		slog.Info("registered relay tools", "count", count)
	} else {
		slog.Warn("NEW_API_KEY not set, relay tools disabled")
	}

	// Register API tools
	if cfg.SystemKey != "" && cfg.APIToolsEnabled {
		apiDefs, err := openapipkg.Parse(embeddedSpecs.APISpec)
		if err != nil {
			return fmt.Errorf("parse api spec: %w", err)
		}

		if err := registry.ValidateUniqueNames(apiDefs, "api_"); err != nil {
			return fmt.Errorf("api tools: %w", err)
		}

		apiHandler := handler.New(relayClient, client.SourceAPI, metrics)
		count := registry.RegisterTools(server, apiDefs, registry.Options{
			NamePrefix: "api_",
		}, apiHandler.MakeHandler)

		slog.Info("registered api tools", "count", count)
	} else {
		slog.Info("API tools disabled", "key_set", cfg.SystemKey != "", "enabled", cfg.APIToolsEnabled)
	}

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)

		// 1. Stop accepting new requests (cancel context)
		cancel()

		// 2. Wait for in-flight tool calls (context cancellation propagates, 10s deadline)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// 3. Flush OTel tracing spans and logging data
		if err := shutdownTracing(shutdownCtx); err != nil {
			slog.Error("flush tracing failed", "error", err)
		}

		// 4. Close metrics HTTP server
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("metrics server shutdown failed", "error", err)
		}

		slog.Info("graceful shutdown complete")
	}()

	// Run transport
	switch cfg.Transport {
	case "http":
		slog.Info("starting HTTP transport", "addr", cfg.HTTPAddr)
		httpHandler := mcp.NewStreamableHTTPHandler(
			func(r *http.Request) *mcp.Server { return server },
			nil,
		)
		httpServer := &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: httpHandler,
		}
		go func() {
			<-ctx.Done()
			httpServer.Shutdown(context.Background())
		}()
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
	default:
		slog.Info("starting stdio transport")
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			return fmt.Errorf("stdio server: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./cmd/server`
Expected: BUILD SUCCESS

- [ ] **Step 3: Run with missing config to verify error handling**

```bash
unset NEW_API_BASE_URL
go run ./cmd/server 2>&1 || true
```
Expected: Error message about missing NEW_API_BASE_URL

- [ ] **Step 4: Commit**

```bash
git add cmd/server/
git commit -m "feat: add main entry point with transport selection and graceful shutdown"
```

---

### Task 11: CLAUDE.md

**Files:**
- Create: `CLAUDE.md`

- [ ] **Step 1: Create CLAUDE.md**

Create `CLAUDE.md`:

```markdown
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

## Architecture

- `openapi/` — Embedded OpenAPI specs (api.json, relay.json) via `go:embed`
- `internal/openapi/` — Parses OpenAPI JSON into `[]ToolDef` using kin-openapi
- `internal/registry/` — Filters tools by config and registers them on `mcp.Server` via `server.AddTool()`
- `internal/handler/` — Creates `mcp.ToolHandler` functions that map MCP tool calls → upstream HTTP requests
- `internal/client/` — HTTP client for upstream New API calls with API key injection
- `internal/observability/` — Logging (slog), Metrics (Prometheus), Tracing (OTel)
- `cmd/server/` — Entry point wiring everything together

## Key Design Decisions

- Tools are registered dynamically at startup using `server.AddTool(tool, handler)` (low-level API, not generic `AddTool[In,Out]`)
- Two API key types: `NEW_API_KEY` (relay/model tools) and `NEW_API_SYSTEM_KEY` (admin tools)
- API tools use `api_` name prefix; relay tools have no prefix
- API tools default OFF (whole group toggle); relay tools default ON (disable by tag)
- All config via environment variables
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md for Claude Code guidance"
```

---

### Task 12: Integration Test

**Files:**
- Create: `integration_test.go` (root level)

- [ ] **Step 1: Write integration test**

Create `integration_test.go`:

```go
//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api-mcp-server/internal/client"
	"github.com/QuantumNous/new-api-mcp-server/internal/handler"
	openapipkg "github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/QuantumNous/new-api-mcp-server/internal/registry"
	embeddedSpecs "github.com/QuantumNous/new-api-mcp-server/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIntegration_FullPipeline(t *testing.T) {
	// Mock upstream API
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"path":   r.URL.Path,
			"method": r.Method,
			"auth":   r.Header.Get("Authorization"),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	// Parse relay spec
	defs, err := openapipkg.Parse(embeddedSpecs.RelaySpec)
	if err != nil {
		t.Fatalf("parse relay spec: %v", err)
	}
	t.Logf("Parsed %d relay tool definitions", len(defs))

	// Create MCP server with tools
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)
	c := client.New(upstream.URL, "sk-test", "", 5_000_000_000)
	h := handler.New(c, client.SourceRelay, nil)

	count := registry.RegisterTools(server, defs, registry.Options{
		DisabledGroups: []string{"未实现"},
	}, h.MakeHandler)
	t.Logf("Registered %d relay tools", count)

	if count == 0 {
		t.Fatal("expected at least 1 registered tool")
	}

	// Connect an in-memory client and call a tool
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	go server.Run(context.Background(), serverTransport)

	session, err := mcpClient.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}

	// List tools
	toolsResult, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	t.Logf("Listed %d tools", len(toolsResult.Tools))

	if len(toolsResult.Tools) == 0 {
		t.Fatal("no tools listed")
	}

	// Call the first available tool
	firstTool := toolsResult.Tools[0]
	callResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      firstTool.Name,
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(%s) error: %v", firstTool.Name, err)
	}
	t.Logf("Called tool %s, isError=%v", firstTool.Name, callResult.IsError)

	if len(callResult.Content) == 0 {
		t.Error("expected content in result")
	}
}
```

- [ ] **Step 2: Run the integration test**

Run: `go test -tags=integration -v -run TestIntegration_FullPipeline`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add integration_test.go
git commit -m "test: add integration test with in-memory MCP transport"
```

---

### Task 13: Final Verification

- [ ] **Step 1: Run all tests**

```bash
make test
```
Expected: ALL PASS

- [ ] **Step 2: Run linter (if available)**

```bash
make lint || echo "golangci-lint not installed, skipping"
```

- [ ] **Step 3: Build binary**

```bash
make build
ls -la bin/new-api-mcp-server
```
Expected: Binary exists

- [ ] **Step 4: Test stdio mode quick smoke test**

```bash
NEW_API_BASE_URL=https://example.com NEW_API_KEY=sk-test echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}},"id":1}' | go run ./cmd/server 2>/dev/null
```
Expected: JSON response with server info

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git add -A
git status
# Only commit if there are changes
```
