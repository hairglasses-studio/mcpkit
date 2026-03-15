package workflow

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware returns a NodeMiddleware that creates a child span
// per workflow node execution with node-specific attributes.
func TracingMiddleware(tracer trace.Tracer) NodeMiddleware {
	return func(nodeName string, next NodeFunc) NodeFunc {
		return func(ctx context.Context, state State) (State, error) {
			ctx, span := tracer.Start(ctx, "workflow.node",
				trace.WithAttributes(
					attribute.String("mcp.workflow.node", nodeName),
					attribute.Int("mcp.workflow.step", state.Step),
				),
			)
			defer span.End()

			start := time.Now()
			newState, err := next(ctx, state)
			duration := time.Since(start)

			span.SetAttributes(
				attribute.Float64("mcp.workflow.duration_ms", float64(duration.Milliseconds())),
			)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return newState, err
		}
	}
}
