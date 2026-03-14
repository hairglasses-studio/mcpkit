package observability

import (
	"context"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Middleware returns a registry.Middleware that records OTel metrics and
// tracing spans for every tool invocation.
func (p *Provider) Middleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		category := td.Category
		if category == "" {
			category = "unknown"
		}
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			ctx, span := p.StartSpan(ctx, name)
			if span != nil {
				defer span.End()
			}

			p.StartToolExecution(ctx, name, category)
			defer p.EndToolExecution(ctx, name, category)

			start := time.Now()
			result, err := next(ctx, request)
			p.RecordToolInvocation(ctx, name, category, time.Since(start), err)

			return result, err
		}
	}
}
