package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	BaseURL   string
	APIKey    string
	SystemKey string
	Timeout   time.Duration

	Transport     string
	HTTPAddr      string
	HTTPAuthToken string

	RelayEnabledGroups []string
	RelayAllGroups     bool
	APIToolsEnabled    bool

	LogLevel          string
	LogFormat         string
	LogConsoleEnabled bool

	OTLPEndpoint string
	ServiceName  string

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
		HTTPAuthToken:     os.Getenv("MCP_HTTP_AUTH_TOKEN"),
		APIToolsEnabled:   os.Getenv("MCP_API_TOOLS_ENABLED") == "true",
		LogLevel:          envOrDefault("MCP_LOG_LEVEL", "info"),
		LogFormat:         envOrDefault("MCP_LOG_FORMAT", "json"),
		LogConsoleEnabled: os.Getenv("MCP_LOG_CONSOLE_ENABLED") != "false",
		OTLPEndpoint:      os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:       envOrDefault("OTEL_SERVICE_NAME", "new-api-mcp-server"),
		MetricsAddr:       envOrDefault("MCP_METRICS_ADDR", ":9090"),
		MetricsPath:       envOrDefault("MCP_METRICS_PATH", "/metrics"),
	}

	if groups := os.Getenv("MCP_RELAY_ENABLED_GROUPS"); groups != "" {
		if groups == "all" {
			cfg.RelayAllGroups = true
		} else {
			cfg.RelayEnabledGroups = strings.Split(groups, ",")
		}
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
