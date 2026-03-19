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
