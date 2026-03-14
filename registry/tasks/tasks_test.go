package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	if id1 == id2 {
		t.Error("GenerateID should produce unique IDs")
	}
	if len(id1) < 10 {
		t.Errorf("GenerateID too short: %s", id1)
	}
}

func TestManagerCreateAndGet(t *testing.T) {
	mgr := NewManager()
	entry := mgr.Create(time.Minute)
	if entry == nil {
		t.Fatal("Create returned nil")
	}
	if entry.Task.TaskId == "" {
		t.Error("task ID should not be empty")
	}
	if entry.Task.Status != mcp.TaskStatusWorking {
		t.Errorf("initial status = %s, want working", entry.Task.Status)
	}

	got := mgr.Get(entry.Task.TaskId)
	if got == nil {
		t.Fatal("Get returned nil for existing task")
	}
	if got.Task.TaskId != entry.Task.TaskId {
		t.Error("Get returned wrong task")
	}
}

func TestManagerGetNotFound(t *testing.T) {
	mgr := NewManager()
	if mgr.Get("nonexistent") != nil {
		t.Error("Get should return nil for nonexistent task")
	}
}

func TestManagerList(t *testing.T) {
	mgr := NewManager()
	mgr.Create(time.Minute)
	mgr.Create(time.Minute)

	tasks := mgr.List()
	if len(tasks) != 2 {
		t.Errorf("List returned %d tasks, want 2", len(tasks))
	}
}

func TestManagerCancel(t *testing.T) {
	mgr := NewManager()
	entry := mgr.Create(time.Minute)

	if err := mgr.Cancel(entry.Task.TaskId); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	snap := entry.Snapshot()
	if snap.Status != mcp.TaskStatusCancelled {
		t.Errorf("status after cancel = %s, want cancelled", snap.Status)
	}
}

func TestManagerCancelNotFound(t *testing.T) {
	mgr := NewManager()
	if err := mgr.Cancel("nonexistent"); err == nil {
		t.Error("Cancel should fail for nonexistent task")
	}
}

func TestManagerCancelTerminal(t *testing.T) {
	mgr := NewManager()
	entry := mgr.Create(time.Minute)
	entry.Update(mcp.TaskStatusCompleted, "done")

	if err := mgr.Cancel(entry.Task.TaskId); err == nil {
		t.Error("Cancel should fail for terminal task")
	}
}

func TestManagerCleanup(t *testing.T) {
	mgr := NewManager()
	mgr.Create(time.Millisecond) // will expire quickly
	time.Sleep(5 * time.Millisecond)

	mgr.Cleanup()
	if mgr.Count() != 0 {
		t.Errorf("Count after cleanup = %d, want 0", mgr.Count())
	}
}

func TestManagerCount(t *testing.T) {
	mgr := NewManager()
	if mgr.Count() != 0 {
		t.Errorf("initial Count = %d, want 0", mgr.Count())
	}
	mgr.Create(time.Minute)
	if mgr.Count() != 1 {
		t.Errorf("Count after create = %d, want 1", mgr.Count())
	}
}

func TestTaskEntryUpdate(t *testing.T) {
	entry := &TaskEntry{
		Task: mcp.NewTask("test-1"),
	}
	entry.Update(mcp.TaskStatusCompleted, "all done")
	snap := entry.Snapshot()
	if snap.Status != mcp.TaskStatusCompleted {
		t.Errorf("status = %s, want completed", snap.Status)
	}
	if snap.StatusMessage != "all done" {
		t.Errorf("message = %q, want %q", snap.StatusMessage, "all done")
	}
}

func TestTaskMiddleware_NoTaskMeta(t *testing.T) {
	mgr := NewManager()
	mw := TaskMiddleware(mgr)

	called := false
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:      "test_tool",
			Execution: &mcp.ToolExecution{TaskSupport: mcp.TaskSupportOptional},
		},
	}

	wrapped := mw("test_tool", td, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler should be called synchronously when no task meta")
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestTaskMiddleware_ForbiddenSkips(t *testing.T) {
	mgr := NewManager()
	mw := TaskMiddleware(mgr)

	called := false
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "test_tool"},
	}

	wrapped := mw("test_tool", td, handler)
	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	if !called {
		t.Error("handler should be called directly for forbidden task support")
	}
}

func TestTaskMiddleware_WithTaskParams(t *testing.T) {
	mgr := NewManager()
	mw := TaskMiddleware(mgr)

	handlerDone := make(chan struct{})
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		defer close(handlerDone)
		return mcp.NewToolResultText("async result"), nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:      "async_tool",
			Execution: &mcp.ToolExecution{TaskSupport: mcp.TaskSupportOptional},
		},
	}

	wrapped := mw("async_tool", td, handler)

	ttlMs := int64(60000)
	req := mcp.CallToolRequest{}
	req.Params.Task = &mcp.TaskParams{TTL: &ttlMs}

	result, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}

	// Wait for async handler to complete
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not complete in time")
	}

	// Verify a task was created
	if mgr.Count() < 1 {
		t.Error("expected at least one task after async call")
	}
}

func TestTaskMiddleware_RequiredWithoutTaskParams(t *testing.T) {
	mgr := NewManager()
	mw := TaskMiddleware(mgr)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		t.Error("handler should not be called")
		return nil, nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:      "required_tool",
			Execution: &mcp.ToolExecution{TaskSupport: mcp.TaskSupportRequired},
		},
	}

	wrapped := mw("required_tool", td, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result for required task support without task params")
	}
}

func TestGetTaskEntry_NotInContext(t *testing.T) {
	if GetTaskEntry(context.Background()) != nil {
		t.Error("GetTaskEntry should return nil when not in context")
	}
}
