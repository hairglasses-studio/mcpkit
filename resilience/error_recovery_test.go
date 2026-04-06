//go:build !official_sdk

package resilience

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- ErrorRecoveryMiddleware tests ---

func TestErrorRecoveryMiddleware_Success(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	mw := ErrorRecoveryMiddleware(cfg)

	td := registry.ToolDefinition{}
	called := false
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := mw("test-tool", td, inner)
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

func TestErrorRecoveryMiddleware_RetrySuccess(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 3
	mw := ErrorRecoveryMiddleware(cfg)

	td := registry.ToolDefinition{}
	var callCount atomic.Int32

	// Fail on the first call, succeed on the second.
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		n := callCount.Add(1)
		if n == 1 {
			return registry.MakeErrorResult("transient failure"), nil
		}
		return registry.MakeTextResult("recovered"), nil
	}

	wrapped := mw("retry-tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result after retry")
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls (initial + 1 retry), got %d", callCount.Load())
	}
}

func TestErrorRecoveryMiddleware_RetrySuccessOnGoError(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 3
	mw := ErrorRecoveryMiddleware(cfg)

	td := registry.ToolDefinition{}
	var callCount atomic.Int32

	// Return a Go-level error on first call, then succeed.
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		n := callCount.Add(1)
		if n == 1 {
			return nil, errors.New("connection refused")
		}
		return registry.MakeTextResult("reconnected"), nil
	}

	wrapped := mw("go-error-tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected successful result after retry on Go error")
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}

func TestErrorRecoveryMiddleware_MaxRetries(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 2

	var escalated atomic.Bool
	var escalation ErrorEscalation
	var escalationMu sync.Mutex

	cfg.EscalateFunc = func(e ErrorEscalation) {
		escalationMu.Lock()
		defer escalationMu.Unlock()
		escalation = e
		escalated.Store(true)
	}

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	var callCount atomic.Int32
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount.Add(1)
		return registry.MakeErrorResult("persistent failure"), nil
	}

	wrapped := mw("failing-tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("middleware must never return Go error, got: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result after exhausting retries")
	}

	// 1 initial call + 2 retries = 3 total.
	if callCount.Load() != 3 {
		t.Errorf("expected 3 calls (1 initial + 2 retries), got %d", callCount.Load())
	}

	// Escalation must have been called.
	if !escalated.Load() {
		t.Fatal("expected EscalateFunc to be called")
	}

	escalationMu.Lock()
	defer escalationMu.Unlock()
	if escalation.ToolName != "failing-tool" {
		t.Errorf("expected tool name 'failing-tool', got %q", escalation.ToolName)
	}
	if escalation.Retries != 2 {
		t.Errorf("expected 2 retries in escalation, got %d", escalation.Retries)
	}
	if escalation.Duration <= 0 {
		t.Error("expected positive duration in escalation")
	}
	if escalation.Error == nil {
		t.Error("expected non-nil error in escalation")
	}
}

func TestErrorRecoveryMiddleware_NonRetryableError(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 5
	cfg.ShouldRetry = func(err error) bool {
		// Only retry network errors.
		return strings.Contains(strings.ToLower(err.Error()), "network")
	}

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	var callCount atomic.Int32
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount.Add(1)
		return registry.MakeErrorResult("permission denied"), nil
	}

	wrapped := mw("perm-tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for non-retryable error")
	}

	// Should be called exactly once — no retries for non-retryable errors.
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call (no retry for non-retryable), got %d", callCount.Load())
	}
}

func TestErrorRecoveryMiddleware_ZeroMaxRetries(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 0

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	var callCount atomic.Int32
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount.Add(1)
		return nil, errors.New("boom")
	}

	wrapped := mw("zero-retry", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("middleware must never return Go error, got: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result with zero retries")
	}
	if callCount.Load() != 1 {
		t.Errorf("expected exactly 1 call with zero retries, got %d", callCount.Load())
	}
}

func TestErrorRecoveryMiddleware_RespectsContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 100 // Would take many retries — but context cancels first.
	cfg.RetryDelay = 50 * time.Millisecond

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	var callCount atomic.Int32
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount.Add(1)
		return registry.MakeErrorResult("keep failing"), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	wrapped := mw("ctx-tool", td, inner)
	result, err := wrapped(ctx, registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("middleware must never return Go error, got: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result after context cancellation")
	}

	// Should have been called at least once but far fewer than 100.
	calls := callCount.Load()
	if calls < 1 || calls > 10 {
		t.Errorf("expected 1-10 calls before context timeout, got %d", calls)
	}
}

