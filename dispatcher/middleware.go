package dispatcher

import (
	"context"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Middleware returns a registry.Middleware that routes tool calls through the dispatcher.
// Group is resolved from td.RuntimeGroup, falling back to td.CircuitBreakerGroup,
// then Config.GroupFunc. Priority is resolved from Config.PriorityFunc or Config.DefaultPriority.
// Group and priority are determined at middleware-wrapping time (not per-call).
func Middleware(d *Dispatcher) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		// Resolve group at wrap time.
		group := td.RuntimeGroup
		if group == "" {
			group = td.CircuitBreakerGroup
		}
		if group == "" && d.cfg.GroupFunc != nil {
			group = d.cfg.GroupFunc(name, td)
		}

		// Resolve priority at wrap time.
		priority := d.cfg.DefaultPriority
		if d.cfg.PriorityFunc != nil {
			priority = d.cfg.PriorityFunc(name, td)
		}

		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			job := &Job{
				Name:     name,
				TD:       td,
				Ctx:      ctx,
				Request:  request,
				Handler:  next,
				Priority: priority,
				Group:    group,
			}
			return d.Submit(ctx, job)
		}
	}
}
