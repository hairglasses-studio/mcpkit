//go:build !official_sdk

package gateway

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/session"
)

func makeSessionCtxWithID(id string) context.Context {
	s := &testSession{id: id}
	return session.WithSession(context.Background(), s)
}

// testSession is a minimal session.Session implementation for tests.
type testSession struct {
	id string
}

func (s *testSession) ID() string               { return s.id }
func (s *testSession) State() session.State      { return session.StateActive }
func (s *testSession) CreatedAt() time.Time      { return time.Now() }
func (s *testSession) ExpiresAt() time.Time      { return time.Time{} }
func (s *testSession) Get(key string) (any, bool) { return nil, false }
func (s *testSession) Set(key string, val any)   {}
func (s *testSession) Delete(key string)         {}
func (s *testSession) Close() error              { return nil }

func TestSessionAffinity_FirstRequestCreatesAffinity(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "backend1", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	sa := NewSessionAffinity(gw)

	// Before any request, no affinity.
	if sa.Len() != 0 {
		t.Fatalf("expected 0 affinities, got %d", sa.Len())
	}

	ctx := makeSessionCtxWithID("sess-1")
	mw := sa.Middleware()
	handler := mw("backend1.ping", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	result, err := handler(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Affinity should now be recorded.
	if sa.Len() != 1 {
		t.Fatalf("expected 1 affinity, got %d", sa.Len())
	}

	upstream, ok := sa.GetAffinity("sess-1")
	if !ok {
		t.Fatal("expected affinity for sess-1")
	}
	if upstream != "backend1" {
		t.Fatalf("expected affinity to backend1, got %q", upstream)
	}
}

func TestSessionAffinity_SameSessionSameUpstream(t *testing.T) {
	t.Parallel()
	_, cfg1 := newTestUpstream(t, "be1", echoTool("ping"))
	_, cfg2 := newTestUpstream(t, "be2", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg1); err != nil {
		t.Fatalf("AddUpstream be1: %v", err)
	}
	if _, err := gw.AddUpstream(context.Background(), cfg2); err != nil {
		t.Fatalf("AddUpstream be2: %v", err)
	}

	sa := NewSessionAffinity(gw)
	ctx := makeSessionCtxWithID("sticky-session")
	mw := sa.Middleware()

	handler := mw("be1.ping", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	// First call assigns an upstream.
	if _, err := handler(ctx, mcp.CallToolRequest{}); err != nil {
		t.Fatalf("first call: %v", err)
	}

	firstUpstream, _ := sa.GetAffinity("sticky-session")

	// Subsequent calls should route to the same upstream.
	for i := 0; i < 10; i++ {
		if _, err := handler(ctx, mcp.CallToolRequest{}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		current, _ := sa.GetAffinity("sticky-session")
		if current != firstUpstream {
			t.Fatalf("call %d: expected upstream %q, got %q", i, firstUpstream, current)
		}
	}
}

func TestSessionAffinity_DifferentSessionsDifferentUpstreams(t *testing.T) {
	t.Parallel()
	_, cfg1 := newTestUpstream(t, "up1", echoTool("ping"))
	_, cfg2 := newTestUpstream(t, "up2", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg1); err != nil {
		t.Fatalf("AddUpstream up1: %v", err)
	}
	if _, err := gw.AddUpstream(context.Background(), cfg2); err != nil {
		t.Fatalf("AddUpstream up2: %v", err)
	}

	// Use round-robin selector so different sessions get different upstreams.
	sa := NewSessionAffinity(gw)
	mw := sa.Middleware()

	handler := mw("up1.ping", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	// Create enough sessions to ensure at least two distinct upstreams are selected.
	assigned := make(map[string]string)
	for i := 0; i < 10; i++ {
		sessID := "session-" + string(rune('A'+i))
		ctx := makeSessionCtxWithID(sessID)
		if _, err := handler(ctx, mcp.CallToolRequest{}); err != nil {
			t.Fatalf("session %s call: %v", sessID, err)
		}
		upstream, _ := sa.GetAffinity(sessID)
		assigned[sessID] = upstream
	}

	// Collect unique upstreams.
	unique := make(map[string]bool)
	for _, up := range assigned {
		unique[up] = true
	}

	if len(unique) < 2 {
		t.Fatalf("expected at least 2 different upstreams, got %d: %v", len(unique), assigned)
	}
}

func TestSessionAffinity_UpstreamRemovalCleansAffinities(t *testing.T) {
	t.Parallel()
	_, cfg1 := newTestUpstream(t, "rem1", echoTool("ping"))
	_, cfg2 := newTestUpstream(t, "rem2", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg1); err != nil {
		t.Fatalf("AddUpstream rem1: %v", err)
	}
	if _, err := gw.AddUpstream(context.Background(), cfg2); err != nil {
		t.Fatalf("AddUpstream rem2: %v", err)
	}

	sa := NewSessionAffinity(gw)

	// Manually set affinities to control which sessions point where.
	sa.mu.Lock()
	sa.mapping["s1"] = "rem1"
	sa.mapping["s2"] = "rem1"
	sa.mapping["s3"] = "rem2"
	sa.mu.Unlock()

	if sa.Len() != 3 {
		t.Fatalf("expected 3 affinities, got %d", sa.Len())
	}

	// Clean up affinities for rem1.
	cleaned := sa.CleanupUpstream("rem1")
	if cleaned != 2 {
		t.Fatalf("expected 2 cleaned affinities, got %d", cleaned)
	}

	if sa.Len() != 1 {
		t.Fatalf("expected 1 remaining affinity, got %d", sa.Len())
	}

	// s3 should still be mapped.
	upstream, ok := sa.GetAffinity("s3")
	if !ok || upstream != "rem2" {
		t.Fatalf("expected s3→rem2, got %q (ok=%v)", upstream, ok)
	}

	// s1, s2 should be gone.
	if _, ok := sa.GetAffinity("s1"); ok {
		t.Fatal("expected s1 affinity to be removed")
	}
	if _, ok := sa.GetAffinity("s2"); ok {
		t.Fatal("expected s2 affinity to be removed")
	}
}

func TestSessionAffinity_MiddlewareReassignsOnUpstreamRemoval(t *testing.T) {
	t.Parallel()
	_, cfg1 := newTestUpstream(t, "alive", echoTool("ping"))
	_, cfg2 := newTestUpstream(t, "doomed", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg1); err != nil {
		t.Fatalf("AddUpstream alive: %v", err)
	}
	if _, err := gw.AddUpstream(context.Background(), cfg2); err != nil {
		t.Fatalf("AddUpstream doomed: %v", err)
	}

	sa := NewSessionAffinity(gw)

	// Manually assign a session to the doomed upstream.
	sa.mu.Lock()
	sa.mapping["reassign-sess"] = "doomed"
	sa.mu.Unlock()

	// Remove the doomed upstream from the gateway.
	gw.RemoveUpstream("doomed")

	ctx := makeSessionCtxWithID("reassign-sess")
	mw := sa.Middleware()
	handler := mw("alive.ping", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	// The middleware should detect the upstream is gone and reassign.
	_, err := handler(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler error after upstream removal: %v", err)
	}

	upstream, ok := sa.GetAffinity("reassign-sess")
	if !ok {
		t.Fatal("expected affinity to be reassigned")
	}
	if upstream != "alive" {
		t.Fatalf("expected reassignment to 'alive', got %q", upstream)
	}
}

func TestSessionAffinity_NoSessionPassesThrough(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()

	sa := NewSessionAffinity(gw)
	mw := sa.Middleware()

	called := false
	handler := mw("test.tool", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("pass"), nil
	})

	// No session in context — should pass through.
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !called {
		t.Fatal("expected next handler to be called")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if sa.Len() != 0 {
		t.Fatalf("expected no affinities for session-less request, got %d", sa.Len())
	}
}

func TestSessionAffinity_NoUpstreamsPassesThrough(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()

	sa := NewSessionAffinity(gw)
	mw := sa.Middleware()

	called := false
	handler := mw("test.tool", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("fallback"), nil
	})

	ctx := makeSessionCtxWithID("orphan-sess")
	result, err := handler(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !called {
		t.Fatal("expected next handler to be called when no upstreams")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSessionAffinity_RemoveAffinity(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()

	sa := NewSessionAffinity(gw)
	sa.mu.Lock()
	sa.mapping["remove-me"] = "upstream-x"
	sa.mu.Unlock()

	if sa.Len() != 1 {
		t.Fatalf("expected 1 affinity, got %d", sa.Len())
	}

	sa.RemoveAffinity("remove-me")

	if sa.Len() != 0 {
		t.Fatalf("expected 0 affinities after removal, got %d", sa.Len())
	}

	_, ok := sa.GetAffinity("remove-me")
	if ok {
		t.Fatal("expected no affinity after removal")
	}
}

func TestSessionAffinity_CleanupUpstreamNotFound(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()

	sa := NewSessionAffinity(gw)
	cleaned := sa.CleanupUpstream("nonexistent")
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned, got %d", cleaned)
	}
}

func TestSessionAffinity_CustomSelector(t *testing.T) {
	t.Parallel()
	_, cfg1 := newTestUpstream(t, "cs1", echoTool("op"))
	_, cfg2 := newTestUpstream(t, "cs2", echoTool("op"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg1); err != nil {
		t.Fatalf("AddUpstream cs1: %v", err)
	}
	if _, err := gw.AddUpstream(context.Background(), cfg2); err != nil {
		t.Fatalf("AddUpstream cs2: %v", err)
	}

	// Custom selector that always picks the last upstream alphabetically.
	sel := &lastSelector{}
	sa := NewSessionAffinity(gw, SessionAffinityConfig{Selector: sel})

	mw := sa.Middleware()
	handler := mw("cs1.op", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	ctx := makeSessionCtxWithID("custom-sess")
	if _, err := handler(ctx, mcp.CallToolRequest{}); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	upstream, ok := sa.GetAffinity("custom-sess")
	if !ok {
		t.Fatal("expected affinity")
	}
	// Should be "cs2" since lastSelector picks last alphabetically.
	if upstream != "cs2" {
		t.Fatalf("expected cs2, got %q", upstream)
	}
}

// lastSelector always picks the last upstream alphabetically (for deterministic tests).
type lastSelector struct{}

func (s *lastSelector) Select(upstreams []string) string {
	if len(upstreams) == 0 {
		return ""
	}
	sorted := make([]string, len(upstreams))
	copy(sorted, upstreams)
	sort.Strings(sorted)
	return sorted[len(sorted)-1]
}

func TestSessionAffinity_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "conc", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	sa := NewSessionAffinity(gw)
	mw := sa.Middleware()

	handler := mw("conc.ping", registry.ToolDefinition{}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	})

	const workers = 20
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		sessID := "conc-sess-" + string(rune('A'+i))
		go func(sid string) {
			defer wg.Done()
			ctx := makeSessionCtxWithID(sid)
			for j := 0; j < iterations; j++ {
				_, _ = handler(ctx, mcp.CallToolRequest{})
				_, _ = sa.GetAffinity(sid)
				_ = sa.Len()
			}
		}(sessID)
	}

	// Also run concurrent cleanups and removals.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < iterations; j++ {
			sa.CleanupUpstream("nonexistent")
			sa.RemoveAffinity("nonexistent-sess")
		}
	}()

	wg.Wait()

	// All worker sessions should have affinities.
	if sa.Len() != workers {
		t.Fatalf("expected %d affinities, got %d", workers, sa.Len())
	}
}

func TestRoundRobinSelector_EmptyList(t *testing.T) {
	t.Parallel()
	rr := &RoundRobinSelector{}
	result := rr.Select(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil upstreams, got %q", result)
	}
	result = rr.Select([]string{})
	if result != "" {
		t.Fatalf("expected empty string for empty upstreams, got %q", result)
	}
}

func TestRoundRobinSelector_Distribution(t *testing.T) {
	t.Parallel()
	rr := &RoundRobinSelector{}
	upstreams := []string{"a", "b", "c"}

	counts := make(map[string]int)
	for i := 0; i < 9; i++ {
		selected := rr.Select(upstreams)
		counts[selected]++
	}

	// Each upstream should be selected 3 times.
	for _, name := range upstreams {
		if counts[name] != 3 {
			t.Fatalf("expected 3 selections for %q, got %d", name, counts[name])
		}
	}
}

func TestNewSessionAffinity_DefaultSelector(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()

	sa := NewSessionAffinity(gw)
	if sa.selector == nil {
		t.Fatal("expected default selector to be set")
	}
	// Verify it's a RoundRobinSelector.
	if _, ok := sa.selector.(*RoundRobinSelector); !ok {
		t.Fatalf("expected *RoundRobinSelector, got %T", sa.selector)
	}
}

func TestNewSessionAffinity_NoConfig(t *testing.T) {
	t.Parallel()
	gw, _ := NewGateway()
	defer gw.Close()

	// Call with no config args.
	sa := NewSessionAffinity(gw)
	if sa == nil {
		t.Fatal("expected non-nil SessionAffinity")
	}
	if sa.gateway != gw {
		t.Fatal("expected gateway reference to match")
	}
}

func TestSessionAffinity_AssignUpstreamDoubleCheck(t *testing.T) {
	t.Parallel()
	_, cfg := newTestUpstream(t, "dc", echoTool("ping"))
	gw, _ := NewGateway()
	defer gw.Close()

	if _, err := gw.AddUpstream(context.Background(), cfg); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	sa := NewSessionAffinity(gw)

	// Pre-populate the affinity before calling assignUpstream to exercise
	// the double-check path inside assignUpstream.
	sa.mu.Lock()
	sa.mapping["pre-assigned"] = "dc"
	sa.mu.Unlock()

	result := sa.assignUpstream("pre-assigned")
	if result != "dc" {
		t.Fatalf("expected double-check to return existing 'dc', got %q", result)
	}
}
