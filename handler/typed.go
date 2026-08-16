//go:build !official_sdk

package handler

import (
	"context"
	"encoding/json"
	"fmt"

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
// The reflection itself lives in schema_reflect.go (untagged) so the
// official_sdk build derives properties from the exact same code -- see that
// file's header for the P78.38 divergence this sharing exists to prevent.
func generateInputSchema[T any]() mcp.ToolArgumentsSchema {
	props, required := reflectSchemaProperties[T]()
	return mcp.ToolArgumentsSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

// generateOutputSchema generates a ToolOutputSchema from a Go struct type,
// from the same shared reflection as generateInputSchema.
func generateOutputSchema[T any]() *mcp.ToolOutputSchema {
	props, required := reflectSchemaProperties[T]()
	return &mcp.ToolOutputSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}
