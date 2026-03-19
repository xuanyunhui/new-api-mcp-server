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
