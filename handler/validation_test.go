//go:build !official_sdk

package handler

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func makeTestSchema(props map[string]any, required []string) *registry.ToolOutputSchema {
	return &registry.ToolOutputSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

func TestOutputValidation_NoSchema_PassThrough(t *testing.T) {
	t.Parallel()
	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{} // No OutputSchema

	called := false
	handler := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
	if registry.IsResultError(result) {
		t.Error("expected non-error result")
	}
}

func TestOutputValidation_ValidResult(t *testing.T) {
	t.Parallel()
	schema := makeTestSchema(map[string]any{
		"name":  map[string]any{"type": "string"},
		"count": map[string]any{"type": "integer"},
	}, []string{"name"})

	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{OutputSchema: schema}

	handler := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeStructuredResult(
			registry.MakeTextContent(`{"name":"test","count":42}`),
			map[string]any{"name": "test", "count": float64(42)},
		), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected valid result to pass through")
	}
}

func TestOutputValidation_MissingRequired(t *testing.T) {
	t.Parallel()
	schema := makeTestSchema(map[string]any{
		"name": map[string]any{"type": "string"},
	}, []string{"name"})

	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{OutputSchema: schema}

	handler := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeStructuredResult(
			registry.MakeTextContent(`{}`),
			map[string]any{},
		), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for missing required field")
	}
}

func TestOutputValidation_WrongType(t *testing.T) {
	t.Parallel()
	schema := makeTestSchema(map[string]any{
		"count": map[string]any{"type": "integer"},
	}, nil)

	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{OutputSchema: schema}

	handler := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeStructuredResult(
			registry.MakeTextContent(`{"count":"not-a-number"}`),
			map[string]any{"count": "not-a-number"},
		), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for wrong type")
	}
}

func TestOutputValidation_ErrorResult_SkipsValidation(t *testing.T) {
	t.Parallel()
	schema := makeTestSchema(map[string]any{
		"name": map[string]any{"type": "string"},
	}, []string{"name"})

	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{OutputSchema: schema}

	handler := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeErrorResult("something went wrong"), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Error results should pass through without validation
	if !registry.IsResultError(result) {
		t.Error("expected error result to pass through")
	}
}

func TestOutputValidation_NilStructuredContent_Passes(t *testing.T) {
	t.Parallel()
	schema := makeTestSchema(map[string]any{
		"name": map[string]any{"type": "string"},
	}, []string{"name"})

	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{OutputSchema: schema}

	handler := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		// Return text-only result with no structured content
		return registry.MakeTextResult("ok"), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected nil structured content to pass through")
	}
}

func TestOutputValidation_AllTypes(t *testing.T) {
	t.Parallel()
	schema := makeTestSchema(map[string]any{
		"s": map[string]any{"type": "string"},
		"n": map[string]any{"type": "number"},
		"i": map[string]any{"type": "integer"},
		"b": map[string]any{"type": "boolean"},
		"a": map[string]any{"type": "array"},
		"o": map[string]any{"type": "object"},
	}, nil)

	mw := OutputValidationMiddleware()
	td := registry.ToolDefinition{OutputSchema: schema}

	handler := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeStructuredResult(
			registry.MakeTextContent("ok"),
			map[string]any{
				"s": "hello",
				"n": float64(3.14),
				"i": float64(42),
				"b": true,
				"a": []any{"x"},
				"o": map[string]any{"k": "v"},
			},
		), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected all types to validate correctly")
	}
}
