package observability

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/registry"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestAttrKeys_Values verifies that the GenAI semantic convention attribute
// key constants have the expected string values.
func TestAttrKeys_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key     string
		wantStr string
	}{
		{string(AttrGenAISystem), "gen_ai.system"},
		{string(AttrGenAIOperationName), "gen_ai.operation.name"},
		{string(AttrGenAIRequestModel), "gen_ai.request.model"},
		{string(AttrGenAIUsageInput), "gen_ai.usage.input_tokens"},
		{string(AttrGenAIUsageOutput), "gen_ai.usage.output_tokens"},
	}

	for _, tc := range cases {
		if tc.key != tc.wantStr {
			t.Errorf("attribute key: expected %q, got %q", tc.wantStr, tc.key)
		}
	}
}

// TestStartSpan_GenAIAttributes verifies that StartSpan sets gen_ai.system and
// gen_ai.operation.name on the created span.
func TestStartSpan_GenAIAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	p := &Provider{tracer: tp.Tracer("test")}

	ctx, span := p.StartSpan(context.Background(), "my-genai-tool")
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

	attrMap := make(map[string]string, len(spans[0].Attributes))
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsString()
	}

	if got := attrMap["gen_ai.system"]; got != "mcp" {
		t.Errorf("gen_ai.system: expected %q, got %q", "mcp", got)
	}
	if got := attrMap["gen_ai.operation.name"]; got != "tool_call" {
		t.Errorf("gen_ai.operation.name: expected %q, got %q", "tool_call", got)
	}
	if got := attrMap["tool.name"]; got != "my-genai-tool" {
		t.Errorf("tool.name: expected %q, got %q", "my-genai-tool", got)
	}
}

// buildTracedProvider creates a Provider backed by both an in-memory tracer
// and in-memory metrics for use in attribute tests.
func buildTracedProvider(t *testing.T) (*Provider, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")
	metrics, err := createMetrics(meter)
	if err != nil {
		t.Fatalf("createMetrics: %v", err)
	}

	return &Provider{tracer: tp.Tracer("test"), metrics: metrics}, exporter
}

// TestMiddleware_GenAITokenUsageFromContext verifies that when finops token
// usage is present in the context, the observability middleware attaches
// gen_ai.usage.input_tokens, gen_ai.usage.output_tokens, and
// gen_ai.request.model to the span.
func TestMiddleware_GenAITokenUsageFromContext(t *testing.T) {
	p, exporter := buildTracedProvider(t)

	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "token-tool"},
		Category: "ai",
	}

	usage := finops.TokenUsage{
		InputTokens:  50,
		OutputTokens: 200,
		Model:        "claude-3-5-sonnet",
	}

	// Inner handler — the token usage is injected into the context by an
	// "outer" wrapper below, simulating an upstream finops middleware.
	inner := mw("token-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	// Inject token usage before the observability middleware runs.
	ctx := finops.WithTokenUsage(context.Background(), usage)
	if _, err := inner(ctx, registry.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}

	attrMap := make(map[string]any, len(spans[0].Attributes))
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if v, ok := attrMap["gen_ai.usage.input_tokens"]; !ok || v != int64(50) {
		t.Errorf("gen_ai.usage.input_tokens: expected 50, got %v (present=%v)", v, ok)
	}
	if v, ok := attrMap["gen_ai.usage.output_tokens"]; !ok || v != int64(200) {
		t.Errorf("gen_ai.usage.output_tokens: expected 200, got %v (present=%v)", v, ok)
	}
	if v, ok := attrMap["gen_ai.request.model"]; !ok || v != "claude-3-5-sonnet" {
		t.Errorf("gen_ai.request.model: expected claude-3-5-sonnet, got %v (present=%v)", v, ok)
	}
}

// TestMiddleware_GenAITokenUsageFromHolder verifies that token usage written by
// an inner finops middleware via the mutable holder is bridged onto the span
// by the observability middleware, without any static context injection.
func TestMiddleware_GenAITokenUsageFromHolder(t *testing.T) {
	p, exporter := buildTracedProvider(t)

	// Build a finops tracker whose middleware will record usage and populate the holder.
	tracker := finops.NewTracker()
	finosMW := finops.Middleware(tracker)

	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "holder-tool"},
		Category: "ai",
	}

	// Innermost handler — returns a non-trivial text result so tokens are estimated.
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("hello world response"), nil
	}

	// Chain: observability wraps finops wraps inner.
	obsWrapped := p.Middleware()("holder-tool", td,
		finosMW("holder-tool", td, inner),
	)

	if _, err := obsWrapped(context.Background(), registry.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}

	attrMap := make(map[string]any, len(spans[0].Attributes))
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if _, ok := attrMap["gen_ai.usage.input_tokens"]; !ok {
		t.Error("gen_ai.usage.input_tokens not set on span via holder path")
	}
	if _, ok := attrMap["gen_ai.usage.output_tokens"]; !ok {
		t.Error("gen_ai.usage.output_tokens not set on span via holder path")
	}
}

// TestMiddleware_GenAINoTokenUsage verifies that when no finops token usage is
// present in the context, the middleware does not set token usage attributes.
func TestMiddleware_GenAINoTokenUsage(t *testing.T) {
	p, exporter := buildTracedProvider(t)

	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "no-token-tool"},
		Category: "test",
	}

	wrapped := mw("no-token-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	if _, err := wrapped(context.Background(), registry.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}

	for _, a := range spans[0].Attributes {
		key := string(a.Key)
		if key == "gen_ai.usage.input_tokens" || key == "gen_ai.usage.output_tokens" || key == "gen_ai.request.model" {
			t.Errorf("unexpected attribute %q set when no token usage in context", key)
		}
	}
}

// TestMiddleware_GenAINoModel verifies that gen_ai.request.model is omitted
// when TokenUsage.Model is empty, but token counts are still recorded.
func TestMiddleware_GenAINoModel(t *testing.T) {
	p, exporter := buildTracedProvider(t)

	mw := p.Middleware()
	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "no-model-tool"},
		Category: "test",
	}

	usage := finops.TokenUsage{InputTokens: 10, OutputTokens: 20} // no Model

	inner := mw("no-model-tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	ctx := finops.WithTokenUsage(context.Background(), usage)
	if _, err := inner(ctx, registry.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}

	attrMap := make(map[string]any, len(spans[0].Attributes))
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if v, ok := attrMap["gen_ai.usage.input_tokens"]; !ok || v != int64(10) {
		t.Errorf("gen_ai.usage.input_tokens: expected 10, got %v (present=%v)", v, ok)
	}
	if v, ok := attrMap["gen_ai.usage.output_tokens"]; !ok || v != int64(20) {
		t.Errorf("gen_ai.usage.output_tokens: expected 20, got %v (present=%v)", v, ok)
	}
	if _, modelPresent := attrMap["gen_ai.request.model"]; modelPresent {
		t.Error("gen_ai.request.model should not be set when Model is empty")
	}
}
