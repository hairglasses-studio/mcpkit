//go:build !official_sdk

package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestTaskMiddleware_NoTaskMeta(t *testing.T) {
	mgr := NewManager()
	mw := TaskMiddleware(mgr)

	called := false
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
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
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
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
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		defer close(handlerDone)
		return registry.MakeTextResult("async result"), nil
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

	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
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
	if !registry.IsResultError(result) {
		t.Error("expected error result for required task support without task params")
	}
}
