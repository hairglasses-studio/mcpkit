//go:build !official_sdk

package hitools

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- ApprovalStore tests ---

func TestInMemoryApprovalStore_SubmitAndGet(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	req := ApprovalRequest{
		ID:        "apr-001",
		ToolName:  "deploy_production",
		Action:    "Deploy v2.1.0 to production",
		Context:   "All tests passing, staging verified",
		Urgency:   ApprovalUrgencyHigh,
		CreatedAt: time.Now(),
	}

	if err := store.Submit(ctx, req); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got, err := store.Get(ctx, "apr-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil for existing request")
	}
	if got.ToolName != "deploy_production" {
		t.Errorf("ToolName = %q, want %q", got.ToolName, "deploy_production")
	}
	if got.Urgency != ApprovalUrgencyHigh {
		t.Errorf("Urgency = %q, want %q", got.Urgency, ApprovalUrgencyHigh)
	}
}

func TestInMemoryApprovalStore_GetNotFound(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	got, err := store.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("Get should return nil for nonexistent request")
	}
}

func TestInMemoryApprovalStore_SubmitDuplicate(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	req := ApprovalRequest{ID: "apr-dup", ToolName: "test"}
	if err := store.Submit(ctx, req); err != nil {
		t.Fatalf("first Submit: %v", err)
	}

	err := store.Submit(ctx, req)
	if err == nil {
		t.Error("expected error for duplicate submission")
	}
}

func TestInMemoryApprovalStore_RespondAndGetResponse(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	req := ApprovalRequest{ID: "apr-002", ToolName: "test"}
	store.Submit(ctx, req)

	resp := ApprovalResponse{
		RequestID: "apr-002",
		Decision:  Approved,
		Comment:   "Looks good",
		Timestamp: time.Now(),
	}
	if err := store.Respond(ctx, resp); err != nil {
		t.Fatalf("Respond: %v", err)
	}

	got, err := store.GetResponse(ctx, "apr-002")
	if err != nil {
		t.Fatalf("GetResponse: %v", err)
	}
	if got == nil {
		t.Fatal("GetResponse returned nil for responded request")
	}
	if got.Decision != Approved {
		t.Errorf("Decision = %q, want %q", got.Decision, Approved)
	}
	if got.Comment != "Looks good" {
		t.Errorf("Comment = %q, want %q", got.Comment, "Looks good")
	}
}

func TestInMemoryApprovalStore_RespondNotFound(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	resp := ApprovalResponse{RequestID: "nonexistent", Decision: Denied}
	err := store.Respond(ctx, resp)
	if err == nil {
		t.Error("expected error for responding to nonexistent request")
	}
}

func TestInMemoryApprovalStore_RespondAlreadyResolved(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	req := ApprovalRequest{ID: "apr-003", ToolName: "test"}
	store.Submit(ctx, req)
	store.Respond(ctx, ApprovalResponse{RequestID: "apr-003", Decision: Approved})

	err := store.Respond(ctx, ApprovalResponse{RequestID: "apr-003", Decision: Denied})
	if err == nil {
		t.Error("expected error for double-responding to same request")
	}
}

func TestInMemoryApprovalStore_GetResponseNotFound(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	got, err := store.GetResponse(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetResponse: %v", err)
	}
	if got != nil {
		t.Error("GetResponse should return nil for nonexistent response")
	}
}

func TestInMemoryApprovalStore_Pending(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	// Submit 3 requests, resolve 1.
	store.Submit(ctx, ApprovalRequest{ID: "a", ToolName: "t1", CreatedAt: time.Now()})
	store.Submit(ctx, ApprovalRequest{ID: "b", ToolName: "t2", CreatedAt: time.Now()})
	store.Submit(ctx, ApprovalRequest{ID: "c", ToolName: "t3", CreatedAt: time.Now()})
	store.Respond(ctx, ApprovalResponse{RequestID: "b", Decision: Approved})

	pending, err := store.Pending(ctx)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("Pending = %d items, want 2", len(pending))
	}

	ids := make(map[string]bool)
	for _, p := range pending {
		ids[p.ID] = true
	}
	if !ids["a"] || !ids["c"] {
		t.Errorf("Pending IDs = %v, want a and c", ids)
	}
}

