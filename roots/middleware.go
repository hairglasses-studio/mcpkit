package roots

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Middleware returns a registry.Middleware that injects a RootsClient into
// the handler context, so tool handlers can call ClientFromContext(ctx).
func Middleware(client RootsClient) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			if client != nil {
				ctx = WithRootsClient(ctx, client)
			}
			return next(ctx, request)
		}
	}
}
