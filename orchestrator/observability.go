package orchestrator

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware returns a StageMiddleware that creates a child span
// per orchestration stage with stage-specific attributes.
func TracingMiddleware(tracer trace.Tracer) StageMiddleware {
	return func(stageName string, next StageFunc) StageFunc {
		return func(ctx context.Context, input StageInput) (*StageOutput, error) {
			ctx, span := tracer.Start(ctx, "orchestrator.stage",
				trace.WithAttributes(
					attribute.String("mcp.orchestrator.stage", stageName),
				),
			)
			defer span.End()

			start := time.Now()
			output, err := next(ctx, input)
			duration := time.Since(start)

			span.SetAttributes(
				attribute.Float64("mcp.orchestrator.duration_ms", float64(duration.Milliseconds())),
			)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else if output != nil && output.Status == "error" {
				span.SetStatus(codes.Error, output.Error)
			}

			return output, err
		}
	}
}
