package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/hairglasses-studio/mcpkit/protocol"
)

func TestCancellationMiddleware_NormalExecution(t *testing.T) {
	t.Parallel()

	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("ok"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	wrapped := CancellationMiddleware()("test_tool", td, inner)

	result, err := wrapped(context.Background(), CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if IsResultError(result) {
		t.Error("expected non-error result")
	}
}

func TestCancellationMiddleware_HandlerError(t *testing.T) {
	t.Parallel()

	// Non-cancellation errors should pass through unchanged.
	handlerErr := errors.New("database connection failed")
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		return nil, handlerErr
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	wrapped := CancellationMiddleware()("test_tool", td, inner)

	result, err := wrapped(context.Background(), CallToolRequest{})
	if !errors.Is(err, handlerErr) {
		t.Errorf("expected original error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result for non-cancellation error")
	}
}

func TestCancellationMiddleware_ContextCanceled(t *testing.T) {
	t.Parallel()

	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		return nil, context.Canceled
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	wrapped := CancellationMiddleware()("test_tool", td, inner)

	result, err := wrapped(context.Background(), CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}

	// The error should be wrapped as a CancellationError.
	var ce *protocol.CancellationError
	if !errors.As(err, &ce) {
		t.Errorf("expected CancellationError, got %T: %v", err, err)
	}

	// The result should be an error result.
	if !IsResultError(result) {
		t.Error("expected error result")
	}
}

func TestCancellationMiddleware_CancellationError(t *testing.T) {
	t.Parallel()

	// If the handler returns a CancellationError directly, it should be passed through.
	ce := &protocol.CancellationError{Reason: "user requested"}
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		return nil, ce
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	wrapped := CancellationMiddleware()("test_tool", td, inner)

	result, err := wrapped(context.Background(), CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsResultError(result) {
		t.Error("expected error result")
	}
}

func TestWithCancellationReason_RoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	if got := GetCancellationReason(ctx); got != "" {
		t.Errorf("expected empty reason from bare context, got %q", got)
	}

	ctx = WithCancellationReason(ctx, "client disconnected")
	if got := GetCancellationReason(ctx); got != "client disconnected" {
		t.Errorf("expected 'client disconnected', got %q", got)
	}
}

func TestCancellationMiddleware_PreserveResultOnCancel(t *testing.T) {
	t.Parallel()

	// If the handler returns both a result and a cancellation error,
	// the middleware should replace the result with an error result.
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("partial data"), context.Canceled
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	wrapped := CancellationMiddleware()("test_tool", td, inner)

	result, err := wrapped(context.Background(), CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsResultError(result) {
		t.Error("expected error result when context is cancelled")
	}
}
