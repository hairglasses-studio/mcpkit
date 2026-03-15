//go:build !official_sdk

package handoff

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware returns a DelegateMiddleware that creates a child span
// for each agent delegation with handoff-specific attributes.
func TracingMiddleware(tracer trace.Tracer) DelegateMiddleware {
	return func(agentName string, next DelegateFunc) DelegateFunc {
		return func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
			ctx, span := tracer.Start(ctx, "handoff.delegate",
				trace.WithAttributes(
					attribute.String("mcp.handoff.agent", agentName),
				),
			)
			defer span.End()

			start := time.Now()
			result, err := next(ctx, agent, req)
			duration := time.Since(start)

			span.SetAttributes(
				attribute.Float64("mcp.handoff.duration_ms", float64(duration.Milliseconds())),
			)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else if result != nil {
				span.SetAttributes(attribute.String("mcp.handoff.status", result.Status))
				if result.Status == "failed" || result.Status == "timeout" {
					span.SetStatus(codes.Error, result.Status)
				}
			}

			return result, err
		}
	}
}
