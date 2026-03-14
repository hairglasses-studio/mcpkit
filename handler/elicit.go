package handler

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// ElicitForm creates a form-mode elicitation request from a Go struct or schema map.
// The schema must be a flat object with string/number/boolean/enum properties per the MCP spec.
//
// Usage:
//
//	req := handler.ElicitForm("Please provide your name", map[string]any{
//	    "type": "object",
//	    "properties": map[string]any{
//	        "name":  map[string]any{"type": "string", "description": "Your name"},
//	        "agree": map[string]any{"type": "boolean"},
//	    },
//	    "required": []string{"name"},
//	})
func ElicitForm(message string, schema any) mcp.ElicitationParams {
	return mcp.ElicitationParams{
		Mode:            mcp.ElicitationModeForm,
		Message:         message,
		RequestedSchema: schema,
	}
}

// ElicitURL creates a URL-mode elicitation request.
// The elicitationID uniquely identifies this request for completion tracking.
func ElicitURL(message, elicitationID, url string) mcp.ElicitationParams {
	return mcp.ElicitationParams{
		Mode:          mcp.ElicitationModeURL,
		Message:       message,
		ElicitationID: elicitationID,
		URL:           url,
	}
}

// ElicitFormSchema builds a flat JSON Schema for form elicitation from field definitions.
// This is a convenience wrapper that constructs a valid form schema.
func ElicitFormSchema(fields ...FormField) map[string]any {
	properties := make(map[string]any, len(fields))
	var required []string

	for _, f := range fields {
		prop := map[string]any{
			"type": f.Type,
		}
		if f.Description != "" {
			prop["description"] = f.Description
		}
		if len(f.Enum) > 0 {
			prop["enum"] = f.Enum
		}
		if f.Default != nil {
			prop["default"] = f.Default
		}
		properties[f.Name] = prop
		if f.Required {
			required = append(required, f.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// FormField defines a single field in an elicitation form schema.
type FormField struct {
	Name        string
	Type        string // "string", "number", "boolean"
	Description string
	Required    bool
	Enum        []string
	Default     any
}
