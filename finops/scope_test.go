//go:build !official_sdk

package finops

import (
	"context"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// resolveTo returns a TenantResolver that always resolves to the given identity.
func resolveTo(tenantID, userID, sessionID string) TenantResolver {
	return func(ctx context.Context) (string, string, string, bool) {
		return tenantID, userID, sessionID, true
	}
}

// resolveNone returns a TenantResolver that never resolves.
func resolveNone() TenantResolver {
	return func(ctx context.Context) (string, string, string, bool) {
		return "", "", "", false
	}
}

func TestScopedTracker_GetTracker_CreatesOnDemand(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)

	scope := BudgetScope{TenantID: "tenant-1"}
	t1 := st.GetTracker(scope)
	t2 := st.GetTracker(scope)

	if t1 != t2 {
		t.Error("expected GetTracker to return the same Tracker for the same scope")
	}
}

func TestScopedTracker_SetBudget(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)
	scope := BudgetScope{TenantID: "tenant-1"}
	st.SetBudget(scope, ScopedBudget{MaxTokens: 100})

	st.mu.RLock()
	b, ok := st.budgets[scope]
	st.mu.RUnlock()

	if !ok {
		t.Fatal("expected budget to be stored")
	}
	if b.MaxTokens != 100 {
		t.Errorf("expected MaxTokens=100, got %d", b.MaxTokens)
	}
}

func TestScopedTracker_Usage(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)
	scope := BudgetScope{TenantID: "acme", UserID: "alice"}

	tracker := st.GetTracker(scope)
	tracker.Record(UsageEntry{InputTokens: 10, OutputTokens: 20, ToolName: "tool", Category: "cat"})

	summary := st.Usage(scope)
	if summary.TotalInputTokens != 10 {
		t.Errorf("expected TotalInputTokens=10, got %d", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 20 {
		t.Errorf("expected TotalOutputTokens=20, got %d", summary.TotalOutputTokens)
	}
}

func TestScopedMiddleware_RecordsToGlobalAndScoped(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)
	scope := BudgetScope{TenantID: "tenant-x", UserID: "user-1"}

	mw := ScopedMiddleware(st, resolveTo("tenant-x", "user-1", ""))
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("hello response"), nil
	}
	wrapped := mw("my-tool", registry.ToolDefinition{Category: "test"}, handler)

	_, err := wrapped(context.Background(), makeTestRequest("my-tool", map[string]any{"q": "hi"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Global tracker should have 1 invocation.
	globalSummary := global.Summary()
	if globalSummary.TotalInvocations != 1 {
		t.Errorf("expected global TotalInvocations=1, got %d", globalSummary.TotalInvocations)
	}

	// Scoped tracker should also have 1 invocation.
	scopedSummary := st.Usage(scope)
	if scopedSummary.TotalInvocations != 1 {
		t.Errorf("expected scoped TotalInvocations=1, got %d", scopedSummary.TotalInvocations)
	}
}

func TestScopedMiddleware_TokenBudgetEnforcement(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)
	scope := BudgetScope{TenantID: "tenant-budget"}

	// Very small budget — 1 token max.
	st.SetBudget(scope, ScopedBudget{MaxTokens: 1})

	mw := ScopedMiddleware(st, resolveTo("tenant-budget", "", ""))
	callCount := 0
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount++
		return registry.MakeTextResult("hello world response text"), nil
	}
	wrapped := mw("tool", registry.ToolDefinition{Category: "cat"}, handler)

	ctx := context.Background()
	req := makeTestRequest("tool", map[string]any{"x": "y"})

	// Call repeatedly until budget is enforced.
	var errResult *registry.CallToolResult
	for i := range 20 {
		r, e := wrapped(ctx, req)
		if e != nil {
			t.Fatalf("call %d: unexpected Go error: %v", i+1, e)
		}
		if registry.IsResultError(r) {
			errResult = r
			break
		}
	}

	if errResult == nil {
		t.Fatal("expected budget-exceeded error result within 20 calls, got none")
	}

	for _, c := range errResult.Content {
		if text, ok := registry.ExtractTextContent(c); ok {
			if !strings.Contains(text, "budget exceeded") {
				t.Errorf("expected 'budget exceeded' in error message, got: %q", text)
			}
		}
	}
}

func TestScopedMiddleware_MissingTenant_Passthrough(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)

	mw := ScopedMiddleware(st, resolveNone())
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}
	wrapped := mw("tool", registry.ToolDefinition{}, handler)

	result, err := wrapped(context.Background(), makeTestRequest("tool", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success result when no tenant is resolved, got error")
	}

	// Global should still record.
	if global.Summary().TotalInvocations != 1 {
		t.Errorf("expected global TotalInvocations=1, got %d", global.Summary().TotalInvocations)
	}
}

func TestScopedMiddleware_NilResolver_Passthrough(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)

	mw := ScopedMiddleware(st, nil)
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}
	wrapped := mw("tool", registry.ToolDefinition{}, handler)

	result, err := wrapped(context.Background(), makeTestRequest("tool", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success result when resolver is nil")
	}
}

func TestScopedMiddleware_SeparateTenantBudgets(t *testing.T) {
	t.Parallel()

	global := NewTracker()
	st := NewScopedTracker(global)

	scopeA := BudgetScope{TenantID: "tenant-a"}
	_ = BudgetScope{TenantID: "tenant-b"} // tenant-b has no budget configured

	// tenant-a has a tiny budget; tenant-b has no budget.
	st.SetBudget(scopeA, ScopedBudget{MaxTokens: 1})

	callCount := 0
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount++
		return registry.MakeTextResult("response with some text"), nil
	}

	// Exhaust tenant-a budget.
	mwA := ScopedMiddleware(st, resolveTo("tenant-a", "", ""))
	wrappedA := mwA("tool", registry.ToolDefinition{Category: "c"}, handler)
	ctx := context.Background()
	req := makeTestRequest("tool", map[string]any{"k": "v"})

	var aBlocked bool
	for range 20 {
		r, _ := wrappedA(ctx, req)
		if registry.IsResultError(r) {
			aBlocked = true
			break
		}
	}
	if !aBlocked {
		t.Fatal("expected tenant-a to hit budget limit")
	}

	// tenant-b should still work.
	mwB := ScopedMiddleware(st, resolveTo("tenant-b", "", ""))
	wrappedB := mwB("tool", registry.ToolDefinition{Category: "c"}, handler)
	r, err := wrappedB(ctx, req)
	if err != nil || registry.IsResultError(r) {
		t.Error("expected tenant-b to succeed even when tenant-a is over budget")
	}
}
