//go:build !official_sdk

package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TypedHandlerFunc is a handler function with typed input and output.
type TypedHandlerFunc[In any, Out any] func(ctx context.Context, input In) (Out, error)

// TypedHandler wraps a typed handler function into a ToolDefinition.
// It auto-generates the outputSchema from the Out type via JSON schema reflection,
// and populates both structuredContent and content[0].text from the return value.
//
// Usage:
//
//	type SearchInput struct {
//	    Query string `json:"query" jsonschema:"required,description=Search query"`
//	}
//	type SearchOutput struct {
//	    Results []string `json:"results"`
//	    Total   int      `json:"total"`
//	}
//
//	td := handler.TypedHandler[SearchInput, SearchOutput](
//	    "search",
//	    "Search for items",
//	    func(ctx context.Context, input SearchInput) (SearchOutput, error) {
//	        return SearchOutput{Results: []string{"a"}, Total: 1}, nil
//	    },
//	)
func TypedHandler[In any, Out any](name, description string, fn TypedHandlerFunc[In, Out]) registry.ToolDefinition {
	inputSchema := generateInputSchema[In]()
	outputSchema := generateOutputSchema[Out]()

	wrapped := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input In
		if req.Params.Arguments != nil {
			argBytes, err := json.Marshal(req.Params.Arguments)
			if err != nil {
				return ErrorResult(fmt.Errorf("failed to marshal arguments: %w", err)), nil
			}
			if err := json.Unmarshal(argBytes, &input); err != nil {
				hint := ""
				if hb, e := json.Marshal(inputSchema); e == nil {
					hint = "\nExpected schema: " + string(hb)
				}
				return CodedErrorResult(ErrInvalidParam, fmt.Errorf("failed to parse arguments: %w%s", err, hint)), nil
			}
		}

		output, err := fn(ctx, input)
		if err != nil {
			return ErrorResult(err), nil
		}

		return StructuredResult(output), nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        name,
			Description: description,
			InputSchema: mcp.ToolInputSchema(inputSchema),
		},
		Handler:      wrapped,
		OutputSchema: outputSchema,
	}

	return td
}

// generateInputSchema generates a ToolArgumentsSchema from a Go struct type.
func generateInputSchema[T any]() mcp.ToolArgumentsSchema {
	r := jsonschema.Reflector{DoNotReference: true}
	schema := r.Reflect(new(T))

	props := make(map[string]any)
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			props[pair.Key] = schemaToMap(pair.Value)
		}
	}

	var required []string
	if len(schema.Required) > 0 {
		required = schema.Required
	}

	return mcp.ToolArgumentsSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

// generateOutputSchema generates a ToolOutputSchema from a Go struct type.
func generateOutputSchema[T any]() *mcp.ToolOutputSchema {
	r := jsonschema.Reflector{DoNotReference: true}
	schema := r.Reflect(new(T))

	props := make(map[string]any)
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			props[pair.Key] = schemaToMap(pair.Value)
		}
	}

	var required []string
	if len(schema.Required) > 0 {
		required = schema.Required
	}

	out := &mcp.ToolOutputSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
	return out
}

// schemaToMap converts a jsonschema.Schema to a map for use in mcp schemas.
func schemaToMap(s *jsonschema.Schema) map[string]any {
	m := make(map[string]any)

	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Default != nil {
		m["default"] = s.Default
	}

	// Handle array items
	if s.Items != nil && s.Items.Type != "" {
		m["items"] = schemaToMap(s.Items)
	}

	// Handle nested object properties
	if s.Properties != nil && s.Properties.Len() > 0 {
		props := make(map[string]any)
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			props[pair.Key] = schemaToMap(pair.Value)
		}
		m["properties"] = props
		if len(s.Required) > 0 {
			m["required"] = s.Required
		}
	}

	return m
}
