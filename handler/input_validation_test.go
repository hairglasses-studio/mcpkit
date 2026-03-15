//go:build !official_sdk

package handler

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func makeInputSchema(props map[string]any, required []string) registry.ToolInputSchema {
	return registry.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

func TestInputValidation_NoSchema_PassThrough(t *testing.T) {
	t.Parallel()
	mw := InputValidationMiddleware()
	td := registry.ToolDefinition{} // Empty InputSchema

	called := false
	h := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	result, err := h(context.Background(), registry.CallToolRequest{})
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

func TestInputValidation_ValidParams(t *testing.T) {
	t.Parallel()
	mw := InputValidationMiddleware()
	td := registry.ToolDefinition{
		Tool: registry.Tool{
			InputSchema: makeInputSchema(map[string]any{
				"name":  map[string]any{"type": "string"},
				"count": map[string]any{"type": "integer"},
			}, []string{"name"}),
		},
	}

	called := false
	h := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name":  "alice",
		"count": float64(5),
	}

	result, err := h(context.Background(), req)
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

func TestInputValidation_MissingRequired(t *testing.T) {
	t.Parallel()
	mw := InputValidationMiddleware()
	td := registry.ToolDefinition{
		Tool: registry.Tool{
			InputSchema: makeInputSchema(map[string]any{
				"name": map[string]any{"type": "string"},
			}, []string{"name"}),
		},
	}

	called := false
	h := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"other": "value",
	}

	result, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected handler NOT to be called")
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for missing required field")
	}
}

func TestInputValidation_MissingRequired_NilArgs(t *testing.T) {
	t.Parallel()
	mw := InputValidationMiddleware()
	td := registry.ToolDefinition{
		Tool: registry.Tool{
			InputSchema: makeInputSchema(map[string]any{
				"name": map[string]any{"type": "string"},
			}, []string{"name"}),
		},
	}

	called := false
	h := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	// No arguments set — nil args
	result, err := h(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected handler NOT to be called")
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for nil args with required field")
	}
}

func TestInputValidation_WrongType(t *testing.T) {
	t.Parallel()
	mw := InputValidationMiddleware()
	td := registry.ToolDefinition{
		Tool: registry.Tool{
			InputSchema: makeInputSchema(map[string]any{
				"count": map[string]any{"type": "integer"},
			}, nil),
		},
	}

	called := false
	h := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"count": "not-a-number",
	}

	result, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected handler NOT to be called")
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for wrong type")
	}
}

func TestInputValidation_AllTypes(t *testing.T) {
	t.Parallel()
	mw := InputValidationMiddleware()
	td := registry.ToolDefinition{
		Tool: registry.Tool{
			InputSchema: makeInputSchema(map[string]any{
				"s": map[string]any{"type": "string"},
				"n": map[string]any{"type": "number"},
				"i": map[string]any{"type": "integer"},
				"b": map[string]any{"type": "boolean"},
				"a": map[string]any{"type": "array"},
				"o": map[string]any{"type": "object"},
			}, nil),
		},
	}

	called := false
	h := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"s": "hello",
		"n": float64(3.14),
		"i": float64(42),
		"b": true,
		"a": []any{"x"},
		"o": map[string]any{"k": "v"},
	}

	result, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
	if registry.IsResultError(result) {
		t.Error("expected all types to validate correctly")
	}
}

func TestInputValidation_OptionalFieldMissing(t *testing.T) {
	t.Parallel()
	mw := InputValidationMiddleware()
	td := registry.ToolDefinition{
		Tool: registry.Tool{
			InputSchema: makeInputSchema(map[string]any{
				"name":    map[string]any{"type": "string"},
				"optional": map[string]any{"type": "integer"},
			}, []string{"name"}),
		},
	}

	called := false
	h := mw("tool", td, func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	})

	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name": "bob",
		// "optional" intentionally absent
	}

	result, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called when optional field is absent")
	}
	if registry.IsResultError(result) {
		t.Error("expected non-error result when optional field is missing")
	}
}
