package resilience

import (
	"context"
	"errors"
	"fmt"

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
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			if err := reg.Get(td.CircuitBreakerGroup).Wait(ctx); err != nil {
				return registry.MakeErrorResult(fmt.Sprintf("rate limited: %v", err)), nil
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
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			var result *registry.CallToolResult
			var handlerErr error

			cb := reg.Get(td.CircuitBreakerGroup)
			cbErr := cb.Execute(ctx, func(cbCtx context.Context) error {
				result, handlerErr = next(cbCtx, request)
				if handlerErr != nil {
					return handlerErr
				}
				if registry.IsResultError(result) {
					return errors.New("tool returned error result")
				}
				return nil
			})

			if cbErr != nil && errors.Is(cbErr, ErrCircuitOpen) {
				return registry.MakeErrorResult(
					fmt.Sprintf("[CIRCUIT_OPEN] %s service is temporarily unavailable (circuit breaker open)", td.CircuitBreakerGroup),
				), nil
			}

			return result, handlerErr
		}
	}
}
