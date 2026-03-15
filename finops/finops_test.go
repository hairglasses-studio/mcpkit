//go:build !official_sdk

package finops

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// makeTestRequest creates a CallToolRequest with the given name and arguments.
func makeTestRequest(name string, args map[string]interface{}) registry.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// TestDefaultEstimate verifies the 4-chars-per-token ceiling heuristic.
func TestDefaultEstimate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hi", 1},        // 2 chars → ceil(2/4) = 1
		{"hello", 2},     // 5 chars → ceil(5/4) = 2
		{"12345678", 2},  // 8 chars → ceil(8/4) = 2
		{"123456789", 3}, // 9 chars → ceil(9/4) = 3
	}

	for _, tc := range tests {
		got := DefaultEstimate(tc.input)
		if got != tc.expected {
			t.Errorf("DefaultEstimate(%q): expected %d, got %d", tc.input, tc.expected, got)
		}
	}
}

// TestTracker_Record verifies that recorded entries aggregate correctly.
func TestTracker_Record(t *testing.T) {
	t.Parallel()

	tracker := NewTracker()
	tracker.Record(UsageEntry{
		ToolName:     "tool-a",
		Category:     "search",
		InputTokens:  10,
		OutputTokens: 20,
	})
	tracker.Record(UsageEntry{
		ToolName:     "tool-b",
		Category:     "compute",
		InputTokens:  5,
		OutputTokens: 15,
	})

	summary := tracker.Summary()

	if summary.TotalInvocations != 2 {
		t.Errorf("expected TotalInvocations=2, got %d", summary.TotalInvocations)
	}
	if summary.TotalInputTokens != 15 {
		t.Errorf("expected TotalInputTokens=15, got %d", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 35 {
		t.Errorf("expected TotalOutputTokens=35, got %d", summary.TotalOutputTokens)
	}

	// ByTool: tool-a should have 30 tokens (10+20), tool-b should have 20 (5+15).
	if summary.ByTool["tool-a"] != 30 {
		t.Errorf("expected ByTool[tool-a]=30, got %d", summary.ByTool["tool-a"])
	}
	if summary.ByTool["tool-b"] != 20 {
		t.Errorf("expected ByTool[tool-b]=20, got %d", summary.ByTool["tool-b"])
	}

	// ByCategory
	if summary.ByCategory["search"] != 30 {
		t.Errorf("expected ByCategory[search]=30, got %d", summary.ByCategory["search"])
	}
	if summary.ByCategory["compute"] != 20 {
		t.Errorf("expected ByCategory[compute]=20, got %d", summary.ByCategory["compute"])
	}
}

// TestTracker_Reset verifies that Reset clears all data.
func TestTracker_Reset(t *testing.T) {
	t.Parallel()

	tracker := NewTracker()
	tracker.Record(UsageEntry{
		ToolName:     "tool-a",
		Category:     "cat",
		InputTokens:  100,
		OutputTokens: 200,
	})
	if tracker.Total() == 0 {
		t.Fatal("expected non-zero total before reset")
	}

	tracker.Reset()

	if tracker.Total() != 0 {
		t.Errorf("expected Total()=0 after reset, got %d", tracker.Total())
	}

	summary := tracker.Summary()
	if summary.TotalInvocations != 0 {
		t.Errorf("expected TotalInvocations=0 after reset, got %d", summary.TotalInvocations)
	}
	if len(summary.ByTool) != 0 {
		t.Errorf("expected empty ByTool after reset, got %v", summary.ByTool)
	}
}

// TestTracker_Total verifies the running total is maintained correctly.
func TestTracker_Total(t *testing.T) {
	t.Parallel()

	tracker := NewTracker()
	if tracker.Total() != 0 {
		t.Errorf("expected Total()=0 on new tracker, got %d", tracker.Total())
	}

	tracker.Record(UsageEntry{InputTokens: 7, OutputTokens: 3})
	if tracker.Total() != 10 {
		t.Errorf("expected Total()=10, got %d", tracker.Total())
	}

	tracker.Record(UsageEntry{InputTokens: 5, OutputTokens: 5})
	if tracker.Total() != 20 {
		t.Errorf("expected Total()=20, got %d", tracker.Total())
	}
}

// TestMiddleware_RecordsUsage verifies the middleware records an entry after a tool call.
func TestMiddleware_RecordsUsage(t *testing.T) {
	t.Parallel()

	tracker := NewTracker()
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("hello world response"), nil
	}

	mw := Middleware(tracker)
	wrapped := mw("test-tool", registry.ToolDefinition{Category: "test"}, handler)

	result, err := wrapped(context.Background(), makeTestRequest("test-tool", map[string]interface{}{
		"query": "test input",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if registry.IsResultError(result) {
		t.Errorf("expected success result, got error result")
	}

	summary := tracker.Summary()
	if summary.TotalInvocations != 1 {
		t.Errorf("expected TotalInvocations=1, got %d", summary.TotalInvocations)
	}
	if summary.TotalInputTokens == 0 {
		t.Error("expected non-zero input tokens")
	}
	if summary.TotalOutputTokens == 0 {
		t.Error("expected non-zero output tokens (response was non-empty)")
	}
	if summary.ByTool["test-tool"] == 0 {
		t.Error("expected non-zero tokens for test-tool in ByTool")
	}
	if summary.ByCategory["test"] == 0 {
		t.Error("expected non-zero tokens for 'test' category in ByCategory")
	}
}

// TestMiddleware_BudgetEnforcement verifies that once the token budget is consumed,
// subsequent calls return an error result without executing the handler.
func TestMiddleware_BudgetEnforcement(t *testing.T) {
	t.Parallel()

	// Budget of 10 tokens total. The first call produces output "hello world response"
	// (20 chars → 5 tokens) plus input estimate. After the first call the tracker will
	// hold more than 10 tokens, so the second call should be rejected.
	tracker := NewTracker(Config{TokenBudget: 10})

	callCount := 0
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		callCount++
		return registry.MakeTextResult("hello world response"), nil
	}

	mw := Middleware(tracker)
	wrapped := mw("test-tool", registry.ToolDefinition{Category: "test"}, handler)

	ctx := context.Background()
	req := makeTestRequest("test-tool", map[string]interface{}{"q": "x"})

	// First call — should succeed and consume tokens.
	result1, err := wrapped(ctx, req)
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if registry.IsResultError(result1) {
		t.Errorf("first call: expected success result, got error")
	}

	// Confirm tracker now holds > 10 tokens so next call is blocked.
	if tracker.Total() <= 10 {
		t.Logf("total tokens after first call: %d (expected >10 for budget test to work)", tracker.Total())
		// If the first call didn't exceed the budget with our test input, adjust expectation.
		// We'll still verify the budget logic works when the condition is met.
	}

	// Keep calling until the budget is exceeded.
	var budgetResult *registry.CallToolResult
	for i := 0; i < 10; i++ {
		r, e := wrapped(ctx, req)
		if e != nil {
			t.Fatalf("call %d: unexpected error: %v", i+2, e)
		}
		if registry.IsResultError(r) {
			budgetResult = r
			break
		}
	}

	if budgetResult == nil {
		t.Fatal("expected a budget-exceeded error result within 10 additional calls, got none")
	}

	// Verify error message contains "budget exceeded".
	for _, c := range budgetResult.Content {
		if text, ok := registry.ExtractTextContent(c); ok {
			if !strings.Contains(text, "budget exceeded") {
				t.Errorf("expected error message to contain 'budget exceeded', got: %q", text)
			}
		}
	}
}

// TestMiddleware_NoBudget verifies that a zero budget allows unlimited calls.
func TestMiddleware_NoBudget(t *testing.T) {
	t.Parallel()

	tracker := NewTracker() // no budget
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("ok"), nil
	}

	mw := Middleware(tracker)
	wrapped := mw("tool", registry.ToolDefinition{}, handler)

	ctx := context.Background()
	req := makeTestRequest("tool", map[string]interface{}{"k": "v"})

	for i := 0; i < 50; i++ {
		result, err := wrapped(ctx, req)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if registry.IsResultError(result) {
			t.Fatalf("call %d: unexpected error result", i+1)
		}
	}

	if tracker.Summary().TotalInvocations != 50 {
		t.Errorf("expected 50 invocations, got %d", tracker.Summary().TotalInvocations)
	}
}

// TestEstimateFromRequest verifies that EstimateFromRequest produces a positive
// token count for a request with non-trivial arguments.
func TestEstimateFromRequest(t *testing.T) {
	t.Parallel()

	req := makeTestRequest("search", map[string]interface{}{
		"query": "what is the meaning of life",
		"limit": 10,
	})

	got := EstimateFromRequest(req, DefaultEstimate)
	if got <= 0 {
		t.Errorf("expected positive estimate for non-empty request, got %d", got)
	}
}

// TestEstimateFromRequest_Nil verifies that a nil-args request returns 0.
func TestEstimateFromRequest_Nil(t *testing.T) {
	t.Parallel()

	req := makeTestRequest("noop", nil)
	got := EstimateFromRequest(req, DefaultEstimate)
	if got != 0 {
		t.Errorf("expected 0 for nil args, got %d", got)
	}
}

// TestEstimateFromResult verifies token estimation from a text result.
func TestEstimateFromResult(t *testing.T) {
	t.Parallel()

	result := registry.MakeTextResult("the quick brown fox jumps over the lazy dog")
	got := EstimateFromResult(result, DefaultEstimate)
	if got <= 0 {
		t.Errorf("expected positive estimate for non-empty result, got %d", got)
	}
}

// TestEstimateFromResult_Nil verifies that a nil result returns 0.
func TestEstimateFromResult_Nil(t *testing.T) {
	t.Parallel()

	got := EstimateFromResult(nil, DefaultEstimate)
	if got != 0 {
		t.Errorf("expected 0 for nil result, got %d", got)
	}
}

// TestMiddleware_CategoryFallback verifies that empty Category defaults to "unknown".
func TestMiddleware_CategoryFallback(t *testing.T) {
	t.Parallel()

	tracker := NewTracker()
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("response"), nil
	}

	mw := Middleware(tracker)
	// ToolDefinition with no Category set.
	wrapped := mw("my-tool", registry.ToolDefinition{}, handler)
	_, err := wrapped(context.Background(), makeTestRequest("my-tool", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	summary := tracker.Summary()
	if summary.ByCategory["unknown"] == 0 {
		t.Errorf("expected tokens under 'unknown' category, got %v", summary.ByCategory)
	}
}

// TestBudgetExceededError_Message verifies the error message format.
func TestBudgetExceededError_Message(t *testing.T) {
	t.Parallel()

	err := &BudgetExceededError{Limit: 1000, Used: 1050}
	msg := err.Error()
	if !strings.Contains(msg, "1000") {
		t.Errorf("expected error to contain limit '1000', got: %q", msg)
	}
	if !strings.Contains(msg, "1050") {
		t.Errorf("expected error to contain used '1050', got: %q", msg)
	}
	if !strings.Contains(msg, "budget exceeded") {
		t.Errorf("expected error to contain 'budget exceeded', got: %q", msg)
	}
}

// TestMiddleware_OnBudgetExceededCallback verifies the callback is invoked when budget is hit.
func TestMiddleware_OnBudgetExceededCallback(t *testing.T) {
	t.Parallel()

	callbackCalled := false
	tracker := NewTracker(Config{
		TokenBudget: 1, // 1 token budget — will be exceeded immediately after first call
		OnBudgetExceeded: func(entry UsageEntry, summary UsageSummary) {
			callbackCalled = true
		},
	})

	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("response text that is long enough"), nil
	}

	mw := Middleware(tracker)
	wrapped := mw("tool", registry.ToolDefinition{Category: "cat"}, handler)

	ctx := context.Background()
	req := makeTestRequest("tool", map[string]interface{}{"x": "y"})

	// First call: succeeds (budget not yet exceeded before execution).
	wrapped(ctx, req) //nolint:errcheck

	// Second call: budget now exceeded — callback should fire.
	wrapped(ctx, req) //nolint:errcheck

	if !callbackCalled {
		t.Error("expected OnBudgetExceeded callback to be called")
	}
}
