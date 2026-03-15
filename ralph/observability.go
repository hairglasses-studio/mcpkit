package ralph

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingHooks returns a Hooks that creates OpenTelemetry spans for each
// loop iteration.
//
// Each iteration gets a span named "ralph.iteration" with attributes:
//   - mcp.ralph.iteration: the iteration number
//   - mcp.ralph.task_id: task ID (when present in the iteration log)
//   - mcp.ralph.status: "ok" or "error"
//
// Note: because the Hooks API does not pass a parent context, spans are
// rooted at the tracer's provider rather than under a request span.
func TracingHooks(tracer trace.Tracer) Hooks {
	// Sequential iteration guarantees — no concurrent access to this map.
	spans := make(map[int]trace.Span)

	return Hooks{
		OnIterationStart: func(iteration int) {
			_, span := tracer.Start(context.Background(), "ralph.iteration",
				trace.WithAttributes(
					attribute.Int("mcp.ralph.iteration", iteration),
				),
			)
			spans[iteration] = span
		},
		OnIterationEnd: func(entry IterationLog) {
			span, ok := spans[entry.Iteration]
			if !ok {
				return
			}
			if entry.TaskID != "" {
				span.SetAttributes(attribute.String("mcp.ralph.task_id", entry.TaskID))
			}
			span.SetAttributes(attribute.String("mcp.ralph.status", "ok"))
			span.End()
			delete(spans, entry.Iteration)
		},
		OnError: func(iteration int, err error) {
			span, ok := spans[iteration]
			if !ok {
				return
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(attribute.String("mcp.ralph.status", "error"))
			span.End()
			delete(spans, iteration)
		},
	}
}
