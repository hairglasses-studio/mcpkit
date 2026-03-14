//go:build official_sdk

package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TypedHandlerFunc is a handler function with typed input and output.
type TypedHandlerFunc[In any, Out any] func(ctx context.Context, input In) (Out, error)

// TypedHandler wraps a typed handler function into a ToolDefinition.
// It auto-generates the input schema from the In type and populates both
// structuredContent and text content from the return value.
func TypedHandler[In any, Out any](name, description string, fn TypedHandlerFunc[In, Out]) registry.ToolDefinition {
	wrapped := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		var input In
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
				return CodedErrorResult(ErrInvalidParam, fmt.Errorf("failed to parse arguments: %w", err)), nil
			}
		}

		output, err := fn(ctx, input)
		if err != nil {
			return ErrorResult(err), nil
		}

		return StructuredResult(output), nil
	}

	// Build the Tool with InputSchema as a map (the official SDK uses `any` for InputSchema)
	inputSchema := generateSchemaMap[In]()

	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        name,
			Description: description,
			InputSchema: inputSchema,
		},
		Handler: wrapped,
	}

	return td
}

// generateSchemaMap generates a JSON Schema map from a Go struct type.
// This produces a map[string]any that the official SDK accepts as InputSchema.
func generateSchemaMap[T any]() map[string]any {
	// Use JSON marshaling of a zero value to introspect the struct
	var zero T
	data, err := json.Marshal(zero)
	if err != nil {
		return map[string]any{"type": "object"}
	}

	// Parse the zero value to discover field names
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return map[string]any{"type": "object"}
	}

	properties := make(map[string]any, len(fields))
	for key, val := range fields {
		properties[key] = inferFieldSchema(val)
	}

	return map[string]any{
		"type":       "object",
		"properties": properties,
	}
}

func inferFieldSchema(val any) map[string]any {
	switch val.(type) {
	case string:
		return map[string]any{"type": "string"}
	case float64:
		return map[string]any{"type": "number"}
	case bool:
		return map[string]any{"type": "boolean"}
	case []any:
		return map[string]any{"type": "array"}
	case map[string]any:
		return map[string]any{"type": "object"}
	default:
		return map[string]any{"type": "string"}
	}
}
