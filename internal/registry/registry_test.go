package registry

import (
	"testing"

	"github.com/QuantumNous/new-api-mcp-server/internal/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegister_EnabledGroups(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "list_models", Method: "GET", Path: "/v1/models", Tags: []string{"models"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "create_chat", Method: "POST", Path: "/v1/chat/completions", Tags: []string{"chat"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "create_video", Method: "POST", Path: "/v1/video", Tags: []string{"视频生成"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{
		EnabledGroups: []string{"models", "chat"},
	}

	count := RegisterTools(server, defs, opts, nil)

	if count != 2 {
		t.Errorf("registered %d tools, want 2 (only models and chat enabled)", count)
	}
}

func TestRegister_AllGroups(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "list_models", Method: "GET", Path: "/v1/models", Tags: []string{"models"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "create_chat", Method: "POST", Path: "/v1/chat/completions", Tags: []string{"chat"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "create_video", Method: "POST", Path: "/v1/video", Tags: []string{"视频生成"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{
		AllGroups: true,
	}

	count := RegisterTools(server, defs, opts, nil)

	if count != 3 {
		t.Errorf("registered %d tools, want 3 (all groups enabled)", count)
	}
}

func TestRegister_DefaultNoneEnabled(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "list_models", Method: "GET", Path: "/v1/models", Tags: []string{"models"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{}

	count := RegisterTools(server, defs, opts, nil)
	if count != 0 {
		t.Errorf("registered %d tools, want 0 (default none enabled)", count)
	}
}

func TestRegister_PrefixMatch(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "create_video", Method: "POST", Path: "/v1/video", Tags: []string{"视频生成"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "create_kling", Method: "POST", Path: "/kling/video", Tags: []string{"视频生成/Kling格式"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "list_files", Method: "GET", Path: "/v1/files", Tags: []string{"未实现/Files"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{
		EnabledGroups: []string{"视频生成"},
	}

	count := RegisterTools(server, defs, opts, nil)
	if count != 2 {
		t.Errorf("registered %d tools, want 2 (视频生成 prefix matches 视频生成/Kling格式)", count)
	}
}

func TestRegister_APITools_Prefix(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0.1"}, nil)

	defs := []openapi.ToolDef{
		{Name: "get_channels", Method: "GET", Path: "/api/channel/", Tags: []string{"渠道管理"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	opts := Options{
		AllGroups:  true,
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
