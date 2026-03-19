package openapi

import (
	"os"
	"testing"
)

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
