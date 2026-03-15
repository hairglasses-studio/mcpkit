package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Middleware returns a registry.Middleware that logs tool invocations.
// Each call logs the tool name, category, duration, and whether it succeeded.
func Middleware(logger *slog.Logger) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		category := td.Category
		if category == "" {
			category = "unknown"
		}
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			start := time.Now()
			result, err := next(ctx, request)
			duration := time.Since(start)

			if err != nil {
				logger.ErrorContext(ctx, "tool invocation failed",
					"tool", name,
					"category", category,
					"duration_ms", duration.Milliseconds(),
					"error", err,
				)
			} else if registry.IsResultError(result) {
				logger.WarnContext(ctx, "tool returned error",
					"tool", name,
					"category", category,
					"duration_ms", duration.Milliseconds(),
				)
			} else {
				logger.InfoContext(ctx, "tool invocation",
					"tool", name,
					"category", category,
					"duration_ms", duration.Milliseconds(),
				)
			}

			return result, err
		}
	}
}
