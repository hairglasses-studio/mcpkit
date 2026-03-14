package tasks

import (
	"context"
	"testing"
	"time"

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
	if entry.Task.Status != registry.TaskStatusWorking {
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
	if snap.Status != registry.TaskStatusCancelled {
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
	entry.Update(registry.TaskStatusCompleted, "done")

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
		Task: TaskInfo{TaskId: "test-1", Status: registry.TaskStatusWorking},
	}
	entry.Update(registry.TaskStatusCompleted, "all done")
	snap := entry.Snapshot()
	if snap.Status != registry.TaskStatusCompleted {
		t.Errorf("status = %s, want completed", snap.Status)
	}
	if snap.StatusMessage != "all done" {
		t.Errorf("message = %q, want %q", snap.StatusMessage, "all done")
	}
}

func TestTaskInfoIsTerminal(t *testing.T) {
	tests := []struct {
		status   registry.TaskStatus
		terminal bool
	}{
		{registry.TaskStatusWorking, false},
		{registry.TaskStatusInputRequired, false},
		{registry.TaskStatusCompleted, true},
		{registry.TaskStatusFailed, true},
		{registry.TaskStatusCancelled, true},
	}
	for _, tt := range tests {
		info := TaskInfo{Status: tt.status}
		if info.IsTerminal() != tt.terminal {
			t.Errorf("IsTerminal(%s) = %v, want %v", tt.status, info.IsTerminal(), tt.terminal)
		}
	}
}

func TestGetTaskEntry_NotInContext(t *testing.T) {
	if GetTaskEntry(context.Background()) != nil {
		t.Error("GetTaskEntry should return nil when not in context")
	}
}
