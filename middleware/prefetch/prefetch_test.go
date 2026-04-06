//go:build !official_sdk

package prefetch

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/mark3labs/mcp-go/mcp"
)

// testHandler returns a handler that produces a simple text result.
func testHandler(text string) registry.ToolHandlerFunc {
	return func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult(text), nil
	}
}

// testReq builds a minimal CallToolRequest.
func testReq() registry.CallToolRequest {
	return registry.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "test_tool",
		},
	}
}

// contextCapturingHandler returns a handler that captures the context for inspection.
func contextCapturingHandler(ctxCh chan context.Context) registry.ToolHandlerFunc {
	return func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		ctxCh <- ctx
		return registry.MakeTextResult("ok"), nil
	}
}

func TestPrefetchMiddleware_InjectsData(t *testing.T) {
	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"git_status": {
				Fetch: func(_ context.Context) (any, error) {
					return "M README.md", nil
				},
				ShouldPrefetch: nil, // nil means match all tools
			},
		},
		CacheTTL:      5 * time.Minute,
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	ctxCh := make(chan context.Context, 1)
	wrapped := mw("test_tool", td, contextCapturingHandler(ctxCh))

	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	capturedCtx := <-ctxCh
	val, ok := PrefetchFromContext[string](capturedCtx, "git_status")
	if !ok {
		t.Fatal("expected git_status to be present in context")
	}
	if val != "M README.md" {
		t.Errorf("expected %q, got %q", "M README.md", val)
	}
}

func TestPrefetchMiddleware_ShouldPrefetchFilter(t *testing.T) {
	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"git_status": {
				Fetch: func(_ context.Context) (any, error) {
					return "dirty", nil
				},
				ShouldPrefetch: func(toolName string) bool {
					return toolName == "git_commit" // only for git_commit
				},
			},
		},
		CacheTTL:      5 * time.Minute,
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	// Call with a non-matching tool name.
	ctxCh := make(chan context.Context, 1)
	wrapped := mw("other_tool", td, contextCapturingHandler(ctxCh))

	_, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	capturedCtx := <-ctxCh
	_, ok := PrefetchFromContext[string](capturedCtx, "git_status")
	if ok {
		t.Error("git_status should not be prefetched for other_tool")
	}

	// Call with a matching tool name.
	ctxCh2 := make(chan context.Context, 1)
	wrapped2 := mw("git_commit", td, contextCapturingHandler(ctxCh2))

	_, err = wrapped2(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	capturedCtx2 := <-ctxCh2
	val, ok := PrefetchFromContext[string](capturedCtx2, "git_status")
	if !ok {
		t.Fatal("expected git_status to be present for git_commit")
	}
	if val != "dirty" {
		t.Errorf("expected %q, got %q", "dirty", val)
	}
}

func TestPrefetchMiddleware_CachesTTL(t *testing.T) {
	var fetchCount atomic.Int32

	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"expensive_data": {
				Fetch: func(_ context.Context) (any, error) {
					fetchCount.Add(1)
					return "fetched", nil
				},
			},
		},
		CacheTTL:      1 * time.Hour, // long TTL ensures cache hits
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	// Call the tool 5 times. Fetch should only happen once.
	for i := range 5 {
		wrapped := mw("test_tool", td, testHandler("ok"))
		_, err := wrapped(context.Background(), testReq())
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}

	count := fetchCount.Load()
	if count != 1 {
		t.Errorf("expected 1 fetch (cached), got %d", count)
	}
}

func TestPrefetchMiddleware_CacheExpires(t *testing.T) {
	var fetchCount atomic.Int32

	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"volatile_data": {
				Fetch: func(_ context.Context) (any, error) {
					fetchCount.Add(1)
					return "data", nil
				},
			},
		},
		CacheTTL:      10 * time.Millisecond, // very short TTL
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	// First call: triggers fetch.
	wrapped := mw("test_tool", td, testHandler("ok"))
	_, _ = wrapped(context.Background(), testReq())

	if fetchCount.Load() != 1 {
		t.Fatal("expected first call to trigger fetch")
	}

	// Wait for cache to expire.
	time.Sleep(20 * time.Millisecond)

	// Second call: should trigger fresh fetch.
	wrapped = mw("test_tool", td, testHandler("ok"))
	_, _ = wrapped(context.Background(), testReq())

	count := fetchCount.Load()
	if count != 2 {
		t.Errorf("expected 2 fetches after TTL expiry, got %d", count)
	}
}

