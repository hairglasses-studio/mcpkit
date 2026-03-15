package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/registry"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
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

// TestMiddleware_HandlerError verifies that when the inner handler returns an
// error the middleware propagates it and still records mcp_tool_errors.
func TestMiddleware_HandlerError(t *testing.T) {
	ctx := context.Background()
	p, reader := buildTestProvider(t)

	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "err-tool"},
		Category: "test",
	}

	handlerErr := errors.New("timeout: upstream unavailable")
	wrapped := mw("err-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, handlerErr
	})

	_, err := wrapped(ctx, registry.CallToolRequest{})
	if err != handlerErr {
		t.Fatalf("expected handlerErr to be propagated, got: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	errs := findMetric(rm, "mcp_tool_errors")
	if errs == nil {
		t.Fatal("mcp_tool_errors metric not found")
	}
	errSum, ok := errs.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64] for mcp_tool_errors, got %T", errs.Data)
	}
	if len(errSum.DataPoints) == 0 || errSum.DataPoints[0].Value != 1 {
		val := int64(0)
		if len(errSum.DataPoints) > 0 {
			val = errSum.DataPoints[0].Value
		}
		t.Errorf("expected mcp_tool_errors=1 after handler error, got %d", val)
	}
}

// TestMiddleware_HandlerError_WithTracer verifies that a span is still created
// and ended when the inner handler returns an error (and no panic occurs).
func TestMiddleware_HandlerError_WithTracer(t *testing.T) {
	ctx := context.Background()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")
	metrics, err := createMetrics(meter)
	if err != nil {
		t.Fatalf("createMetrics: %v", err)
	}

	p := &Provider{tracer: tp.Tracer("test"), metrics: metrics}
	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "err-traced-tool"},
		Category: "test",
	}

	handlerErr := errors.New("something went wrong")
	wrapped := mw("err-traced-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, handlerErr
	})

	if _, err := wrapped(ctx, registry.CallToolRequest{}); err == nil {
		t.Fatal("expected error to be returned")
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span even when handler errors")
	}
	if spans[0].Name != "err-traced-tool" {
		t.Errorf("expected span name 'err-traced-tool', got %q", spans[0].Name)
	}
}

// TestAddGenAISpanAttrs_NilSpan verifies addGenAISpanAttrs is a no-op on nil span.
func TestAddGenAISpanAttrs_NilSpan(t *testing.T) {
	// Must not panic.
	addGenAISpanAttrs(nil, finops.TokenUsage{InputTokens: 10, OutputTokens: 20, Model: "m"})
}

// TestAddGenAISpanAttrs_NonRecordingSpan verifies addGenAISpanAttrs is a no-op
// when span.IsRecording() returns false (e.g. a noop span).
func TestAddGenAISpanAttrs_NonRecordingSpan(t *testing.T) {
	// trace.SpanFromContext on a plain background context returns a noop span
	// that is not recording.
	noopSpan := trace.SpanFromContext(context.Background())
	if noopSpan.IsRecording() {
		t.Skip("noop span unexpectedly reports IsRecording=true")
	}
	// Must not panic.
	addGenAISpanAttrs(noopSpan, finops.TokenUsage{InputTokens: 5, OutputTokens: 15, Model: "x"})
}

// TestAddGenAISpanAttrs_RecordingSpan_WithModel verifies addGenAISpanAttrs sets
// all three GenAI attributes on a recording span when Model is non-empty.
func TestAddGenAISpanAttrs_RecordingSpan_WithModel(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-span")
	addGenAISpanAttrs(span, finops.TokenUsage{
		InputTokens:  100,
		OutputTokens: 200,
		Model:        "claude-3-opus",
	})
	span.End()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}

	attrMap := make(map[string]interface{}, len(spans[0].Attributes))
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if v, ok := attrMap["gen_ai.usage.input_tokens"]; !ok || v != int64(100) {
		t.Errorf("gen_ai.usage.input_tokens: expected 100, got %v (present=%v)", v, ok)
	}
	if v, ok := attrMap["gen_ai.usage.output_tokens"]; !ok || v != int64(200) {
		t.Errorf("gen_ai.usage.output_tokens: expected 200, got %v (present=%v)", v, ok)
	}
	if v, ok := attrMap["gen_ai.request.model"]; !ok || v != "claude-3-opus" {
		t.Errorf("gen_ai.request.model: expected 'claude-3-opus', got %v (present=%v)", v, ok)
	}
}