func TestErrorRecoveryMiddleware_CustomFormatError(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 0
	cfg.FormatError = func(err error) string {
		return "CUSTOM: " + err.Error()
	}

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, errors.New("original error")
	}

	wrapped := mw("custom-fmt", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}

	text := result.Content[0].(registry.TextContent).Text
	if !strings.Contains(text, "CUSTOM: original error") {
		t.Errorf("expected custom-formatted error, got: %q", text)
	}
}

func TestErrorRecoveryMiddleware_NilFormatErrorUsesDefault(t *testing.T) {
	t.Parallel()
	cfg := ErrorRecoveryConfig{
		MaxRetries:  0,
		FormatError: nil, // Should default to CompactError.
	}

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, errors.New("timeout reached")
	}

	wrapped := mw("nil-fmt", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}

	text := result.Content[0].(registry.TextContent).Text
	// CompactError should classify "timeout reached" as TIMEOUT.
	if !strings.Contains(text, "TIMEOUT") {
		t.Errorf("expected TIMEOUT classification from default CompactError, got: %q", text)
	}
}

func TestErrorRecoveryMiddleware_HandlerContractNoGoError(t *testing.T) {
	t.Parallel()
	// Verify the middleware always returns (result, nil) even when the
	// inner handler returns a Go error.
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 0
	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, errors.New("raw Go error")
	}

	wrapped := mw("contract-tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("middleware violated handler contract: returned Go error: %v", err)
	}
	if result == nil {
		t.Fatal("middleware violated handler contract: returned nil result")
	}
	if !result.IsError {
		t.Fatal("expected error result for Go-level error")
	}
}

func TestErrorRecoveryMiddleware_RetryWithDelay(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 2
	cfg.RetryDelay = 20 * time.Millisecond

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	var callCount atomic.Int32
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount.Add(1)
		return registry.MakeErrorResult("still failing"), nil
	}

	start := time.Now()
	wrapped := mw("delay-tool", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}
	if callCount.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", callCount.Load())
	}
	// 2 retries with 20ms delay each should take at least 40ms.
	if elapsed < 30*time.Millisecond {
		t.Errorf("expected at least ~40ms for 2 delayed retries, got %v", elapsed)
	}
}

// --- Concurrent safety ---

func TestErrorRecoveryMiddleware_Concurrent(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = 2

	var escalationCount atomic.Int32
	cfg.EscalateFunc = func(_ ErrorEscalation) {
		escalationCount.Add(1)
	}

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	var totalCalls atomic.Int32
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		totalCalls.Add(1)
		return registry.MakeErrorResult("concurrent failure"), nil
	}

	wrapped := mw("concurrent-tool", td, inner)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			result, err := wrapped(context.Background(), registry.CallToolRequest{})
			if err != nil {
				t.Errorf("unexpected Go error in goroutine: %v", err)
			}
			if result == nil || !result.IsError {
				t.Error("expected error result in goroutine")
			}
		}()
	}

	wg.Wait()

	// Each goroutine: 1 initial + 2 retries = 3 calls.
	expectedCalls := int32(goroutines * 3)
	if totalCalls.Load() != expectedCalls {
		t.Errorf("expected %d total calls across %d goroutines, got %d", expectedCalls, goroutines, totalCalls.Load())
	}

	// Each goroutine should escalate exactly once.
	if escalationCount.Load() != goroutines {
		t.Errorf("expected %d escalations, got %d", goroutines, escalationCount.Load())
	}
}

// --- CompactError tests ---

func TestCompactError_NilError(t *testing.T) {
	t.Parallel()
	if got := CompactError(nil); got != "" {
		t.Errorf("expected empty string for nil error, got: %q", got)
	}
}

func TestCompactError_Timeout(t *testing.T) {
	t.Parallel()
	result := CompactError(context.DeadlineExceeded)
	if !strings.Contains(result, "[TIMEOUT]") {
		t.Errorf("expected [TIMEOUT] classification, got: %q", result)
	}
	if !strings.Contains(result, "Hint:") {
		t.Errorf("expected recovery hint, got: %q", result)
	}
}

func TestCompactError_Cancelled(t *testing.T) {
	t.Parallel()
	result := CompactError(context.Canceled)
	if !strings.Contains(result, "[CANCELLED]") {
		t.Errorf("expected [CANCELLED] classification, got: %q", result)
	}
}

