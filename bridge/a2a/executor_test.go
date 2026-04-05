package a2a

import (
	"context"
	"fmt"
	"testing"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestBridgeExecutor_HappyPath(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "test",
		tools: []registry.ToolDefinition{
			{
				Tool:    registry.Tool{Name: "greet", Description: "Say hello"},
				Handler: greetHandler,
			},
		},
	})

	exec := NewBridgeExecutor(reg, ExecutorConfig{})

	execCtx := makeExecCtx("greet", map[string]any{"name": "world"})
	events := collectEvents(t, exec.Execute(context.Background(), execCtx))

	// Expected sequence: Task(submitted) -> StatusUpdate(working) -> ArtifactUpdate -> StatusUpdate(completed)
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// First event: submitted task.
	if task, ok := events[0].(*a2atypes.Task); ok {
		if task.Status.State != a2atypes.TaskStateSubmitted {
			t.Errorf("expected submitted state, got %s", task.Status.State)
		}
	} else {
		t.Errorf("expected *a2a.Task as first event, got %T", events[0])
	}

	// Second event: working status.
	assertStatusUpdate(t, events[1], a2atypes.TaskStateWorking)

	// Third event: artifact with the result.
	artEvent, ok := events[2].(*a2atypes.TaskArtifactUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskArtifactUpdateEvent, got %T", events[2])
	}
	if len(artEvent.Artifact.Parts) == 0 {
		t.Fatal("expected artifact to have parts")
	}
	text := artEvent.Artifact.Parts[0].Text()
	if text != "hello world" {
		t.Errorf("expected artifact text %q, got %q", "hello world", text)
	}

	// Fourth event: completed status.
	assertStatusUpdate(t, events[3], a2atypes.TaskStateCompleted)
}

func TestBridgeExecutor_ToolError(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "test",
		tools: []registry.ToolDefinition{
			{
				Tool: registry.Tool{Name: "fail_tool", Description: "Always fails"},
				Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
					return registry.MakeErrorResult("something went wrong"), nil
				},
			},
		},
	})

	exec := NewBridgeExecutor(reg, ExecutorConfig{})
	execCtx := makeExecCtx("fail_tool", nil)
	events := collectEvents(t, exec.Execute(context.Background(), execCtx))

	// Expected: submitted -> working -> failed
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// The last event should be a failed status.
	lastEvent := events[len(events)-1]
	statusUpdate, ok := lastEvent.(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", lastEvent)
	}
	if statusUpdate.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("expected failed state, got %s", statusUpdate.Status.State)
	}
	if statusUpdate.Status.Message == nil {
		t.Fatal("expected error message in failed status")
	}
	errText := statusUpdate.Status.Message.Parts[0].Text()
	if errText != "something went wrong" {
		t.Errorf("expected error text %q, got %q", "something went wrong", errText)
	}
}

func TestBridgeExecutor_ToolReturnsError(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "test",
		tools: []registry.ToolDefinition{
			{
				Tool: registry.Tool{Name: "err_tool", Description: "Returns Go error"},
				Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
					return nil, fmt.Errorf("connection refused")
				},
			},
		},
	})

	exec := NewBridgeExecutor(reg, ExecutorConfig{})
	execCtx := makeExecCtx("err_tool", nil)
	events := collectEvents(t, exec.Execute(context.Background(), execCtx))

	// Should end with failed status due to Go error.
	lastEvent := events[len(events)-1]
	statusUpdate, ok := lastEvent.(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", lastEvent)
	}
	if statusUpdate.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("expected failed state, got %s", statusUpdate.Status.State)
	}
}

func TestBridgeExecutor_UnknownTool(t *testing.T) {
	reg := registry.NewToolRegistry()
	exec := NewBridgeExecutor(reg, ExecutorConfig{})

	execCtx := makeExecCtx("nonexistent_tool", nil)
	events := collectEvents(t, exec.Execute(context.Background(), execCtx))

	// Should have submitted + failed.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Last event should be failed with unknown tool message.
	lastEvent := events[len(events)-1]
	statusUpdate, ok := lastEvent.(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", lastEvent)
	}
	if statusUpdate.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("expected failed state, got %s", statusUpdate.Status.State)
	}
	if statusUpdate.Status.Message == nil {
		t.Fatal("expected error message")
	}
	errText := statusUpdate.Status.Message.Parts[0].Text()
	if errText != "unknown tool: nonexistent_tool" {
		t.Errorf("expected error text %q, got %q", "unknown tool: nonexistent_tool", errText)
	}
}

func TestBridgeExecutor_Cancel(t *testing.T) {
	reg := registry.NewToolRegistry()
	exec := NewBridgeExecutor(reg, ExecutorConfig{})

	execCtx := &a2asrv.ExecutorContext{
		TaskID:    a2atypes.NewTaskID(),
		ContextID: a2atypes.NewContextID(),
	}

	events := collectEvents(t, exec.Cancel(context.Background(), execCtx))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	statusUpdate, ok := events[0].(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", events[0])
	}
	if statusUpdate.Status.State != a2atypes.TaskStateCanceled {
		t.Errorf("expected canceled state, got %s", statusUpdate.Status.State)
	}
}