// TestAddGenAISpanAttrs_RecordingSpan_NoModel verifies addGenAISpanAttrs omits
// gen_ai.request.model when TokenUsage.Model is empty.
func TestAddGenAISpanAttrs_RecordingSpan_NoModel(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-span-no-model")
	addGenAISpanAttrs(span, finops.TokenUsage{
		InputTokens:  50,
		OutputTokens: 75,
		// Model is empty
	})
	span.End()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}

	attrMap := make(map[string]interface{}, len(spans[0].Attributes))
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if v, ok := attrMap["gen_ai.usage.input_tokens"]; !ok || v != int64(50) {
		t.Errorf("gen_ai.usage.input_tokens: expected 50, got %v (present=%v)", v, ok)
	}
	if v, ok := attrMap["gen_ai.usage.output_tokens"]; !ok || v != int64(75) {
		t.Errorf("gen_ai.usage.output_tokens: expected 75, got %v (present=%v)", v, ok)
	}
	if _, modelPresent := attrMap["gen_ai.request.model"]; modelPresent {
		t.Error("gen_ai.request.model should not be set when Model is empty")
	}
}

// TestInit_MetricsWithOTLPEndpoint verifies that Init succeeds when
// EnableMetrics=true and OTLPEndpoint is set, covering the OTLP metric
// exporter branch. Since gRPC connects lazily the call succeeds even with a
// non-reachable endpoint.
func TestInit_MetricsWithOTLPEndpoint(t *testing.T) {
	p, shutdown, err := Init(context.Background(), Config{
		ServiceName:   "test-service",
		EnableMetrics: true,
		OTLPEndpoint:  "localhost:14317", // non-reachable, but gRPC connects lazily
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Provider")
	}
	if p.metrics == nil {
		t.Error("expected metrics to be initialized")
	}
	// Shutdown should not block since the OTLP exporter connection is lazy.
	ctx, cancel := context.WithTimeout(context.Background(), 2e9) // 2s
	defer cancel()
	_ = shutdown(ctx) // error is acceptable (unreachable endpoint)
}

// TestInit_TracingWithOTLPEndpoint verifies that Init sets up a tracer when
// EnableTracing=true and OTLPEndpoint is non-empty. gRPC connects lazily so
// the call succeeds even with a non-reachable endpoint.
func TestInit_TracingWithOTLPEndpoint(t *testing.T) {
	p, shutdown, err := Init(context.Background(), Config{
		ServiceName:   "test-service",
		EnableTracing: true,
		OTLPEndpoint:  "localhost:14317", // non-reachable, gRPC lazy connect
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Provider")
	}
	if p.tracer == nil {
		t.Error("expected tracer to be initialized when EnableTracing=true and OTLPEndpoint is set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2e9) // 2s
	defer cancel()
	_ = shutdown(ctx) // error acceptable (unreachable endpoint flush)
}

// TestInit_BothEnabled_WithOTLPEndpoint verifies that Init configures both
// tracing and metrics when all options are set.
func TestInit_BothEnabled_WithOTLPEndpoint(t *testing.T) {
	p, shutdown, err := Init(context.Background(), Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		EnableTracing:  true,
		EnableMetrics:  true,
		OTLPEndpoint:   "localhost:14317",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Provider")
	}
	if p.tracer == nil {
		t.Error("expected tracer to be initialized")
	}
	if p.metrics == nil {
		t.Error("expected metrics to be initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2e9)
	defer cancel()
	_ = shutdown(ctx)
}

// TestInit_NothingEnabled verifies Init succeeds and returns a working
// shutdown func when both EnableTracing and EnableMetrics are false.
func TestInit_NothingEnabled(t *testing.T) {
	p, shutdown, err := Init(context.Background(), Config{
		ServiceName: "test-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Provider")
	}
	if p.metrics != nil {
		t.Error("expected nil metrics when EnableMetrics=false")
	}
	if p.tracer != nil {
		t.Error("expected nil tracer when EnableTracing=false")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

// TestInit_MetricsWithPrometheusPort verifies that Init launches the Prometheus
// goroutine when PrometheusPort is non-empty. The goroutine starts in the
// background; we only verify Init returns without error and the metrics field
// is populated.
func TestInit_MetricsWithPrometheusPort(t *testing.T) {
	// Use a high ephemeral port unlikely to conflict; the goroutine will exit
	// quickly if anything goes wrong, and we don't block on it.
	p, shutdown, err := Init(context.Background(), Config{
		ServiceName:    "test-service",
		EnableMetrics:  true,
		PrometheusPort: "19091",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Provider")
	}
	if p.metrics == nil {
		t.Error("expected metrics to be initialized when EnableMetrics=true")
	}
	_ = shutdown(context.Background())
}
