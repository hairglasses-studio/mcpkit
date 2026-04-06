//go:build !official_sdk

package finops

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- Test helpers ---

// mockCostTracker is a minimal CostTracker for testing.
type mockCostTracker struct {
	mu     sync.RWMutex
	total  float64
	budget float64
}

func newMockTracker(budget float64) *mockCostTracker {
	return &mockCostTracker{budget: budget}
}

func (m *mockCostTracker) RecordCost(cost float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.total += cost
	if m.budget > 0 && m.total > m.budget {
		return &DollarBudgetExceededError{Limit: m.budget, Used: m.total}
	}
	return nil
}

func (m *mockCostTracker) TotalCost() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.total
}

func (m *mockCostTracker) RemainingBudget() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.budget <= 0 {
		return 0
	}
	r := m.budget - m.total
	if r < 0 {
		return 0
	}
	return r
}

func makeEnforcementTestRequest(name string, args map[string]any) registry.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// --- Tests ---

// TestBudgetEnforcer_UnderBudget verifies that Check returns ActionWarn and no
// error when spending is well under the budget limit.
func TestBudgetEnforcer_UnderBudget(t *testing.T) {
	t.Parallel()

	enforcer := NewBudgetEnforcer(nil, nil)
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "daily",
		Limit:  10.0,
		Action: ActionBlock,
	})

	// Record some cost well under the limit.
	enforcer.RecordCost(2.0)

	action, err := enforcer.Check(context.Background())
	if err != nil {
		t.Fatalf("expected no error under budget, got: %v", err)
	}
	if action != ActionWarn {
		t.Errorf("expected ActionWarn under budget, got: %s", action)
	}

	if enforcer.CurrentSpend() != 2.0 {
		t.Errorf("expected CurrentSpend=2.0, got %f", enforcer.CurrentSpend())
	}
	if enforcer.RemainingBudget() != 8.0 {
		t.Errorf("expected RemainingBudget=8.0, got %f", enforcer.RemainingBudget())
	}
}

// TestBudgetEnforcer_AlertThresholds verifies that alerts fire at the correct
// threshold percentages (50%, 75%, 90%) and that each alert fires only once.
func TestBudgetEnforcer_AlertThresholds(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var alerts []BudgetAlert

	alertFunc := func(alert BudgetAlert) {
		mu.Lock()
		alerts = append(alerts, alert)
		mu.Unlock()
	}

	enforcer := NewBudgetEnforcer(nil, alertFunc)
	enforcer.AddPolicy(BudgetPolicy{
		Name:       "daily",
		Limit:      100.0,
		Thresholds: []float64{0.50, 0.75, 0.90},
		Action:     ActionBlock,
	})

	// Record $40 — under all thresholds.
	enforcer.RecordCost(40.0)
	mu.Lock()
	count := len(alerts)
	mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 alerts at 40%%, got %d", count)
	}

	// Record $15 more — now at $55 (55%), should trigger 50% threshold.
	enforcer.RecordCost(15.0)
	mu.Lock()
	count = len(alerts)
	mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 alert at 55%%, got %d", count)
	}
	mu.Lock()
	if alerts[0].Threshold != 0.50 {
		t.Errorf("expected first alert at threshold 0.50, got %f", alerts[0].Threshold)
	}
	mu.Unlock()

	// Record $25 more — now at $80 (80%), should trigger 75% threshold.
	enforcer.RecordCost(25.0)
	mu.Lock()
	count = len(alerts)
	mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 alerts at 80%%, got %d", count)
	}
	mu.Lock()
	if alerts[1].Threshold != 0.75 {
		t.Errorf("expected second alert at threshold 0.75, got %f", alerts[1].Threshold)
	}
	mu.Unlock()

	// Record $15 more — now at $95 (95%), should trigger 90% threshold.
	enforcer.RecordCost(15.0)
	mu.Lock()
	count = len(alerts)
	mu.Unlock()
	if count != 3 {
		t.Fatalf("expected 3 alerts at 95%%, got %d", count)
	}
	mu.Lock()
	if alerts[2].Threshold != 0.90 {
		t.Errorf("expected third alert at threshold 0.90, got %f", alerts[2].Threshold)
	}
	mu.Unlock()

	// Record another $5 — still at $100, thresholds already fired. No new alerts.
	enforcer.RecordCost(5.0)
	mu.Lock()
	count = len(alerts)
	mu.Unlock()
	if count != 3 {
		t.Errorf("expected no new alerts after re-crossing thresholds, got %d", count)
	}
}

