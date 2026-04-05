//go:build !official_sdk

package resilience

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// successHandler returns a successful text result.
func successHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("ok"), nil
}

// errorHandler returns an error result (IsError=true).
func errorHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeErrorResult("tool failed"), nil
}

// goErrorHandler returns a Go-level error.
func goErrorHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return nil, errors.New("go error")
}

// --- RateLimitMiddleware tests ---

func TestRateLimitMiddleware_NoGroupPassesThrough(t *testing.T) {
	t.Parallel()
	reg := NewRateLimitRegistry()
	mw := RateLimitMiddleware(reg)

	// Tool with no CircuitBreakerGroup — middleware must be a no-op (returns next directly).
	td := registry.ToolDefinition{}
	called := false
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("passthrough"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result")
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestRateLimitMiddleware_PassesThroughWhenTokenAvailable(t *testing.T) {
	t.Parallel()
	reg := NewRateLimitRegistry(RateLimitConfig{Rate: 100, Burst: 10})
	mw := RateLimitMiddleware(reg)

	td := registry.ToolDefinition{CircuitBreakerGroup: "svc"}
	called := false
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("limited-ok"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected successful result, got IsError=%v", result.IsError)
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestRateLimitMiddleware_ReturnsErrorWhenContextCancelledWhileWaiting(t *testing.T) {
	t.Parallel()

	// Burst of 1, very slow rate — exhaust the burst then cancel context to trigger error.
	reg := NewRateLimitRegistry(RateLimitConfig{Rate: 0.01, Burst: 1})
	// Pre-exhaust the limiter for "svc" so the next call must wait.
	lim := reg.Get("svc")
	lim.Allow() // consume the single burst token

	mw := RateLimitMiddleware(reg)
	td := registry.ToolDefinition{CircuitBreakerGroup: "svc"}

	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("should not reach"), nil
	}

	wrapped := mw("tool", td, inner)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	result, err := wrapped(ctx, registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error (middleware should return error result, not Go error): %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result when rate limit context is cancelled")
	}
	if !strings.Contains(result.Content[0].(registry.TextContent).Text, "rate limited") {
		t.Errorf("expected 'rate limited' in result text, got: %v", result.Content)
	}
}

// --- CircuitBreakerMiddleware tests ---

func TestCircuitBreakerMiddleware_NoGroupPassesThrough(t *testing.T) {
	t.Parallel()
	reg := NewCircuitBreakerRegistry(nil)
	mw := CircuitBreakerMiddleware(reg)

	// Tool with no CircuitBreakerGroup — middleware must be a no-op.
	td := registry.ToolDefinition{}
	called := false
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("passthrough"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result")
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestCircuitBreakerMiddleware_PassesThroughOnSuccess(t *testing.T) {
	t.Parallel()
	reg := NewCircuitBreakerRegistry(nil)
	mw := CircuitBreakerMiddleware(reg)

	td := registry.ToolDefinition{CircuitBreakerGroup: "svc"}
	called := false
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("success"), nil
	}

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected successful result, IsError=%v", result.IsError)
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestCircuitBreakerMiddleware_OpensCircuitOnRepeatedGoErrors(t *testing.T) {
	t.Parallel()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          time.Hour,
		HalfOpenMaxCalls: 1,
	}
	reg := NewCircuitBreakerRegistry(nil, cfg)
	mw := CircuitBreakerMiddleware(reg)

	td := registry.ToolDefinition{CircuitBreakerGroup: "failing-svc"}
	inner := goErrorHandler

	wrapped := mw("tool", td, inner)

	// Drive 3 failures to open the circuit.
	for i := range 3 {
		result, err := wrapped(context.Background(), registry.CallToolRequest{})
		// The middleware returns the Go error from the handler as-is when not ErrCircuitOpen.
		if err == nil && result != nil && !result.IsError {
			t.Errorf("call %d: expected error result or Go error", i+1)
		}
	}

	// Circuit should now be open; next call should return CIRCUIT_OPEN result.
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error after circuit opened: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result when circuit is open")
	}
	text := result.Content[0].(registry.TextContent).Text
	if !strings.Contains(text, "CIRCUIT_OPEN") {
		t.Errorf("expected CIRCUIT_OPEN in result text, got: %q", text)
	}
}

func TestCircuitBreakerMiddleware_OpensCircuitOnRepeatedErrorResults(t *testing.T) {
	t.Parallel()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          time.Hour,
		HalfOpenMaxCalls: 1,
	}
	reg := NewCircuitBreakerRegistry(nil, cfg)
	mw := CircuitBreakerMiddleware(reg)

	td := registry.ToolDefinition{CircuitBreakerGroup: "err-result-svc"}
	inner := errorHandler // returns IsError=true result

	wrapped := mw("tool", td, inner)

	// Drive 2 error-result failures to open the circuit.
	for range 2 {
		wrapped(context.Background(), registry.CallToolRequest{}) //nolint:errcheck
	}

	// Circuit should now be open.
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error when circuit is open: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result when circuit is open")
	}
	text := result.Content[0].(registry.TextContent).Text
	if !strings.Contains(text, "CIRCUIT_OPEN") {
		t.Errorf("expected CIRCUIT_OPEN in result text, got: %q", text)
	}
}

func TestCircuitBreakerMiddleware_CircuitOpenMessageContainsGroupName(t *testing.T) {
	t.Parallel()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
		HalfOpenMaxCalls: 1,
	}
	reg := NewCircuitBreakerRegistry(nil, cfg)
	mw := CircuitBreakerMiddleware(reg)

	const group = "my-upstream"
	td := registry.ToolDefinition{CircuitBreakerGroup: group}
	inner := goErrorHandler

	wrapped := mw("tool", td, inner)
	// One failure to open the circuit.
	wrapped(context.Background(), registry.CallToolRequest{}) //nolint:errcheck

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}
	text := result.Content[0].(registry.TextContent).Text
	if !strings.Contains(text, group) {
		t.Errorf("expected group name %q in CIRCUIT_OPEN message, got: %q", group, text)
	}
	if !strings.Contains(text, "CIRCUIT_OPEN") {
		t.Errorf("expected CIRCUIT_OPEN prefix, got: %q", text)
	}
}

func TestCircuitBreakerMiddleware_PropagatesGoErrorWhenCircuitClosed(t *testing.T) {
	t.Parallel()
	// With a high threshold the circuit stays closed; Go errors from the handler
	// must be returned to the caller unchanged.
	cfg := CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
		HalfOpenMaxCalls: 1,
	}
	reg := NewCircuitBreakerRegistry(nil, cfg)
	mw := CircuitBreakerMiddleware(reg)

	td := registry.ToolDefinition{CircuitBreakerGroup: "svc"}
	inner := goErrorHandler

	wrapped := mw("tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err == nil {
		t.Error("expected Go error to be propagated when circuit is closed")
	}
	if result != nil {
		t.Errorf("expected nil result when Go error propagated, got: %v", result)
	}
}
