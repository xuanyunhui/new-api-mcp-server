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

type Handler struct {
	client  *client.Client
	source  client.Source
	metrics *observability.Metrics
}

func New(c *client.Client, source client.Source, metrics *observability.Metrics) *Handler {
	return &Handler{client: c, source: source, metrics: metrics}
}

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

		var args map[string]any
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
		}

		// Substitute path parameters
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
		isError := resp.StatusCode < 200 || resp.StatusCode >= 300
		status := "success"
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
