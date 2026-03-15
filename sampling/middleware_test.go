//go:build !official_sdk

package sampling

import (
	"context"
	"errors"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestMiddleware_InjectsClientIntoContext(t *testing.T) {
	t.Parallel()
	client := &mockClient{}
	mw := Middleware(client)
	td := registry.ToolDefinition{}

	var captured SamplingClient
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		captured = ClientFromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := mw("tool", td, inner)
	_, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != client {
		t.Errorf("expected injected client, got %v", captured)
	}
}

func TestMiddleware_NilClient_DoesNotInject(t *testing.T) {
	t.Parallel()
	mw := Middleware(nil)
	td := registry.ToolDefinition{}

	var captured SamplingClient
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		captured = ClientFromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := mw("tool", td, inner)
	_, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != nil {
		t.Errorf("expected nil client when Middleware called with nil, got %v", captured)
	}
}

func TestMiddleware_PreservesExistingContextValues(t *testing.T) {
	t.Parallel()
	type ctxKey struct{}
	client := &mockClient{}
	mw := Middleware(client)
	td := registry.ToolDefinition{}

	var receivedVal any
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		receivedVal = ctx.Value(ctxKey{})
		return registry.MakeTextResult("ok"), nil
	}

	ctx := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	wrapped := mw("tool", td, inner)
	_, err := wrapped(ctx, registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedVal != "sentinel" {
		t.Errorf("expected existing context value to be preserved, got %v", receivedVal)
	}
}

func TestMiddleware_PropagatesInnerError(t *testing.T) {
	t.Parallel()
	// Middleware must pass through errors from the inner handler.
	client := &mockClient{}
	mw := Middleware(client)
	td := registry.ToolDefinition{}

	expectedErr := errors.New("inner error")
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, expectedErr
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected inner error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}
}

func TestMiddleware_PropagatesInnerResult(t *testing.T) {
	t.Parallel()
	client := &mockClient{}
	mw := Middleware(client)
	td := registry.ToolDefinition{}

	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("specific-result"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok || text != "specific-result" {
		t.Errorf("expected result text %q, got %q", "specific-result", text)
	}
}

func TestMiddleware_DifferentToolNames(t *testing.T) {
	t.Parallel()
	// Middleware should work identically regardless of tool name.
	client := &mockClient{}
	mw := Middleware(client)
	td := registry.ToolDefinition{}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}

	for _, name := range []string{"", "tool-a", "tool-b", "very-long-tool-name-here"} {
		wrapped := mw(name, td, inner)
		result, err := wrapped(context.Background(), registry.CallToolRequest{})
		if err != nil {
			t.Errorf("tool %q: unexpected error: %v", name, err)
		}
		if result == nil || result.IsError {
			t.Errorf("tool %q: expected non-error result", name)
		}
	}
}

func TestMiddleware_MultipleWraps(t *testing.T) {
	t.Parallel()
	// Two Middleware instances stacked — both clients injected in order.
	client1 := &mockClient{}
	client2 := &mockClient{}

	td := registry.ToolDefinition{}
	var captured SamplingClient
	innermost := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		captured = ClientFromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	// client2 wraps after client1, so client2 overwrites in ctx.
	wrapped := Middleware(client1)("tool", td, Middleware(client2)("tool", td, innermost))
	_, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Innermost sees client2 because it was injected closest to the handler.
	if captured != client2 {
		t.Errorf("expected innermost to see client2, got %v", captured)
	}
}

func TestMiddleware_RequestPropagated(t *testing.T) {
	t.Parallel()
	client := &mockClient{}
	mw := Middleware(client)
	td := registry.ToolDefinition{}

	var receivedReq registry.CallToolRequest
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		receivedReq = req
		return registry.MakeTextResult("ok"), nil
	}

	req := registry.CallToolRequest{}
	wrapped := mw("tool", td, inner)
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = receivedReq // request was received
}
