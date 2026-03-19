package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

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
