package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
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