func TestInMemoryApprovalStore_PendingEmpty(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	pending, err := store.Pending(ctx)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("Pending = %d items, want 0", len(pending))
	}
}

func TestInMemoryApprovalStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryApprovalStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("apr-concurrent-%d", n)
			store.Submit(ctx, ApprovalRequest{ID: id, ToolName: "test"})
			store.Get(ctx, id)
			store.Pending(ctx)
			store.Respond(ctx, ApprovalResponse{RequestID: id, Decision: Approved})
			store.GetResponse(ctx, id)
		}(i)
	}
	wg.Wait()
}

// --- Decision type tests ---

func TestDecisionConstants(t *testing.T) {
	if Approved != "approved" {
		t.Errorf("Approved = %q, want %q", Approved, "approved")
	}
	if Denied != "denied" {
		t.Errorf("Denied = %q, want %q", Denied, "denied")
	}
	if Modified != "modified" {
		t.Errorf("Modified = %q, want %q", Modified, "modified")
	}
}

// --- ApprovalUrgency tests ---

func TestApprovalUrgencyConstants(t *testing.T) {
	tests := []struct {
		got  ApprovalUrgency
		want string
	}{
		{ApprovalUrgencyLow, "low"},
		{ApprovalUrgencyNormal, "normal"},
		{ApprovalUrgencyHigh, "high"},
		{ApprovalUrgencyCritical, "critical"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("ApprovalUrgency = %q, want %q", tt.got, tt.want)
		}
	}
}

// --- generateApprovalID tests ---

func TestGenerateApprovalID(t *testing.T) {
	id, err := generateApprovalID()
	if err != nil {
		t.Fatalf("generateApprovalID: %v", err)
	}
	if id == "" {
		t.Error("generated ID should not be empty")
	}
	if len(id) < 10 {
		t.Errorf("generated ID %q seems too short", id)
	}

	// IDs should be unique.
	id2, _ := generateApprovalID()
	if id == id2 {
		t.Error("two generated IDs should not be equal")
	}
}

// --- formatToolContext tests ---

func TestFormatToolContext_WithArgs(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        "deploy",
			Description: "Deploy to production",
		},
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"env": "production", "version": "v2.1.0"}

	ctx := formatToolContext(td, req)
	if ctx == "" {
		t.Error("context should not be empty")
	}
	// Should contain tool name and description.
	if !contains(ctx, "deploy") {
		t.Error("context should contain tool name")
	}
	if !contains(ctx, "Deploy to production") {
		t.Error("context should contain description")
	}
}

func TestFormatToolContext_NoArgs(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        "status",
			Description: "Check deployment status",
		},
	}
	req := mcp.CallToolRequest{}

	ctx := formatToolContext(td, req)
	if !contains(ctx, "status") {
		t.Error("context should contain tool name")
	}
	// Should not mention "Arguments" when there are none.
	if contains(ctx, "Arguments") {
		t.Error("context should not mention arguments when there are none")
	}
}

func TestFormatToolContext_NoDescription(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "test_tool"},
	}
	req := mcp.CallToolRequest{}

	ctx := formatToolContext(td, req)
	// Falls back to tool name when no description.
	if !contains(ctx, "test_tool") {
		t.Error("context should contain tool name as fallback description")
	}
}

// --- ApprovalMiddleware tests ---

func makeToolReq() registry.CallToolRequest {
	return mcp.CallToolRequest{}
}

func okToolHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("executed"), nil
}

func TestApprovalMiddleware_NoApprovalNeeded(t *testing.T) {
	store := NewInMemoryApprovalStore()
	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(name string, _ registry.ToolDefinition) bool {
			return false // nothing needs approval
		},
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "safe_tool"}}
	handler := mw("safe_tool", td, okToolHandler)

	result, err := handler(context.Background(), makeToolReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success when no approval needed")
	}
}

