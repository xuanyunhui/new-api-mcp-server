package registry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Options struct {
	DisabledGroups []string
	NamePrefix     string
}

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