func TestPrefetchMiddleware_Concurrent(t *testing.T) {
	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32

	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"slow_a": {
				Fetch: func(_ context.Context) (any, error) {
					cur := currentConcurrent.Add(1)
					// Track peak concurrency.
					for {
						old := maxConcurrent.Load()
						if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					currentConcurrent.Add(-1)
					return "a", nil
				},
			},
			"slow_b": {
				Fetch: func(_ context.Context) (any, error) {
					cur := currentConcurrent.Add(1)
					for {
						old := maxConcurrent.Load()
						if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					currentConcurrent.Add(-1)
					return "b", nil
				},
			},
			"slow_c": {
				Fetch: func(_ context.Context) (any, error) {
					cur := currentConcurrent.Add(1)
					for {
						old := maxConcurrent.Load()
						if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					currentConcurrent.Add(-1)
					return "c", nil
				},
			},
		},
		CacheTTL:      1 * time.Hour,
		MaxConcurrent: 2, // limit to 2 concurrent
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	ctxCh := make(chan context.Context, 1)
	wrapped := mw("test_tool", td, contextCapturingHandler(ctxCh))

	_, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	capturedCtx := <-ctxCh

	// All three should be fetched.
	for _, key := range []string{"slow_a", "slow_b", "slow_c"} {
		val, ok := PrefetchFromContext[string](capturedCtx, key)
		if !ok {
			t.Errorf("expected %s to be present in context", key)
		}
		if val == "" {
			t.Errorf("expected non-empty value for %s", key)
		}
	}

	// Max concurrent should not exceed 2.
	peak := maxConcurrent.Load()
	if peak > 2 {
		t.Errorf("max concurrent should not exceed 2, got %d", peak)
	}
}

func TestPrefetchMiddleware_ProviderError(t *testing.T) {
	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"good_data": {
				Fetch: func(_ context.Context) (any, error) {
					return "healthy", nil
				},
			},
			"broken_data": {
				Fetch: func(_ context.Context) (any, error) {
					return nil, errors.New("connection refused")
				},
			},
		},
		CacheTTL:      5 * time.Minute,
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	ctxCh := make(chan context.Context, 1)
	wrapped := mw("test_tool", td, contextCapturingHandler(ctxCh))

	// Should not error even though one provider fails.
	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	capturedCtx := <-ctxCh

	// Good data should be present.
	val, ok := PrefetchFromContext[string](capturedCtx, "good_data")
	if !ok {
		t.Fatal("expected good_data to be present")
	}
	if val != "healthy" {
		t.Errorf("expected %q, got %q", "healthy", val)
	}

	// Broken data should be absent (graceful degradation).
	_, ok = PrefetchFromContext[string](capturedCtx, "broken_data")
	if ok {
		t.Error("broken_data should not be present after fetch error")
	}
}

func TestPrefetchFromContext_TypeSafety(t *testing.T) {
	// Store a string value.
	ctx := context.WithValue(context.Background(), contextKey("data"), "hello")

	// Correct type: should succeed.
	val, ok := PrefetchFromContext[string](ctx, "data")
	if !ok {
		t.Fatal("expected string extraction to succeed")
	}
	if val != "hello" {
		t.Errorf("expected %q, got %q", "hello", val)
	}

	// Wrong type: should fail gracefully.
	intVal, ok := PrefetchFromContext[int](ctx, "data")
	if ok {
		t.Error("expected int extraction to fail for string value")
	}
	if intVal != 0 {
		t.Errorf("expected zero value for int, got %d", intVal)
	}

	// Missing key: should fail gracefully.
	missing, ok := PrefetchFromContext[string](ctx, "nonexistent")
	if ok {
		t.Error("expected missing key to return false")
	}
	if missing != "" {
		t.Errorf("expected empty string for missing key, got %q", missing)
	}
}

func TestPrefetchFromContext_ComplexTypes(t *testing.T) {
	type GitStatus struct {
		Branch  string
		Changed int
	}

	status := GitStatus{Branch: "main", Changed: 3}
	ctx := context.WithValue(context.Background(), contextKey("git"), status)

	val, ok := PrefetchFromContext[GitStatus](ctx, "git")
	if !ok {
		t.Fatal("expected struct extraction to succeed")
	}
	if val.Branch != "main" || val.Changed != 3 {
		t.Errorf("unexpected struct: %+v", val)
	}

	// Pointer vs value mismatch: should fail.
	_, ok = PrefetchFromContext[*GitStatus](ctx, "git")
	if ok {
		t.Error("expected pointer extraction to fail for value type")
	}
}

func TestPrefetchMiddleware_NoProviders(t *testing.T) {
	// Empty providers should be zero-overhead passthrough.
	cfg := Config{
		Providers:     map[string]PrefetchProvider{},
		CacheTTL:      5 * time.Minute,
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	called := false
	handler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	}

	wrapped := mw("test_tool", td, handler)
	result, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler should be called")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestPrefetchMiddleware_MultipleProviders(t *testing.T) {
	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"user_info": {
				Fetch: func(_ context.Context) (any, error) {
					return map[string]string{"name": "alice"}, nil
				},
			},
			"system_load": {
				Fetch: func(_ context.Context) (any, error) {
					return 0.42, nil
				},
			},
			"config": {
				Fetch: func(_ context.Context) (any, error) {
					return []string{"debug", "verbose"}, nil
				},
			},
		},
		CacheTTL:      5 * time.Minute,
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	ctxCh := make(chan context.Context, 1)
	wrapped := mw("test_tool", td, contextCapturingHandler(ctxCh))

	_, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	capturedCtx := <-ctxCh

	userInfo, ok := PrefetchFromContext[map[string]string](capturedCtx, "user_info")
	if !ok {
		t.Fatal("expected user_info in context")
	}
	if userInfo["name"] != "alice" {
		t.Errorf("unexpected user_info: %v", userInfo)
	}

	load, ok := PrefetchFromContext[float64](capturedCtx, "system_load")
	if !ok {
		t.Fatal("expected system_load in context")
	}
	if load != 0.42 {
		t.Errorf("expected 0.42, got %f", load)
	}

	flags, ok := PrefetchFromContext[[]string](capturedCtx, "config")
	if !ok {
		t.Fatal("expected config in context")
	}
	if len(flags) != 2 || flags[0] != "debug" {
		t.Errorf("unexpected config: %v", flags)
	}
}

