//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	c := client.New(upstream.URL, "sk-test", "", 5*time.Second)
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
