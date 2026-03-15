//go:build official_sdk

package handler

import (
	"github.com/hairglasses-studio/mcpkit/registry"
)

// InputValidationMiddleware returns a no-op middleware for the official SDK build.
// The official SDK uses untyped schemas (ToolInputSchema = any), so field-level
// validation is not yet supported.
func InputValidationMiddleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return next
	}
}
