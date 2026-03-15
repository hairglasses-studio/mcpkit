package observability

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// buildTestProvider creates a Provider backed by an in-memory metric reader
// for test introspection.
func buildTestProvider(t *testing.T) (*Provider, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")
	metrics, err := createMetrics(meter)
	if err != nil {
		t.Fatalf("createMetrics: %v", err)
	}
	return &Provider{metrics: metrics}, reader
}

// TestMiddleware_RecordsMetrics verifies that wrapping and invoking a tool
// handler via the middleware records mcp_tool_invocations and
// mcp_tool_duration_seconds, and leaves mcp_tools_active at zero (net effect).
func TestMiddleware_RecordsMetrics(t *testing.T) {
	ctx := context.Background()
	p, reader := buildTestProvider(t)

	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "test-tool"},
		Category: "test-cat",
	}

	called := false
	wrapped := mw("test-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	result, err := wrapped(ctx, registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !called {
		t.Fatal("inner handler was not called")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// mcp_tool_invocations should be 1.
	inv := findMetric(rm, "mcp_tool_invocations")
	if inv == nil {
		t.Fatal("mcp_tool_invocations not found")
	}
	sum, ok := inv.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64] for mcp_tool_invocations, got %T", inv.Data)
	}
	if len(sum.DataPoints) == 0 || sum.DataPoints[0].Value != 1 {
		val := int64(0)
		if len(sum.DataPoints) > 0 {
			val = sum.DataPoints[0].Value
		}
		t.Errorf("expected mcp_tool_invocations=1, got %d", val)
	}

	// mcp_tool_duration_seconds should have a data point.
	dur := findMetric(rm, "mcp_tool_duration_seconds")
	if dur == nil {
		t.Fatal("mcp_tool_duration_seconds not found")
	}
	hist, ok := dur.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("expected Histogram[float64] for mcp_tool_duration_seconds, got %T", dur.Data)
	}
	if len(hist.DataPoints) == 0 {
		t.Fatal("expected at least one histogram data point for mcp_tool_duration_seconds")
	}

	// mcp_tools_active net effect should be zero (incremented then decremented).
	active := findMetric(rm, "mcp_tools_active")
	if active != nil {
		gauge, ok := active.Data.(metricdata.Sum[int64])
		if ok && len(gauge.DataPoints) > 0 && gauge.DataPoints[0].Value != 0 {
			t.Errorf("expected mcp_tools_active=0 (net) after handler completes, got %d", gauge.DataPoints[0].Value)
		}
	}
}

// TestMiddleware_SetsSpan verifies that when a tracer is configured, the
// middleware creates a span with the correct name.
func TestMiddleware_SetsSpan(t *testing.T) {
	ctx := context.Background()

	// Set up in-memory trace exporter.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	// Set up in-memory metrics.
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")
	metrics, err := createMetrics(meter)
	if err != nil {
		t.Fatalf("createMetrics: %v", err)
	}

	p := &Provider{
		tracer:  tp.Tracer("test"),
		metrics: metrics,
	}

	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "traced-tool"},
		Category: "trace-cat",
	}

	wrapped := mw("traced-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("traced"), nil
	})

	if _, err := wrapped(ctx, registry.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span to be exported")
	}
	if spans[0].Name != "traced-tool" {
		t.Errorf("expected span name 'traced-tool', got %q", spans[0].Name)
	}
}

// TestMiddleware_EmptyCategory verifies that when ToolDefinition has an empty
// Category, the middleware substitutes "unknown" as the category attribute on
// recorded metrics.
func TestMiddleware_EmptyCategory(t *testing.T) {
	ctx := context.Background()
	p, reader := buildTestProvider(t)

	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "no-cat-tool"},
		Category: "", // empty — should become "unknown"
	}

	wrapped := mw("no-cat-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	if _, err := wrapped(ctx, registry.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	inv := findMetric(rm, "mcp_tool_invocations")
	if inv == nil {
		t.Fatal("mcp_tool_invocations not found")
	}
	sum, ok := inv.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64], got %T", inv.Data)
	}
	if len(sum.DataPoints) == 0 {
		t.Fatal("expected at least one data point")
	}

	// Confirm the category attribute is "unknown".
	foundCategory := false
	for _, attr := range sum.DataPoints[0].Attributes.ToSlice() {
		if string(attr.Key) == "category" {
			foundCategory = true
			if attr.Value.AsString() != "unknown" {
				t.Errorf("expected category=unknown for empty ToolDefinition.Category, got %q", attr.Value.AsString())
			}
		}
	}
	if !foundCategory {
		t.Error("category attribute not found on mcp_tool_invocations data point")
	}
}