func TestBridgeExecutor_InvalidMessage(t *testing.T) {
	reg := registry.NewToolRegistry()
	exec := NewBridgeExecutor(reg, ExecutorConfig{})

	// Message with no DataPart containing a skill field.
	execCtx := &a2asrv.ExecutorContext{
		TaskID:    a2atypes.NewTaskID(),
		ContextID: a2atypes.NewContextID(),
		Message: &a2atypes.Message{
			ID:   a2atypes.NewMessageID(),
			Role: a2atypes.MessageRoleUser,
			Parts: []*a2atypes.Part{
				a2atypes.NewTextPart("just some text"),
			},
		},
	}

	events := collectEvents(t, exec.Execute(context.Background(), execCtx))

	// Should have submitted + failed.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	lastEvent := events[len(events)-1]
	statusUpdate, ok := lastEvent.(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", lastEvent)
	}
	if statusUpdate.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("expected failed state, got %s", statusUpdate.Status.State)
	}
}

func TestBridgeExecutor_NilMessage(t *testing.T) {
	reg := registry.NewToolRegistry()
	exec := NewBridgeExecutor(reg, ExecutorConfig{})

	execCtx := &a2asrv.ExecutorContext{
		TaskID:    a2atypes.NewTaskID(),
		ContextID: a2atypes.NewContextID(),
		Message:   nil,
	}

	events := collectEvents(t, exec.Execute(context.Background(), execCtx))

	// Should have submitted + failed.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	lastEvent := events[len(events)-1]
	statusUpdate, ok := lastEvent.(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", lastEvent)
	}
	if statusUpdate.Status.State != a2atypes.TaskStateFailed {
		t.Errorf("expected failed state, got %s", statusUpdate.Status.State)
	}
}

func TestBridgeExecutor_Middleware(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "test",
		tools: []registry.ToolDefinition{
			{
				Tool:    registry.Tool{Name: "mw_tool", Description: "Middleware test"},
				Handler: greetHandler,
			},
		},
	})

	var middlewareCalled bool
	var middlewareToolName string
	mw := func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			middlewareCalled = true
			middlewareToolName = name
			return next(ctx, req)
		}
	}

	exec := NewBridgeExecutor(reg, ExecutorConfig{
		Middleware: []registry.Middleware{mw},
	})

	execCtx := makeExecCtx("mw_tool", map[string]any{"name": "test"})
	events := collectEvents(t, exec.Execute(context.Background(), execCtx))

	if !middlewareCalled {
		t.Error("expected middleware to be called")
	}
	if middlewareToolName != "mw_tool" {
		t.Errorf("expected middleware tool name %q, got %q", "mw_tool", middlewareToolName)
	}

	// Should still complete successfully.
	lastEvent := events[len(events)-1]
	if su, ok := lastEvent.(*a2atypes.TaskStatusUpdateEvent); ok {
		if su.Status.State != a2atypes.TaskStateCompleted {
			t.Errorf("expected completed state, got %s", su.Status.State)
		}
	} else {
		t.Errorf("expected status update as last event, got %T", lastEvent)
	}
}

// --- test helpers ---

func greetHandler(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
	args := registry.ExtractArguments(req)
	name, _ := args["name"].(string)
	if name == "" {
		name = "anonymous"
	}
	return registry.MakeTextResult("hello " + name), nil
}

// makeExecCtx builds an ExecutorContext with a DataPart message referencing
// the given skill and arguments.
func makeExecCtx(skill string, args map[string]any) *a2asrv.ExecutorContext {
	data := map[string]any{
		"skill":     skill,
		"arguments": args,
	}
	msg := &a2atypes.Message{
		ID:   a2atypes.NewMessageID(),
		Role: a2atypes.MessageRoleUser,
		Parts: []*a2atypes.Part{
			a2atypes.NewDataPart(data),
		},
	}
	return &a2asrv.ExecutorContext{
		TaskID:    a2atypes.NewTaskID(),
		ContextID: a2atypes.NewContextID(),
		Message:   msg,
	}
}

// collectEvents drains an event iterator into a slice.
func collectEvents(t *testing.T, seq func(func(a2atypes.Event, error) bool)) []a2atypes.Event {
	t.Helper()
	var events []a2atypes.Event
	for event, err := range seq {
		if err != nil {
			t.Fatalf("unexpected error from event iterator: %v", err)
		}
		events = append(events, event)
	}
	return events
}

// assertStatusUpdate checks that an event is a TaskStatusUpdateEvent with the
// expected state.
func assertStatusUpdate(t *testing.T, event a2atypes.Event, expectedState a2atypes.TaskState) {
	t.Helper()
	su, ok := event.(*a2atypes.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("expected *a2a.TaskStatusUpdateEvent, got %T", event)
	}
	if su.Status.State != expectedState {
		t.Errorf("expected state %s, got %s", expectedState, su.Status.State)
	}
}
