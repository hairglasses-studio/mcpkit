//go:build official_sdk

package handler

import (
	"github.com/hairglasses-studio/mcpkit/registry"
)

// OutputValidationMiddleware returns a no-op middleware for the official SDK build.
// The official SDK uses untyped schemas (ToolOutputSchema = any), so field-level
// validation is not yet supported.
func OutputValidationMiddleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return next
	}
}
