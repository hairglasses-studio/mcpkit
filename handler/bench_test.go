//go:build !official_sdk

package handler

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// BenchmarkTypedHandler_Invocation measures the full TypedHandler path:
// JSON marshal arguments -> unmarshal into typed struct -> call handler ->
// marshal result into structured output.
func BenchmarkTypedHandler_Invocation(b *testing.B) {
	td := TypedHandler[testInput, testOutput](
		"bench_search",
		"Benchmark search tool",
		func(_ context.Context, input testInput) (testOutput, error) {
			return testOutput{
				Results: []string{input.Query},
				Total:   1,
			}, nil
		},
	)

	ctx := context.Background()
	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"query": "benchmark test",
		"limit": float64(10),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		td.Handler(ctx, req)
	}
}

// BenchmarkGetStringParam measures parameter extraction from a tool request,
// which is the most commonly called param helper in all MCP handlers.
func BenchmarkGetStringParam(b *testing.B) {
	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name":     "test-value",
		"category": "benchmarks",
		"format":   "detailed",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetStringParam(req, "name")
	}
}

// BenchmarkGetIntParam measures integer parameter extraction with type
// assertion from float64 (JSON number default).
func BenchmarkGetIntParam(b *testing.B) {
	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"limit":  float64(50),
		"offset": float64(10),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetIntParam(req, "limit", 10)
	}
}

// BenchmarkTextResult measures the result builder hot path — creating a
// CallToolResult with text content, which every non-error handler return uses.
func BenchmarkTextResult(b *testing.B) {
	text := `{"status": "ok", "count": 42, "items": ["a", "b", "c"]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TextResult(text)
	}
}

// BenchmarkJSONResult measures JSON marshaling into a tool result, the path
// used by most handlers that return structured data.
func BenchmarkJSONResult(b *testing.B) {
	data := map[string]any{
		"status": "ok",
		"count":  42,
		"items":  []string{"alpha", "bravo", "charlie"},
		"nested": map[string]any{
			"key": "value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		JSONResult(data)
	}
}

// BenchmarkCodedErrorResult measures error result construction with code prefix.
func BenchmarkCodedErrorResult(b *testing.B) {
	err := errForBench("missing required parameter: name")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CodedErrorResult(ErrInvalidParam, err)
	}
}

type errForBench string

func (e errForBench) Error() string { return string(e) }
