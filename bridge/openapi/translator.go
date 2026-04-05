package openapi

import (
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// operationToTool translates an OpenAPI operation into an mcpkit ToolDefinition.
//
// Mapping rules:
//   - operationId or method_path -> Tool.Name
//   - operation.Summary (or Description) -> Tool.Description
//   - path/query/header params -> InputSchema properties
//   - request body -> "body" property in InputSchema
//   - path params -> Required
//   - tags[0] -> Category
func (b *Bridge) operationToTool(
	path, method string,
	op *openapi3.Operation,
	params openapi3.Parameters,
) registry.ToolDefinition {
	name := b.toolName(path, method, op)
	description := op.Summary
	if description == "" {
		description = op.Description
	}
	if description == "" {
		description = method + " " + path
	}

	// Build input schema properties and required list.
	properties := make(map[string]any)
	var required []string

	for _, pRef := range params {
		p := pRef.Value
		if p == nil {
			continue
		}
		prop := paramToProperty(p)
		properties[p.Name] = prop
		if p.Required || p.In == "path" {
			required = append(required, p.Name)
		}
	}

	// Handle request body.
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		bodySchema := extractRequestBodySchema(op.RequestBody.Value)
		properties["body"] = bodySchema
		if op.RequestBody.Value.Required {
			required = append(required, "body")
		}
	}

	tool := mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: properties,
		},
	}
	if len(required) > 0 {
		tool.InputSchema.Required = required
	}

	// Determine write status from HTTP method.
	isWrite := method != "GET" && method != "HEAD" && method != "OPTIONS"

	// Extract category from first tag.
	var category string
	if len(op.Tags) > 0 {
		category = op.Tags[0]
	}

	return registry.ToolDefinition{
		Tool:     tool,
		Category: category,
		IsWrite:  isWrite,
	}
}

// toolName generates the MCP tool name from the operation.
func (b *Bridge) toolName(path, method string, op *openapi3.Operation) string {
	switch b.config.NameStyle {
	case "path_method":
		return pathMethodName(path, method)
	default: // "operationId"
		if op.OperationID != "" {
			return op.OperationID
		}
		return pathMethodName(path, method)
	}
}

// pathMethodName generates a snake_case tool name from method + path.
// Example: GET /pets/{petId} -> get_pets_petId
func pathMethodName(path, method string) string {
	sanitized := strings.ReplaceAll(path, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "{", "")
	sanitized = strings.ReplaceAll(sanitized, "}", "")
	sanitized = strings.Trim(sanitized, "_")
	return strings.ToLower(method) + "_" + sanitized
}

// paramToProperty converts an OpenAPI parameter to a JSON Schema property map.
func paramToProperty(p *openapi3.Parameter) map[string]any {
	prop := make(map[string]any)
	if p.Schema != nil && p.Schema.Value != nil {
		prop["type"] = p.Schema.Value.Type.Slice()[0]
		if p.Schema.Value.Enum != nil {
			prop["enum"] = p.Schema.Value.Enum
		}
	} else {
		prop["type"] = "string"
	}
	if p.Description != "" {
		prop["description"] = p.Description
	}
	return prop
}

// extractRequestBodySchema extracts the JSON Schema from a request body.
// If the body has an application/json media type with a schema, its properties
// are inlined. Otherwise a simple {"type": "string"} for the body content is used.
func extractRequestBodySchema(body *openapi3.RequestBody) map[string]any {
	if body.Content != nil {
		if mt := body.Content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
			schema := mt.Schema.Value
			return openAPISchemaToJSONSchema(schema)
		}
	}
	return map[string]any{
		"type":        "string",
		"description": "Request body (JSON string)",
	}
}

// openAPISchemaToJSONSchema converts an OpenAPI Schema to a JSON Schema map
// suitable for MCP tool InputSchema properties.
func openAPISchemaToJSONSchema(schema *openapi3.Schema) map[string]any {
	result := make(map[string]any)

	if len(schema.Type.Slice()) > 0 {
		result["type"] = schema.Type.Slice()[0]
	}
	if schema.Description != "" {
		result["description"] = schema.Description
	}

	// Handle object type with properties.
	if len(schema.Properties) > 0 {
		props := make(map[string]any)
		for name, propRef := range schema.Properties {
			if propRef.Value != nil {
				props[name] = openAPISchemaToJSONSchema(propRef.Value)
			}
		}
		result["properties"] = props
	}
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	// Handle array type with items.
	if schema.Items != nil && schema.Items.Value != nil {
		result["items"] = openAPISchemaToJSONSchema(schema.Items.Value)
	}

	// Handle enums.
	if schema.Enum != nil {
		result["enum"] = schema.Enum
	}

	return result
}
