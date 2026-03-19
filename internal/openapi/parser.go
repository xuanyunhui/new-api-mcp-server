package openapi

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type ToolDef struct {
	Name        string
	Description string
	Method      string
	Path        string
	Tags        []string
	InputSchema any

	PathParams   []ParamDef
	QueryParams  []ParamDef
	HeaderParams []ParamDef
	HasBody      bool
}

type ParamDef struct {
	Name     string
	In       string
	Required bool
	Schema   map[string]any
}

func Parse(specData []byte) ([]ToolDef, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specData)
	if err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}

	var defs []ToolDef
	nameCount := map[string]int{}

	for path, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			def := buildToolDef(path, method, op)

			// Deduplicate: if name already seen, append path slug
			nameCount[def.Name]++
			if nameCount[def.Name] > 1 {
				def.Name = def.Name + "_" + sanitizeToolName(strings.ReplaceAll(strings.Trim(path, "/"), "/", "_"))
			}

			defs = append(defs, def)
		}
	}

	return defs, nil
}

func buildToolDef(path, method string, op *openapi3.Operation) ToolDef {
	name := sanitizeToolName(op.OperationID)
	if name == "" {
		name = generateName(method, path)
	}

	desc := op.Summary
	if op.Description != "" {
		if desc != "" {
			desc += " - " + op.Description
		} else {
			desc = op.Description
		}
	}

	var tags []string
	tags = append(tags, op.Tags...)

	def := ToolDef{
		Name:        name,
		Description: desc,
		Method:      strings.ToUpper(method),
		Path:        path,
		Tags:        tags,
	}

	properties := map[string]any{}
	var required []any

	for _, paramRef := range op.Parameters {
		param := paramRef.Value
		if param == nil {
			continue
		}

		pd := ParamDef{
			Name:     param.Name,
			In:       param.In,
			Required: param.Required,
		}

		propSchema := map[string]any{}
		if param.Schema != nil && param.Schema.Value != nil {
			propSchema = schemaToMap(param.Schema.Value)
		}
		if param.Description != "" {
			propSchema["description"] = param.Description
		}
		pd.Schema = propSchema

		properties[param.Name] = propSchema

		if param.Required {
			required = append(required, param.Name)
		}

		switch param.In {
		case "path":
			def.PathParams = append(def.PathParams, pd)
		case "query":
			def.QueryParams = append(def.QueryParams, pd)
		case "header":
			def.HeaderParams = append(def.HeaderParams, pd)
		}
	}

	if op.RequestBody != nil && op.RequestBody.Value != nil {
		rb := op.RequestBody.Value
		if ct := rb.Content.Get("application/json"); ct != nil && ct.Schema != nil && ct.Schema.Value != nil {
			bodySchema := schemaToMap(ct.Schema.Value)
			properties["body"] = bodySchema
			if rb.Required {
				required = append(required, "body")
			}
			def.HasBody = true
		}
	}

	inputSchema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}

	def.InputSchema = inputSchema

	return def
}

func schemaToMap(s *openapi3.Schema) map[string]any {
	m := map[string]any{}

	if s.Type != nil {
		types := s.Type.Slice()
		if len(types) == 1 {
			m["type"] = types[0]
		} else if len(types) > 1 {
			m["type"] = types
		}
	}

	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if s.Enum != nil {
		m["enum"] = s.Enum
	}

	if len(s.Properties) > 0 {
		props := map[string]any{}
		for name, ref := range s.Properties {
			if ref.Value != nil {
				props[name] = schemaToMap(ref.Value)
			}
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		req := make([]any, len(s.Required))
		for i, r := range s.Required {
			req[i] = r
		}
		m["required"] = req
	}

	if s.Items != nil && s.Items.Value != nil {
		m["items"] = schemaToMap(s.Items.Value)
	}

	// Composition keywords
	if len(s.OneOf) > 0 {
		oneOf := make([]any, 0, len(s.OneOf))
		for _, ref := range s.OneOf {
			if ref.Value != nil {
				oneOf = append(oneOf, schemaToMap(ref.Value))
			}
		}
		m["oneOf"] = oneOf
	}
	if len(s.AnyOf) > 0 {
		anyOf := make([]any, 0, len(s.AnyOf))
		for _, ref := range s.AnyOf {
			if ref.Value != nil {
				anyOf = append(anyOf, schemaToMap(ref.Value))
			}
		}
		m["anyOf"] = anyOf
	}
	if len(s.AllOf) > 0 {
		allOf := make([]any, 0, len(s.AllOf))
		for _, ref := range s.AllOf {
			if ref.Value != nil {
				allOf = append(allOf, schemaToMap(ref.Value))
			}
		}
		m["allOf"] = allOf
	}

	// Additional properties
	if s.AdditionalProperties.Schema != nil && s.AdditionalProperties.Schema.Value != nil {
		m["additionalProperties"] = schemaToMap(s.AdditionalProperties.Schema.Value)
	}

	return m
}

func generateName(method, path string) string {
	slug := strings.ToLower(path)
	slug = strings.ReplaceAll(slug, "{", "")
	slug = strings.ReplaceAll(slug, "}", "")
	slug = strings.ReplaceAll(slug, "/", "_")
	slug = strings.Trim(slug, "_")
	return sanitizeToolName(strings.ToLower(method) + "_" + slug)
}

func sanitizeToolName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
