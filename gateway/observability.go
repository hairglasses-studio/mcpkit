//go:build !official_sdk

package gateway

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TracingMiddleware returns a registry.Middleware that creates a child span
// for each proxied tool call with gateway-specific attributes.
func TracingMiddleware(tracer trace.Tracer) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			upstream := name
			originalTool := name
			if idx := strings.Index(name, "."); idx > 0 {
				upstream = name[:idx]
				originalTool = name[idx+1:]
			}

			ctx, span := tracer.Start(ctx, "gateway.proxy",
				trace.WithAttributes(
					attribute.String("mcp.gateway.upstream", upstream),
					attribute.String("mcp.gateway.tool.original_name", originalTool),
				),
			)
			defer span.End()

			start := time.Now()
			result, err := next(ctx, request)
			duration := time.Since(start)

			span.SetAttributes(
				attribute.Float64("mcp.gateway.duration_ms", float64(duration.Milliseconds())),
			)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else if result != nil && result.IsError {
				span.SetStatus(codes.Error, "tool returned error result")
			}

			return result, err
		}
	}
}
