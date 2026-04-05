//go:build !official_sdk

package dispatcher

import (
	"context"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestMiddleware_ReturnsHandler(t *testing.T) {
	t.Parallel()
	d := New(Config{Workers: 2})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}

	handler := mw("tool", td, inner)
	if handler == nil {
		t.Fatal("expected non-nil handler from Middleware")
	}
}

func TestMiddleware_ExecutesInner(t *testing.T) {
	t.Parallel()
	d := New(Config{Workers: 2})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{}
	called := false
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("executed"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful non-error result")
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestMiddleware_GroupFromRuntimeGroup(t *testing.T) {
	t.Parallel()
	d := New(Config{
		Workers:     4,
		QueueSize:   50,
		GroupLimits: map[string]int{"mygroup": 10},
	})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{RuntimeGroup: "mygroup"}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("grouped"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected successful result, got IsError=%v", result.IsError)
	}
}

func TestMiddleware_GroupFromCircuitBreakerGroup(t *testing.T) {
	t.Parallel()
	d := New(Config{Workers: 2, QueueSize: 50})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{CircuitBreakerGroup: "cb-group"}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("cb"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result")
	}
}

func TestMiddleware_GroupFromGroupFunc(t *testing.T) {
	t.Parallel()
	groupFuncCalled := false
	d := New(Config{
		Workers:   2,
		QueueSize: 50,
		GroupFunc: func(name string, td registry.ToolDefinition) string {
			groupFuncCalled = true
			return "dynamic-group"
		},
	})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	// No RuntimeGroup or CircuitBreakerGroup set — GroupFunc should be used.
	td := registry.ToolDefinition{}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result")
	}
	if !groupFuncCalled {
		t.Error("expected GroupFunc to be called when no RuntimeGroup/CircuitBreakerGroup set")
	}
}

func TestMiddleware_PriorityFromPriorityFunc(t *testing.T) {
	t.Parallel()
	// PriorityFunc overrides DefaultPriority.
	d := New(Config{
		Workers:   1,
		QueueSize: 100,
		PriorityFunc: func(name string, td registry.ToolDefinition) Priority {
			if name == "critical" {
				return PriorityCritical
			}
			return PriorityLow
		},
	})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("prioritized"), nil
	}

	wrapped := mw("critical", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result")
	}
}

func TestMiddleware_ContextPropagated(t *testing.T) {
	t.Parallel()
	type ctxKey struct{}
	d := New(Config{Workers: 2})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{}

	var receivedVal any
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		receivedVal = ctx.Value(ctxKey{})
		return registry.MakeTextResult("ctx"), nil
	}

	wrapped := mw("tool", td, inner)
	ctx := context.WithValue(context.Background(), ctxKey{}, "injected")
	_, err := wrapped(ctx, registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedVal != "injected" {
		t.Errorf("expected context value to be propagated, got %v", receivedVal)
	}
}

func TestMiddleware_ShutdownReturnsError(t *testing.T) {
	t.Parallel()
	d := New(Config{Workers: 1})
	d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("never"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result when dispatcher is shut down")
	}
}

func TestMiddleware_RuntimeGroupTakesPrecedence(t *testing.T) {
	t.Parallel()
	// When both RuntimeGroup and CircuitBreakerGroup are set, RuntimeGroup wins.
	d := New(Config{
		Workers:     4,
		QueueSize:   50,
		GroupLimits: map[string]int{"runtime": 10, "cb": 10},
	})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	mw := Middleware(d)
	td := registry.ToolDefinition{
		RuntimeGroup:        "runtime",
		CircuitBreakerGroup: "cb",
	}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result")
	}
}

func TestMiddleware_ConcurrentCalls(t *testing.T) {
	t.Parallel()
	d := New(Config{Workers: 8, QueueSize: 200})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.Shutdown(ctx) //nolint:errcheck
	}()

	mw := Middleware(d)
	td := registry.ToolDefinition{}
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("concurrent"), nil
	}
	wrapped := mw("tool", td, inner)

	const n = 20
	errs := make(chan error, n)
	for range n {
		go func() {
			result, err := wrapped(context.Background(), registry.CallToolRequest{})
			if err != nil {
				errs <- err
				return
			}
			if result == nil || result.IsError {
				errs <- nil // count as failure via IsError
				return
			}
			errs <- nil
		}()
	}

	for i := range n {
		if err := <-errs; err != nil {
			t.Errorf("concurrent call %d: unexpected error: %v", i, err)
		}
	}
}
