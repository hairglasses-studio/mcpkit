//go:build !official_sdk

package handler

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// InputValidationMiddleware returns a registry.Middleware that validates
// request arguments against td.Tool.InputSchema BEFORE calling the handler.
// Tools with no schema properties and no required fields are passed through unchanged.
func InputValidationMiddleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		// Short-circuit at wrap time if no schema content
		if len(td.Tool.InputSchema.Properties) == 0 && len(td.Tool.InputSchema.Required) == 0 {
			return next
		}

		schema := td.Tool.InputSchema

		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			args := registry.ExtractArguments(request)

			// Check required fields
			for _, field := range schema.Required {
				if args == nil {
					return CodedErrorResult(ErrInvalidParam, fmt.Errorf("missing required field %q", field)), nil
				}
				if _, ok := args[field]; !ok {
					return CodedErrorResult(ErrInvalidParam, fmt.Errorf("missing required field %q", field)), nil
				}
			}

			// Check property types for provided args
			if args != nil && schema.Properties != nil {
				for propName, propSchema := range schema.Properties {
					val, ok := args[propName]
					if !ok {
						continue // Not present; required check already handled above
					}

					propMap, ok := propSchema.(map[string]any)
					if !ok {
						continue // Schema property isn't a map, skip
					}

					expectedType, ok := propMap["type"].(string)
					if !ok {
						continue // No type declaration, skip
					}

					if err := checkType(propName, val, expectedType); err != nil {
						return CodedErrorResult(ErrInvalidParam, err), nil
					}
				}
			}

			return next(ctx, request)
		}
	}
}
