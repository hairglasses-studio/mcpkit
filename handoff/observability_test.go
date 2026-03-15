//go:build !official_sdk

package handoff

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingMiddleware_SpanAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	base := mockDelegate("completed")
	mw := TracingMiddleware(tracer)
	wrapped := mw("test-agent", base)

	agent := AgentRef{Name: "test-agent", Description: "test"}
	req := HandoffRequest{TaskDescription: "test task"}

	result, err := wrapped(context.Background(), agent, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected status completed, got %s", result.Status)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	attrMap := make(map[string]interface{})
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if v, ok := attrMap["mcp.handoff.agent"]; !ok || v != "test-agent" {
		t.Errorf("mcp.handoff.agent: expected test-agent, got %v", v)
	}
	if v, ok := attrMap["mcp.handoff.status"]; !ok || v != "completed" {
		t.Errorf("mcp.handoff.status: expected completed, got %v", v)
	}
	if _, ok := attrMap["mcp.handoff.duration_ms"]; !ok {
		t.Error("expected mcp.handoff.duration_ms attribute")
	}
}

func TestTracingMiddleware_ErrorSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	errDelegate := func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
		return nil, ErrAgentNotFound
	}

	mw := TracingMiddleware(tracer)
	wrapped := mw("missing-agent", errDelegate)

	_, err := wrapped(context.Background(), AgentRef{Name: "missing"}, HandoffRequest{})
	if err == nil {
		t.Fatal("expected error")
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	// Verify span recorded the error
	if len(spans[0].Events) == 0 {
		t.Error("expected error event on span")
	}
}