// TestBudgetEnforcer_BlockWhenExceeded verifies that Check returns ActionBlock
// and an error when the budget limit is exceeded.
func TestBudgetEnforcer_BlockWhenExceeded(t *testing.T) {
	t.Parallel()

	enforcer := NewBudgetEnforcer(nil, nil)
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "daily",
		Limit:  10.0,
		Action: ActionBlock,
	})

	// Exceed the budget.
	enforcer.RecordCost(15.0)

	action, err := enforcer.Check(context.Background())
	if action != ActionBlock {
		t.Errorf("expected ActionBlock, got %s", action)
	}
	if err == nil {
		t.Fatal("expected error when budget exceeded")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected error containing 'budget exceeded', got: %v", err)
	}
}

// TestBudgetEnforcer_DowngradeAction verifies that Check returns ActionDowngrade
// (without error) when a downgrade-action policy is exceeded.
func TestBudgetEnforcer_DowngradeAction(t *testing.T) {
	t.Parallel()

	enforcer := NewBudgetEnforcer(nil, nil)
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "cost-optimization",
		Limit:  5.0,
		Action: ActionDowngrade,
	})

	enforcer.RecordCost(6.0)

	action, err := enforcer.Check(context.Background())
	if err != nil {
		t.Fatalf("expected no error for downgrade action, got: %v", err)
	}
	if action != ActionDowngrade {
		t.Errorf("expected ActionDowngrade, got %s", action)
	}
}

// TestBudgetEnforcer_WarnAction verifies that Check returns ActionWarn even when
// the budget is exceeded if the policy action is warn.
func TestBudgetEnforcer_WarnAction(t *testing.T) {
	t.Parallel()

	var alertFired bool
	enforcer := NewBudgetEnforcer(nil, func(alert BudgetAlert) {
		alertFired = true
	})
	enforcer.AddPolicy(BudgetPolicy{
		Name:       "soft-limit",
		Limit:      5.0,
		Thresholds: []float64{0.9},
		Action:     ActionWarn,
	})

	enforcer.RecordCost(6.0)

	action, err := enforcer.Check(context.Background())
	if err != nil {
		t.Fatalf("expected no error for warn action, got: %v", err)
	}
	if action != ActionWarn {
		t.Errorf("expected ActionWarn, got %s", action)
	}
	if !alertFired {
		t.Error("expected alert to fire when exceeding threshold")
	}
}

// TestEnforcementMiddleware_Blocks verifies that the middleware blocks tool
// execution when the budget is exceeded with ActionBlock.
func TestEnforcementMiddleware_Blocks(t *testing.T) {
	t.Parallel()

	enforcer := NewBudgetEnforcer(nil, nil)
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "daily",
		Limit:  1.0,
		Action: ActionBlock,
	})

	handlerCalls := 0
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		handlerCalls++
		return registry.MakeTextResult("result"), nil
	}

	mw := EnforcementMiddleware(enforcer)
	wrapped := mw("test-tool", registry.ToolDefinition{Category: "test"}, handler)

	ctx := context.Background()
	req := makeEnforcementTestRequest("test-tool", map[string]any{"q": "hello world test input"})

	// First call should succeed (budget not yet exceeded before execution).
	result1, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if registry.IsResultError(result1) {
		t.Fatal("first call: expected success, got error result")
	}
	if handlerCalls != 1 {
		t.Errorf("first call: expected handler called once, got %d", handlerCalls)
	}

	// Manually push the enforcer over the budget limit to simulate accumulated spend.
	enforcer.RecordCost(2.0)

	// The second call should be blocked because Check() sees cost > limit.
	result2, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if !registry.IsResultError(result2) {
		t.Error("second call: expected error result when budget exceeded")
	}
	if handlerCalls != 1 {
		t.Errorf("expected handler NOT called on blocked call, total calls: %d", handlerCalls)
	}

	// Verify the error message.
	for _, c := range result2.Content {
		if text, ok := registry.ExtractTextContent(c); ok {
			if !strings.Contains(text, "budget exceeded") {
				t.Errorf("expected error containing 'budget exceeded', got: %q", text)
			}
		}
	}
}

// TestEnforcementMiddleware_Passes verifies that the middleware allows tool
// execution when the budget is not exceeded.
func TestEnforcementMiddleware_Passes(t *testing.T) {
	t.Parallel()

	enforcer := NewBudgetEnforcer(nil, nil)
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "generous",
		Limit:  1000.0, // Large budget — won't be exceeded.
		Action: ActionBlock,
	})

	handlerCalls := 0
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		handlerCalls++
		return registry.MakeTextResult("ok"), nil
	}

	mw := EnforcementMiddleware(enforcer)
	wrapped := mw("test-tool", registry.ToolDefinition{Category: "test"}, handler)

	ctx := context.Background()
	req := makeEnforcementTestRequest("test-tool", map[string]any{"q": "x"})

	for i := range 10 {
		result, err := wrapped(ctx, req)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if registry.IsResultError(result) {
			t.Fatalf("call %d: unexpected error result", i+1)
		}
	}

	if handlerCalls != 10 {
		t.Errorf("expected 10 handler calls, got %d", handlerCalls)
	}
}