func TestApprovalMiddleware_NilShouldApprove(t *testing.T) {
	store := NewInMemoryApprovalStore()
	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store:         store,
		ShouldApprove: nil, // no filter means nothing needs approval
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "tool"}}
	handler := mw("tool", td, okToolHandler)

	result, err := handler(context.Background(), makeToolReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success when ShouldApprove is nil")
	}
}

func TestApprovalMiddleware_Approved(t *testing.T) {
	store := NewInMemoryApprovalStore()

	var capturedReq ApprovalRequest
	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(name string, _ registry.ToolDefinition) bool {
			return name == "dangerous_tool"
		},
		DefaultUrgency: ApprovalUrgencyHigh,
		Timeout:        2 * time.Second,
		OnRequest: func(_ context.Context, req ApprovalRequest) {
			capturedReq = req
		},
	})

	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "dangerous_tool", Description: "Does dangerous things"},
	}
	handler := mw("dangerous_tool", td, okToolHandler)

	// Simulate async approval: approve after 100ms.
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Find the pending request and approve it.
		pending, _ := store.Pending(context.Background())
		if len(pending) > 0 {
			store.Respond(context.Background(), ApprovalResponse{
				RequestID: pending[0].ID,
				Decision:  Approved,
				Comment:   "LGTM",
				Timestamp: time.Now(),
			})
		}
	}()

	result, err := handler(context.Background(), makeToolReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success after approval")
	}

	// Verify the OnRequest callback was called.
	if capturedReq.ToolName != "dangerous_tool" {
		t.Errorf("OnRequest ToolName = %q, want %q", capturedReq.ToolName, "dangerous_tool")
	}
	if capturedReq.Urgency != ApprovalUrgencyHigh {
		t.Errorf("OnRequest Urgency = %q, want %q", capturedReq.Urgency, ApprovalUrgencyHigh)
	}
}

func TestApprovalMiddleware_Denied(t *testing.T) {
	store := NewInMemoryApprovalStore()

	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(_ string, _ registry.ToolDefinition) bool {
			return true
		},
		Timeout: 2 * time.Second,
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "tool"}}
	handler := mw("tool", td, okToolHandler)

	// Simulate async denial.
	go func() {
		time.Sleep(100 * time.Millisecond)
		pending, _ := store.Pending(context.Background())
		if len(pending) > 0 {
			store.Respond(context.Background(), ApprovalResponse{
				RequestID: pending[0].ID,
				Decision:  Denied,
				Comment:   "Not now",
				Timestamp: time.Now(),
			})
		}
	}()

	result, err := handler(context.Background(), makeToolReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result after denial")
	}
}

func TestApprovalMiddleware_Modified(t *testing.T) {
	store := NewInMemoryApprovalStore()

	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(_ string, _ registry.ToolDefinition) bool {
			return true
		},
		Timeout: 2 * time.Second,
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "tool"}}
	handler := mw("tool", td, okToolHandler)

	// Modified decision should still proceed.
	go func() {
		time.Sleep(100 * time.Millisecond)
		pending, _ := store.Pending(context.Background())
		if len(pending) > 0 {
			store.Respond(context.Background(), ApprovalResponse{
				RequestID: pending[0].ID,
				Decision:  Modified,
				Value:     "staging",
				Comment:   "Deploy to staging first",
				Timestamp: time.Now(),
			})
		}
	}()

	result, err := handler(context.Background(), makeToolReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success after modified approval")
	}
}

func TestApprovalMiddleware_Timeout(t *testing.T) {
	store := NewInMemoryApprovalStore()

	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(_ string, _ registry.ToolDefinition) bool {
			return true
		},
		Timeout: 200 * time.Millisecond, // short timeout
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "tool"}}
	handler := mw("tool", td, okToolHandler)

	// No response — should timeout.
	result, err := handler(context.Background(), makeToolReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result after timeout")
	}
}