func TestNew_FunctionalOptions(t *testing.T) {
	var fetchCalled atomic.Bool

	mw := New(
		WithProvider("data", func(_ context.Context) (any, error) {
			fetchCalled.Store(true)
			return "fetched", nil
		}, nil),
		WithCacheTTL(10*time.Minute),
		WithMaxConcurrent(8),
	)

	td := registry.ToolDefinition{}
	ctxCh := make(chan context.Context, 1)
	wrapped := mw("test_tool", td, contextCapturingHandler(ctxCh))

	_, err := wrapped(context.Background(), testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fetchCalled.Load() {
		t.Error("expected provider to be called")
	}

	capturedCtx := <-ctxCh
	val, ok := PrefetchFromContext[string](capturedCtx, "data")
	if !ok {
		t.Fatal("expected data in context")
	}
	if val != "fetched" {
		t.Errorf("expected %q, got %q", "fetched", val)
	}
}

func TestPrefetchMiddleware_ThreadSafe(t *testing.T) {
	var fetchCount atomic.Int32

	cfg := Config{
		Providers: map[string]PrefetchProvider{
			"shared": {
				Fetch: func(_ context.Context) (any, error) {
					fetchCount.Add(1)
					return "shared_value", nil
				},
			},
		},
		CacheTTL:      1 * time.Hour,
		MaxConcurrent: 4,
	}

	mw := Middleware(cfg)
	td := registry.ToolDefinition{}

	// Run 20 concurrent tool calls.
	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctxCh := make(chan context.Context, 1)
			wrapped := mw("test_tool", td, contextCapturingHandler(ctxCh))

			_, err := wrapped(context.Background(), testReq())
			if err != nil {
				errs <- err
				return
			}

			capturedCtx := <-ctxCh
			val, ok := PrefetchFromContext[string](capturedCtx, "shared")
			if !ok {
				errs <- errors.New("shared data missing from context")
				return
			}
			if val != "shared_value" {
				errs <- errors.New("unexpected shared value: " + val)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestPrefetchMiddleware_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.CacheTTL != DefaultCacheTTL {
		t.Errorf("expected default TTL %v, got %v", DefaultCacheTTL, cfg.CacheTTL)
	}
	if cfg.MaxConcurrent != DefaultMaxConcurrent {
		t.Errorf("expected default max concurrent %d, got %d", DefaultMaxConcurrent, cfg.MaxConcurrent)
	}
	if cfg.Providers == nil {
		t.Error("expected non-nil providers map")
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty providers, got %d", len(cfg.Providers))
	}
}
