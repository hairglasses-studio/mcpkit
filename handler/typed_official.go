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
	inputSchema := reflectSchemaMap[In]()

	// OutputSchema, same map shape as InputSchema (the official SDK uses `any`
	// for both). This was previously never populated on this build — every
	// TypedHandler tool advertised no output schema at all, unlike the mcp-go
	// side's generateOutputSchema. registry.ToolDefinition.OutputSchema is
	// *ToolOutputSchema (= *any here); ApplyToolMetadata (registry/metadata.go,
	// portable) copies *td.OutputSchema into td.Tool.OutputSchema at
	// registration time on both tags, so setting it here is sufficient — no
	// other wiring needed.
	var outputSchema registry.ToolOutputSchema = reflectSchemaMap[Out]()

	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        name,
			Description: description,
			InputSchema: inputSchema,
		},
		Handler:      wrapped,
		OutputSchema: &outputSchema,
	}

	return td
}

// NOTE (P78.38, 2026-08-16): generateSchemaMap/inferFieldSchema used to live
// here, deriving the schema by marshaling a ZERO VALUE of the type and
// enumerating the resulting JSON keys. That silently dropped every
// `json:",omitempty"` field -- which is most of them -- along with all
// descriptions, enums, defaults and required-ness, and mistyped anything
// that marshaled to null. Both schemas now come from schema_reflect.go's
// reflectSchemaMap, the same reflection typed.go (mcp-go) uses, so the two
// builds cannot drift apart again. See schema_reflect.go's header for the
// measured before/after.
