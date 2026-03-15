package memory

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

type contextKey struct{}

// Middleware returns a registry.Middleware that injects store into the request
// context. Handlers can retrieve it with FromContext.
func Middleware(store Store) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			ctx = context.WithValue(ctx, contextKey{}, store)
			return next(ctx, req)
		}
	}
}

// FromContext retrieves the Store injected by Middleware. Returns nil if no
// Store is present in ctx.
func FromContext(ctx context.Context) Store {
	s, _ := ctx.Value(contextKey{}).(Store)
	return s
}
