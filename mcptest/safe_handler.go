//go:build !official_sdk

package mcptest

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// SafeHandlerMiddleware returns a registry.Middleware that wraps any handler
// which returns (nil, error) into a proper (*CallToolResult, nil) response.
// This enforces the MCP handler contract: tools must never return (nil, error).
//
// The middleware converts the error into a CodedErrorResult with the
// SAFE_HANDLER_RECOVERY code, preserving the original error message.
func SafeHandlerMiddleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			result, err := next(ctx, req)
			if result == nil {
				msg := "handler returned nil result"
				if err != nil {
					msg = fmt.Sprintf("handler returned nil result with error: %v", err)
				}
				return registry.MakeErrorResult(fmt.Sprintf("[SAFE_HANDLER_RECOVERY] %s", msg)), nil
			}
			// If we have a result but also an error, clear the error
			// since the MCP contract is (*result, nil).
			if err != nil {
				return result, nil
			}
			return result, nil
		}
	}
}
