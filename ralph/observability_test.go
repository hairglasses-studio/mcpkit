package ralph

import (
	"errors"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingHooks_IterationSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	hooks := TracingHooks(tracer)

	// Simulate an iteration lifecycle.
	hooks.OnIterationStart(1)
	hooks.OnIterationEnd(IterationLog{
		Iteration: 1,
		TaskID:    "task-a",
		Result:    "done",
	})

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	attrMap := make(map[string]interface{})
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if v := attrMap["mcp.ralph.iteration"]; v != int64(1) {
		t.Errorf("mcp.ralph.iteration: expected 1, got %v", v)
	}
	if v := attrMap["mcp.ralph.task_id"]; v != "task-a" {
		t.Errorf("mcp.ralph.task_id: expected task-a, got %v", v)
	}
	if v := attrMap["mcp.ralph.status"]; v != "ok" {
		t.Errorf("mcp.ralph.status: expected ok, got %v", v)
	}
}

func TestTracingHooks_ErrorSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	hooks := TracingHooks(tracer)

	hooks.OnIterationStart(2)
	hooks.OnError(2, errors.New("tool not found"))

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	attrMap := make(map[string]interface{})
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if v := attrMap["mcp.ralph.status"]; v != "error" {
		t.Errorf("mcp.ralph.status: expected error, got %v", v)
	}

	if len(spans[0].Events) == 0 {
		t.Error("expected error event on span")
	}
}

func TestTracingHooks_NoSpanForUnstartedIteration(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	hooks := TracingHooks(tracer)

	// End without start — should not panic.
	hooks.OnIterationEnd(IterationLog{Iteration: 99})
	hooks.OnError(99, errors.New("orphan"))

	spans := exporter.GetSpans()
	if len(spans) != 0 {
		t.Errorf("expected 0 spans for unstarted iteration, got %d", len(spans))
	}
}
