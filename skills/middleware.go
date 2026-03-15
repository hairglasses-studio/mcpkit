package skills

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

type loaderKey struct{}

// Middleware returns a registry.Middleware that injects the ContextLoader
// into the request context. Handlers can retrieve it with LoaderFromContext.
func Middleware(loader *ContextLoader) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			ctx = context.WithValue(ctx, loaderKey{}, loader)
			return next(ctx, req)
		}
	}
}

// LoaderFromContext extracts the ContextLoader from the context, or nil if absent.
func LoaderFromContext(ctx context.Context) *ContextLoader {
	l, _ := ctx.Value(loaderKey{}).(*ContextLoader)
	return l
}