// TestBudgetEnforcer_Concurrent verifies thread safety of the enforcer under
// concurrent access from multiple goroutines.
func TestBudgetEnforcer_Concurrent(t *testing.T) {
	t.Parallel()

	var alertCount atomic.Int64
	enforcer := NewBudgetEnforcer(nil, func(alert BudgetAlert) {
		alertCount.Add(1)
	})
	enforcer.AddPolicy(BudgetPolicy{
		Name:       "concurrent-test",
		Limit:      100.0,
		Thresholds: []float64{0.5, 0.75, 0.9},
		Action:     ActionBlock,
	})

	var wg sync.WaitGroup
	goroutines := 50
	recordsPerGoroutine := 20

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range recordsPerGoroutine {
				enforcer.RecordCost(0.1)
				enforcer.Check(context.Background()) //nolint:errcheck
				_ = enforcer.CurrentSpend()
				_ = enforcer.RemainingBudget()
			}
		}()
	}

	wg.Wait()

	// Total cost should be goroutines * recordsPerGoroutine * 0.1 = 100.0
	expectedCost := float64(goroutines*recordsPerGoroutine) * 0.1
	actualCost := enforcer.CurrentSpend()

	// Allow small floating-point tolerance.
	if actualCost < expectedCost-0.01 || actualCost > expectedCost+0.01 {
		t.Errorf("expected total cost ~%.2f, got %.2f", expectedCost, actualCost)
	}

	// All three thresholds should have fired at least once.
	if alertCount.Load() < 3 {
		t.Errorf("expected at least 3 threshold alerts, got %d", alertCount.Load())
	}
}

// TestBudgetEnforcer_WithExternalTracker verifies the enforcer works with
// an external CostTracker implementation.
func TestBudgetEnforcer_WithExternalTracker(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker(10.0)
	enforcer := NewBudgetEnforcer(tracker, nil)
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "external",
		Limit:  10.0,
		Action: ActionBlock,
	})

	enforcer.RecordCost(3.0)
	if enforcer.CurrentSpend() != 3.0 {
		t.Errorf("expected CurrentSpend=3.0, got %f", enforcer.CurrentSpend())
	}

	enforcer.RecordCost(5.0)
	if enforcer.CurrentSpend() != 8.0 {
		t.Errorf("expected CurrentSpend=8.0, got %f", enforcer.CurrentSpend())
	}

	// Check should pass — still under budget.
	action, err := enforcer.Check(context.Background())
	if err != nil {
		t.Fatalf("expected no error at $8, got: %v", err)
	}
	if action != ActionWarn {
		t.Errorf("expected ActionWarn, got %s", action)
	}

	// Exceed the budget.
	enforcer.RecordCost(5.0)

	action, err = enforcer.Check(context.Background())
	if action != ActionBlock {
		t.Errorf("expected ActionBlock, got %s", action)
	}
	if err == nil {
		t.Fatal("expected error when budget exceeded")
	}
}

// TestBudgetEnforcer_MultiplePolicies verifies that the most restrictive policy
// wins when multiple policies are configured.
func TestBudgetEnforcer_MultiplePolicies(t *testing.T) {
	t.Parallel()

	enforcer := NewBudgetEnforcer(nil, nil)

	// Lenient policy — warn only.
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "monthly",
		Limit:  100.0,
		Action: ActionWarn,
	})

	// Strict policy — block.
	enforcer.AddPolicy(BudgetPolicy{
		Name:   "daily",
		Limit:  5.0,
		Action: ActionBlock,
	})

	// Exceed the strict (daily) policy but stay under the lenient (monthly).
	enforcer.RecordCost(6.0)

	action, err := enforcer.Check(context.Background())
	if action != ActionBlock {
		t.Errorf("expected ActionBlock from strict policy, got %s", action)
	}
	if err == nil {
		t.Fatal("expected error from strict block policy")
	}
}

// TestBudgetEnforcer_NoPolicies verifies that an enforcer with no policies
// always returns ActionWarn with no error.
func TestBudgetEnforcer_NoPolicies(t *testing.T) {
	t.Parallel()

	enforcer := NewBudgetEnforcer(nil, nil)
	enforcer.RecordCost(1000.0)

	action, err := enforcer.Check(context.Background())
	if err != nil {
		t.Fatalf("expected no error with no policies, got: %v", err)
	}
	if action != ActionWarn {
		t.Errorf("expected ActionWarn with no policies, got %s", action)
	}
}
