//go:build !official_sdk

package sampling

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hairglasses-studio/mcpkit/finops"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestAPISamplingClientCreateMessage_RecordsUsageAndSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %q, want /v1/messages", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":       "code-primary",
			"stop_reason": "end_turn",
			"content":     []map[string]any{{"type": "text", "text": "done"}},
			"usage": map[string]any{
				"input_tokens":  21,
				"output_tokens": 7,
			},
		})
	}))
	defer ts.Close()

	client := &APISamplingClient{
		BaseURL:      ts.URL,
		DefaultModel: "code-primary",
		APIKey:       "ollama",
	}
	var holder finops.TokenUsageHolder
	ctx := finops.WithTokenUsageHolder(context.Background(), &holder)

	resp, err := client.CreateMessage(ctx, CompletionRequest([]SamplingMessage{
		TextMessage("user", "hello"),
	}))
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}
	if resp == nil || resp.Model != "code-primary" {
		t.Fatalf("CreateMessage() model = %#v, want code-primary response", resp)
	}

	usage, ok := holder.Load()
	if !ok {
		t.Fatal("expected token usage to be written to holder")
	}
	if usage.InputTokens != 21 || usage.OutputTokens != 7 || usage.Model != "code-primary" {
		t.Fatalf("holder usage = %+v, want input=21 output=7 model=code-primary", usage)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}
	attrMap := make(map[string]any, len(spans[0].Attributes))
	for _, attr := range spans[0].Attributes {
		attrMap[string(attr.Key)] = attr.Value.AsInterface()
	}
	if got := attrMap["gen_ai.system"]; got != "ollama" {
		t.Fatalf("gen_ai.system = %v, want ollama", got)
	}
	if got := attrMap["gen_ai.request.model"]; got != "code-primary" {
		t.Fatalf("gen_ai.request.model = %v, want code-primary", got)
	}
	if got := attrMap["gen_ai.usage.input_tokens"]; got != int64(21) {
		t.Fatalf("gen_ai.usage.input_tokens = %v, want 21", got)
	}
	if got := attrMap["gen_ai.usage.output_tokens"]; got != int64(7) {
		t.Fatalf("gen_ai.usage.output_tokens = %v, want 7", got)
	}
}

func TestNativeOllamaClientGenerate_RecordsUsageAndSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("path = %q, want /api/generate", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":                "code-primary",
			"response":             "ok",
			"done":                 true,
			"prompt_eval_count":    13,
			"eval_count":           5,
			"prompt_eval_duration": 1,
			"eval_duration":        1,
		})
	}))
	defer ts.Close()

	client := &NativeOllamaClient{BaseURL: ts.URL}
	var holder finops.TokenUsageHolder
	ctx := finops.WithTokenUsageHolder(context.Background(), &holder)

	resp, err := client.Generate(ctx, NativeOllamaGenerateRequest{
		Model:  "code-primary",
		Prompt: "hello",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp == nil || resp.Model != "code-primary" {
		t.Fatalf("Generate() = %#v, want model code-primary", resp)
	}

	usage, ok := holder.Load()
	if !ok {
		t.Fatal("expected token usage to be written to holder")
	}
	if usage.InputTokens != 13 || usage.OutputTokens != 5 || usage.Model != "code-primary" {
		t.Fatalf("holder usage = %+v, want input=13 output=5 model=code-primary", usage)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one exported span")
	}
	attrMap := make(map[string]any, len(spans[0].Attributes))
	for _, attr := range spans[0].Attributes {
		attrMap[string(attr.Key)] = attr.Value.AsInterface()
	}
	if got := attrMap["gen_ai.system"]; got != "ollama" {
		t.Fatalf("gen_ai.system = %v, want ollama", got)
	}
	if got := attrMap["gen_ai.operation.name"]; got != "generate" {
		t.Fatalf("gen_ai.operation.name = %v, want generate", got)
	}
	if got := attrMap["gen_ai.usage.input_tokens"]; got != int64(13) {
		t.Fatalf("gen_ai.usage.input_tokens = %v, want 13", got)
	}
	if got := attrMap["gen_ai.usage.output_tokens"]; got != int64(5) {
		t.Fatalf("gen_ai.usage.output_tokens = %v, want 5", got)
	}
}
