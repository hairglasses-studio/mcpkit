package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestInit_RequiresServiceName verifies that Init returns an error when
// ServiceName is empty.
func TestInit_RequiresServiceName(t *testing.T) {
	_, _, err := Init(context.Background(), Config{})
	if err == nil {
		t.Fatal("expected error for empty ServiceName, got nil")
	}
	if !containsStr(err.Error(), "ServiceName is required") {
		t.Errorf("expected error to contain 'ServiceName is required', got: %v", err)
	}
}

// TestInit_MetricsOnly verifies Init succeeds with only metrics enabled
// and no OTLP endpoint.
func TestInit_MetricsOnly(t *testing.T) {
	p, shutdown, err := Init(context.Background(), Config{
		ServiceName:   "test-service",
		EnableMetrics: true,
		EnableTracing: false,
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
	if p.tracer != nil {
		t.Error("expected tracer to be nil when tracing is not enabled")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

// TestInit_TracingWithoutEndpoint verifies that tracing is NOT set up when
// EnableTracing is true but OTLPEndpoint is empty (the code requires both).
func TestInit_TracingWithoutEndpoint(t *testing.T) {
	p, shutdown, err := Init(context.Background(), Config{
		ServiceName:   "test-service",
		EnableTracing: true,
		// No OTLPEndpoint — tracing should not be configured
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background()) //nolint:errcheck
	if p.tracer != nil {
		t.Error("expected tracer to be nil when OTLPEndpoint is empty")
	}
}

// TestRecordToolInvocation_NilProvider verifies no panic when called on nil.
func TestRecordToolInvocation_NilProvider(t *testing.T) {
	var p *Provider
	// Must not panic.
	p.RecordToolInvocation(context.Background(), "tool", "cat", 10*time.Millisecond, nil)
}

// TestRecordToolInvocation_NilMetrics verifies no panic when metrics field is nil.
func TestRecordToolInvocation_NilMetrics(t *testing.T) {
	p := &Provider{}
	// Must not panic.
	p.RecordToolInvocation(context.Background(), "tool", "cat", 10*time.Millisecond, nil)
}

// TestRecordToolInvocation_RecordsMetrics verifies that a successful invocation
// increments mcp_tool_invocations and records mcp_tool_duration_seconds, but
// does NOT increment mcp_tool_errors.
func TestRecordToolInvocation_RecordsMetrics(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")

	metrics, err := createMetrics(meter)
	if err != nil {
		t.Fatalf("createMetrics: %v", err)
	}
	p := &Provider{metrics: metrics}

	p.RecordToolInvocation(ctx, "test-tool", "test-category", 100*time.Millisecond, nil)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	invocations := findMetric(rm, "mcp_tool_invocations")
	if invocations == nil {
		t.Fatal("mcp_tool_invocations metric not found")
	}
	sum, ok := invocations.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64] for mcp_tool_invocations, got %T", invocations.Data)
	}
	if len(sum.DataPoints) == 0 {
		t.Fatal("expected at least one data point for mcp_tool_invocations")
	}
	if sum.DataPoints[0].Value != 1 {
		t.Errorf("expected mcp_tool_invocations count=1, got %d", sum.DataPoints[0].Value)
	}

	duration := findMetric(rm, "mcp_tool_duration_seconds")
	if duration == nil {
		t.Fatal("mcp_tool_duration_seconds metric not found")
	}
	hist, ok := duration.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("expected Histogram[float64] for mcp_tool_duration_seconds, got %T", duration.Data)
	}
	if len(hist.DataPoints) == 0 {
		t.Fatal("expected at least one histogram data point")
	}
	if hist.DataPoints[0].Count != 1 {
		t.Errorf("expected duration histogram count=1, got %d", hist.DataPoints[0].Count)
	}

	// No error was passed — mcp_tool_errors should have no data points.
	errs := findMetric(rm, "mcp_tool_errors")
	if errs != nil {
		errSum, ok := errs.Data.(metricdata.Sum[int64])
		if ok && len(errSum.DataPoints) > 0 {
			t.Errorf("expected no mcp_tool_errors data points for successful invocation, got %d", len(errSum.DataPoints))
		}
	}
}

// TestRecordToolInvocation_WithError verifies that passing an error increments
// mcp_tool_errors with the correct error_type attribute.
func TestRecordToolInvocation_WithError(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")

	metrics, err := createMetrics(meter)
	if err != nil {
		t.Fatalf("createMetrics: %v", err)
	}
	p := &Provider{metrics: metrics}

	toolErr := errors.New("connection refused")
	p.RecordToolInvocation(ctx, "test-tool", "test-category", 50*time.Millisecond, toolErr)

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
	if len(errSum.DataPoints) == 0 {
		t.Fatal("expected at least one data point for mcp_tool_errors")
	}
	if errSum.DataPoints[0].Value != 1 {
		t.Errorf("expected mcp_tool_errors count=1, got %d", errSum.DataPoints[0].Value)
	}

	// Verify error_type attribute is "connection".
	found := false
	for _, attr := range errSum.DataPoints[0].Attributes.ToSlice() {
		if string(attr.Key) == "error_type" {
			if attr.Value.AsString() != "connection" {
				t.Errorf("expected error_type=connection, got %s", attr.Value.AsString())
			}
			found = true
		}
	}
	if !found {
		t.Error("error_type attribute not found on mcp_tool_errors data point")
	}
}

// TestStartEndToolExecution verifies that StartToolExecution increments and
// EndToolExecution decrements the mcp_tools_active gauge.
func TestStartEndToolExecution(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")

	metrics, err := createMetrics(meter)
	if err != nil {
		t.Fatalf("createMetrics: %v", err)
	}
	p := &Provider{metrics: metrics}

	p.StartToolExecution(ctx, "test-tool", "cat")

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect after Start: %v", err)
	}
	active := findMetric(rm, "mcp_tools_active")
	if active == nil {
		t.Fatal("mcp_tools_active metric not found after Start")
	}
	gauge, ok := active.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64] for mcp_tools_active, got %T", active.Data)
	}
	if len(gauge.DataPoints) == 0 || gauge.DataPoints[0].Value != 1 {
		val := int64(0)
		if len(gauge.DataPoints) > 0 {
			val = gauge.DataPoints[0].Value
		}
		t.Errorf("expected mcp_tools_active=1 after Start, got %d", val)
	}

	p.EndToolExecution(ctx, "test-tool", "cat")

	var rm2 metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm2); err != nil {
		t.Fatalf("Collect after End: %v", err)
	}
	active2 := findMetric(rm2, "mcp_tools_active")
	if active2 == nil {
		t.Fatal("mcp_tools_active metric not found after End")
	}
	gauge2, ok := active2.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64] for mcp_tools_active after End, got %T", active2.Data)
	}
	if len(gauge2.DataPoints) == 0 || gauge2.DataPoints[0].Value != 0 {
		val := int64(0)
		if len(gauge2.DataPoints) > 0 {
			val = gauge2.DataPoints[0].Value
		}
		t.Errorf("expected mcp_tools_active=0 after End, got %d", val)
	}
}

