package resilience

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// RateLimitMiddleware returns a registry.Middleware that rate-limits tool
// execution using the tool's CircuitBreakerGroup as the service key.
// If the tool has no CircuitBreakerGroup, the middleware is a no-op.
func RateLimitMiddleware(reg *RateLimitRegistry) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		if td.CircuitBreakerGroup == "" {
			return next
		}
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := reg.Get(td.CircuitBreakerGroup).Wait(ctx); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("rate limited: %v", err)), nil
			}
			return next(ctx, request)
		}
	}
}

// CircuitBreakerMiddleware returns a registry.Middleware that wraps tool
// execution with a circuit breaker from the given registry.
// If the tool has no CircuitBreakerGroup, the middleware is a no-op.
func CircuitBreakerMiddleware(reg *CircuitBreakerRegistry) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		if td.CircuitBreakerGroup == "" {
			return next
		}
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var result *mcp.CallToolResult
			var handlerErr error

			cb := reg.Get(td.CircuitBreakerGroup)
			cbErr := cb.Execute(ctx, func(cbCtx context.Context) error {
				result, handlerErr = next(cbCtx, request)
				if handlerErr != nil {
					return handlerErr
				}
				if result != nil && result.IsError {
					return errors.New("tool returned error result")
				}
				return nil
			})

			if cbErr != nil && errors.Is(cbErr, ErrCircuitOpen) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: fmt.Sprintf("[CIRCUIT_OPEN] %s service is temporarily unavailable (circuit breaker open)", td.CircuitBreakerGroup),
						},
					},
					IsError: true,
				}, nil
			}

			return result, handlerErr
		}
	}
}