func TestCompactError_CircuitOpen(t *testing.T) {
	t.Parallel()
	result := CompactError(fmt.Errorf("wrapped: %w", ErrCircuitOpen))
	if !strings.Contains(result, "[CIRCUIT_OPEN]") {
		t.Errorf("expected [CIRCUIT_OPEN] classification, got: %q", result)
	}
}

func TestCompactError_RateLimited(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("rate limit exceeded for API"))
	if !strings.Contains(result, "[RATE_LIMITED]") {
		t.Errorf("expected [RATE_LIMITED] classification, got: %q", result)
	}
}

func TestCompactError_Permission(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("permission denied: cannot write to /etc"))
	if !strings.Contains(result, "[PERMISSION]") {
		t.Errorf("expected [PERMISSION] classification, got: %q", result)
	}
}

func TestCompactError_NotFound(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("file not found: config.toml"))
	if !strings.Contains(result, "[NOT_FOUND]") {
		t.Errorf("expected [NOT_FOUND] classification, got: %q", result)
	}
}

func TestCompactError_Network(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("connection refused: tcp 127.0.0.1:8080"))
	if !strings.Contains(result, "[NETWORK]") {
		t.Errorf("expected [NETWORK] classification, got: %q", result)
	}
}

func TestCompactError_Transient(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("unexpected EOF"))
	if !strings.Contains(result, "[TRANSIENT]") {
		t.Errorf("expected [TRANSIENT] classification, got: %q", result)
	}
	if !strings.Contains(result, "Hint:") {
		t.Errorf("expected recovery hint for transient error, got: %q", result)
	}
}

func TestCompactError_WrappedContextError(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("tool handler: %w", context.DeadlineExceeded)
	result := CompactError(wrapped)
	if !strings.Contains(result, "[TIMEOUT]") {
		t.Errorf("expected [TIMEOUT] for wrapped deadline error, got: %q", result)
	}
}

func TestCompactError_HTTP429(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("HTTP 429 Too Many Requests"))
	if !strings.Contains(result, "[RATE_LIMITED]") {
		t.Errorf("expected [RATE_LIMITED] for HTTP 429, got: %q", result)
	}
}

func TestCompactError_HTTP403(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("HTTP 403 Forbidden"))
	if !strings.Contains(result, "[PERMISSION]") {
		t.Errorf("expected [PERMISSION] for HTTP 403, got: %q", result)
	}
}

func TestCompactError_HTTP404(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("HTTP 404 Not Found"))
	if !strings.Contains(result, "[NOT_FOUND]") {
		t.Errorf("expected [NOT_FOUND] for HTTP 404, got: %q", result)
	}
}

func TestCompactError_DNSError(t *testing.T) {
	t.Parallel()
	result := CompactError(errors.New("DNS resolution failed for api.example.com"))
	if !strings.Contains(result, "[NETWORK]") {
		t.Errorf("expected [NETWORK] for DNS error, got: %q", result)
	}
}

// --- classifyError tests ---

func TestClassifyError_Nil(t *testing.T) {
	t.Parallel()
	if got := classifyError(nil); got != "UNKNOWN" {
		t.Errorf("expected UNKNOWN for nil error, got: %q", got)
	}
}

// --- DefaultErrorRecoveryConfig ---

func TestDefaultErrorRecoveryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 0 {
		t.Errorf("expected RetryDelay=0, got %v", cfg.RetryDelay)
	}
	if cfg.FormatError == nil {
		t.Error("expected non-nil FormatError in defaults")
	}
	if cfg.ShouldRetry != nil {
		t.Error("expected nil ShouldRetry in defaults (all retryable)")
	}
	if cfg.EscalateFunc != nil {
		t.Error("expected nil EscalateFunc in defaults")
	}
}

// --- Negative MaxRetries ---

func TestErrorRecoveryMiddleware_NegativeMaxRetries(t *testing.T) {
	t.Parallel()
	cfg := DefaultErrorRecoveryConfig()
	cfg.MaxRetries = -5 // Should be clamped to 0.

	mw := ErrorRecoveryMiddleware(cfg)
	td := registry.ToolDefinition{}

	var callCount atomic.Int32
	inner := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount.Add(1)
		return registry.MakeErrorResult("fail"), nil
	}

	wrapped := mw("neg-retry", td, inner)
	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call with negative max retries clamped to 0, got %d", callCount.Load())
	}
}