// TestStartSpan_NilTracer verifies StartSpan on a Provider with nil tracer
// returns the original context and a nil span.
func TestStartSpan_NilTracer(t *testing.T) {
	p := &Provider{tracer: nil}
	ctx := context.Background()
	retCtx, span := p.StartSpan(ctx, "my-tool")
	if retCtx != ctx {
		t.Error("expected original context to be returned when tracer is nil")
	}
	if span != nil {
		t.Error("expected nil span when tracer is nil")
	}
}

// TestStartSpan_WithTracer verifies StartSpan creates a span with the correct
// name and tool.name attribute when a real tracer is provided.
func TestStartSpan_WithTracer(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	p := &Provider{tracer: tp.Tracer("test")}

	ctx, span := p.StartSpan(context.Background(), "my-tool")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	span.End()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}
	if spans[0].Name != "my-tool" {
		t.Errorf("expected span name 'my-tool', got %q", spans[0].Name)
	}

	foundAttr := false
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "tool.name" {
			if attr.Value.AsString() != "my-tool" {
				t.Errorf("expected tool.name=my-tool, got %s", attr.Value.AsString())
			}
			foundAttr = true
		}
	}
	if !foundAttr {
		t.Error("tool.name attribute not found on span")
	}
}

// TestCategorizeError verifies the unexported categorizeError function
// correctly maps error messages to category strings.
func TestCategorizeError(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{nil, "none"},
		{errors.New("connection refused"), "connection"},
		{errors.New("context canceled"), "canceled"},
		{errors.New("timeout exceeded"), "timeout"},
		{errors.New("panic in handler"), "panic"},
		{errors.New("something else entirely"), "other"},
	}

	for _, tc := range tests {
		got := categorizeError(tc.err)
		if got != tc.expected {
			errStr := "<nil>"
			if tc.err != nil {
				errStr = tc.err.Error()
			}
			t.Errorf("categorizeError(%q): expected %q, got %q", errStr, tc.expected, got)
		}
	}
}

// findMetric searches a ResourceMetrics for a metric by name, returning nil
// if not found.
func findMetric(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for i := range rm.ScopeMetrics {
		for j := range rm.ScopeMetrics[i].Metrics {
			if rm.ScopeMetrics[i].Metrics[j].Name == name {
				m := rm.ScopeMetrics[i].Metrics[j]
				return &m
			}
		}
	}
	return nil
}

// containsStr is a small helper to check substring presence.
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