func TestApprovalMiddleware_ContextCancellation(t *testing.T) {
	store := NewInMemoryApprovalStore()

	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(_ string, _ registry.ToolDefinition) bool {
			return true
		},
		// No timeout — rely on context cancellation.
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "tool"}}
	handler := mw("tool", td, okToolHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, err := handler(ctx, makeToolReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result after context cancellation")
	}
}

func TestApprovalMiddleware_OnResponseCallback(t *testing.T) {
	store := NewInMemoryApprovalStore()

	var gotResp ApprovalResponse
	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(_ string, _ registry.ToolDefinition) bool {
			return true
		},
		Timeout: 2 * time.Second,
		OnResponse: func(_ context.Context, _ ApprovalRequest, resp ApprovalResponse) {
			gotResp = resp
		},
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "tool"}}
	handler := mw("tool", td, okToolHandler)

	go func() {
		time.Sleep(100 * time.Millisecond)
		pending, _ := store.Pending(context.Background())
		if len(pending) > 0 {
			store.Respond(context.Background(), ApprovalResponse{
				RequestID: pending[0].ID,
				Decision:  Approved,
				Comment:   "Approved via test",
				Timestamp: time.Now(),
			})
		}
	}()

	handler(context.Background(), makeToolReq())

	if gotResp.Decision != Approved {
		t.Errorf("OnResponse Decision = %q, want %q", gotResp.Decision, Approved)
	}
	if gotResp.Comment != "Approved via test" {
		t.Errorf("OnResponse Comment = %q, want %q", gotResp.Comment, "Approved via test")
	}
}

func TestApprovalMiddleware_DefaultUrgency(t *testing.T) {
	store := NewInMemoryApprovalStore()

	var capturedUrgency ApprovalUrgency
	mw := ApprovalMiddleware(ApprovalMiddlewareConfig{
		Store: store,
		ShouldApprove: func(_ string, _ registry.ToolDefinition) bool {
			return true
		},
		// No DefaultUrgency set — should default to "normal".
		Timeout: 2 * time.Second,
		OnRequest: func(_ context.Context, req ApprovalRequest) {
			capturedUrgency = req.Urgency
		},
	})

	td := registry.ToolDefinition{Tool: mcp.Tool{Name: "tool"}}
	handler := mw("tool", td, okToolHandler)

	go func() {
		time.Sleep(100 * time.Millisecond)
		pending, _ := store.Pending(context.Background())
		if len(pending) > 0 {
			store.Respond(context.Background(), ApprovalResponse{
				RequestID: pending[0].ID,
				Decision:  Approved,
				Timestamp: time.Now(),
			})
		}
	}()

	handler(context.Background(), makeToolReq())

	if capturedUrgency != ApprovalUrgencyNormal {
		t.Errorf("default urgency = %q, want %q", capturedUrgency, ApprovalUrgencyNormal)
	}
}

// --- ThreadEventData tests ---

func TestThreadEventData_Fields(t *testing.T) {
	data := ThreadEventData{
		ApprovalID: "apr-001",
		ToolName:   "deploy",
		Action:     "Deploy to production",
		Urgency:    ApprovalUrgencyHigh,
		Decision:   Approved,
		Comment:    "Ship it",
	}

	if data.ApprovalID != "apr-001" {
		t.Errorf("ApprovalID = %q", data.ApprovalID)
	}
	if data.Decision != Approved {
		t.Errorf("Decision = %q", data.Decision)
	}
}

// --- ApprovalOption tests ---

func TestApprovalOption_Struct(t *testing.T) {
	opts := []ApprovalOption{
		{Label: "Approve", Value: "approve", Description: "Proceed with deployment", IsDefault: true},
		{Label: "Deny", Value: "deny", Description: "Cancel the deployment"},
		{Label: "Defer", Value: "defer", Description: "Schedule for later"},
	}

	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	if !opts[0].IsDefault {
		t.Error("first option should be default")
	}
	if opts[1].IsDefault {
		t.Error("second option should not be default")
	}
}

// --- helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// need fmt for Sprintf in concurrent test
var _ = fmt.Sprintf
