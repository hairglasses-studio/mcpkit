package session

import (
	"context"
	"sync"
	"testing"
	"time"
)

func newTestCheckpointManager() (*CheckpointManager, *ThreadStore) {
	store := NewThreadStore()
	cm := NewCheckpointManager(store, FormatJSON)
	return cm, store
}

func newTestThread(t *testing.T) *Thread {
	t.Helper()
	th, err := NewThread()
	if err != nil {
		t.Fatalf("NewThread: %v", err)
	}
	// Add some initial events to simulate a running agent.
	th.Append(Event{
		ID:        "tool-call-1",
		Type:      EventToolCall,
		Timestamp: time.Now(),
		Data:      "list_files",
		Metadata:  map[string]string{"tool": "list_files"},
	})
	th.Append(Event{
		ID:        "tool-result-1",
		Type:      EventToolResult,
		Timestamp: time.Now(),
		Data:      "file1.go, file2.go",
	})
	return th
}

func TestCheckpointManager_SaveResume(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	originalID := th.ID
	originalLen := th.Len()

	// Save.
	checkpointID, err := cm.Save(ctx, th)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if checkpointID != originalID {
		t.Fatalf("checkpoint ID: got %q, want %q", checkpointID, originalID)
	}

	// Resume.
	resumed, err := cm.Resume(ctx, checkpointID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if resumed.ID != originalID {
		t.Fatalf("resumed ID: got %q, want %q", resumed.ID, originalID)
	}
	if resumed.Len() != originalLen {
		t.Fatalf("resumed events: got %d, want %d", resumed.Len(), originalLen)
	}

	// Verify events are intact.
	events := resumed.Replay()
	if events[0].Type != EventToolCall {
		t.Errorf("event 0 type: got %q, want %q", events[0].Type, EventToolCall)
	}
	if events[1].Type != EventToolResult {
		t.Errorf("event 1 type: got %q, want %q", events[1].Type, EventToolResult)
	}
}

func TestCheckpointManager_SaveNil(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	_, err := cm.Save(ctx, nil)
	if err == nil {
		t.Fatal("expected error saving nil thread")
	}
}

func TestCheckpointManager_Pause(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	eventsBefore := th.Len()

	// Pause with a reason.
	checkpointID, err := cm.Pause(ctx, th, "awaiting human approval")
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if checkpointID != th.ID {
		t.Fatalf("checkpoint ID: got %q, want %q", checkpointID, th.ID)
	}

	// Verify a checkpoint event was appended.
	if th.Len() != eventsBefore+1 {
		t.Fatalf("expected %d events after pause, got %d", eventsBefore+1, th.Len())
	}

	last, ok := th.Last()
	if !ok {
		t.Fatal("expected non-empty thread after pause")
	}
	if last.Type != EventCheckpoint {
		t.Fatalf("last event type: got %q, want %q", last.Type, EventCheckpoint)
	}
	if last.Metadata[checkpointStatusKey] != string(StatusPaused) {
		t.Fatalf("checkpoint status: got %q, want %q", last.Metadata[checkpointStatusKey], StatusPaused)
	}
	if last.Metadata[checkpointReasonKey] != "awaiting human approval" {
		t.Fatalf("checkpoint reason: got %q, want %q", last.Metadata[checkpointReasonKey], "awaiting human approval")
	}

	// Verify status is paused.
	status, err := cm.Status(ctx, th.ID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != StatusPaused {
		t.Fatalf("status: got %q, want %q", status, StatusPaused)
	}
}

func TestCheckpointManager_PauseEmptyReason(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)

	_, err := cm.Pause(ctx, th, "")
	if err != nil {
		t.Fatalf("Pause with empty reason: %v", err)
	}

	last, _ := th.Last()
	if _, hasReason := last.Metadata[checkpointReasonKey]; hasReason {
		t.Fatal("expected no reason metadata when reason is empty")
	}
}

func TestCheckpointManager_PauseNil(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	_, err := cm.Pause(ctx, nil, "reason")
	if err == nil {
		t.Fatal("expected error pausing nil thread")
	}
}

func TestCheckpointManager_ResumeNotFound(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	_, err := cm.Resume(ctx, "nonexistent-thread-id")
	if err == nil {
		t.Fatal("expected error for nonexistent thread")
	}
}

func TestCheckpointManager_Status_Running(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	cm.Save(ctx, th)

	status, err := cm.Status(ctx, th.ID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != StatusRunning {
		t.Fatalf("expected running, got %q", status)
	}
}

func TestCheckpointManager_Status_Paused(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	cm.Pause(ctx, th, "tool boundary")

	status, err := cm.Status(ctx, th.ID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != StatusPaused {
		t.Fatalf("expected paused, got %q", status)
	}
}

func TestCheckpointManager_Status_Completed(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	cm.Complete(ctx, th)

	status, err := cm.Status(ctx, th.ID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != StatusCompleted {
		t.Fatalf("expected completed, got %q", status)
	}
}

func TestCheckpointManager_Status_Failed(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	cm.Fail(ctx, th, "tool execution error")

	status, err := cm.Status(ctx, th.ID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != StatusFailed {
		t.Fatalf("expected failed, got %q", status)
	}
}

func TestCheckpointManager_Status_NotFound(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	_, err := cm.Status(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent thread")
	}
}

func TestCheckpointManager_Status_Transitions(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	cm.Save(ctx, th)

	// Running -> Paused -> Running (via more events) -> Completed.
	status, _ := cm.Status(ctx, th.ID)
	if status != StatusRunning {
		t.Fatalf("initial: got %q, want running", status)
	}

	cm.Pause(ctx, th, "step 1")
	status, _ = cm.Status(ctx, th.ID)
	if status != StatusPaused {
		t.Fatalf("after pause: got %q, want paused", status)
	}

	// Resume and add more events — the thread goes back to running
	// because there are events after the pause checkpoint.
	th.Append(Event{
		ID:        "tool-call-2",
		Type:      EventToolCall,
		Timestamp: time.Now(),
		Data:      "resumed work",
	})
	status, _ = cm.Status(ctx, th.ID)
	if status != StatusRunning {
		t.Fatalf("after resume work: got %q, want running", status)
	}

	cm.Complete(ctx, th)
	status, _ = cm.Status(ctx, th.ID)
	if status != StatusCompleted {
		t.Fatalf("after complete: got %q, want completed", status)
	}
}

func TestCheckpointManager_ListPaused(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	// Create several threads with different states.
	running1 := NewThreadWithID("running-1")
	running1.Append(Event{ID: "e1", Type: EventToolCall, Timestamp: time.Now(), Data: "work"})
	cm.Save(ctx, running1)

	paused1 := NewThreadWithID("paused-1")
	paused1.Append(Event{ID: "e2", Type: EventToolCall, Timestamp: time.Now(), Data: "work"})
	cm.Pause(ctx, paused1, "awaiting input")

	paused2 := NewThreadWithID("paused-2")
	paused2.Append(Event{ID: "e3", Type: EventToolCall, Timestamp: time.Now(), Data: "work"})
	cm.Pause(ctx, paused2, "tool boundary")

	completed1 := NewThreadWithID("completed-1")
	completed1.Append(Event{ID: "e4", Type: EventToolCall, Timestamp: time.Now(), Data: "work"})
	cm.Complete(ctx, completed1)

	failed1 := NewThreadWithID("failed-1")
	failed1.Append(Event{ID: "e5", Type: EventToolCall, Timestamp: time.Now(), Data: "work"})
	cm.Fail(ctx, failed1, "crash")

	// List paused.
	paused, err := cm.ListPaused(ctx)
	if err != nil {
		t.Fatalf("ListPaused: %v", err)
	}
	if len(paused) != 2 {
		t.Fatalf("expected 2 paused threads, got %d", len(paused))
	}

	// Verify all returned threads are actually paused.
	pausedIDs := make(map[string]bool)
	for _, th := range paused {
		pausedIDs[th.ID] = true
	}
	if !pausedIDs["paused-1"] {
		t.Error("expected paused-1 in results")
	}
	if !pausedIDs["paused-2"] {
		t.Error("expected paused-2 in results")
	}
}

func TestCheckpointManager_ListPaused_Empty(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	paused, err := cm.ListPaused(ctx)
	if err != nil {
		t.Fatalf("ListPaused: %v", err)
	}
	if len(paused) != 0 {
		t.Fatalf("expected 0 paused threads, got %d", len(paused))
	}
}

func TestCheckpointManager_Complete(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)
	eventsBefore := th.Len()

	id, err := cm.Complete(ctx, th)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if id != th.ID {
		t.Fatalf("ID: got %q, want %q", id, th.ID)
	}
	if th.Len() != eventsBefore+1 {
		t.Fatalf("expected %d events, got %d", eventsBefore+1, th.Len())
	}

	last, _ := th.Last()
	if last.Metadata[checkpointStatusKey] != string(StatusCompleted) {
		t.Fatalf("status: got %q, want %q", last.Metadata[checkpointStatusKey], StatusCompleted)
	}
}

func TestCheckpointManager_CompleteNil(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	_, err := cm.Complete(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error completing nil thread")
	}
}

func TestCheckpointManager_Fail(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)

	id, err := cm.Fail(ctx, th, "tool execution failed")
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if id != th.ID {
		t.Fatalf("ID: got %q, want %q", id, th.ID)
	}

	last, _ := th.Last()
	if last.Metadata[checkpointStatusKey] != string(StatusFailed) {
		t.Fatalf("status: got %q, want %q", last.Metadata[checkpointStatusKey], StatusFailed)
	}
	if last.Metadata[checkpointReasonKey] != "tool execution failed" {
		t.Fatalf("reason: got %q, want %q", last.Metadata[checkpointReasonKey], "tool execution failed")
	}
}

func TestCheckpointManager_FailNil(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	_, err := cm.Fail(context.Background(), nil, "reason")
	if err == nil {
		t.Fatal("expected error failing nil thread")
	}
}

func TestCheckpointManager_ConcurrentSaveResume(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	const goroutines = 50

	// Pre-create threads.
	threads := make([]*Thread, goroutines)
	for i := range goroutines {
		th := NewThreadWithID("concurrent-" + itoa(i))
		th.Append(Event{
			ID:        "e-" + itoa(i),
			Type:      EventToolCall,
			Timestamp: time.Now(),
			Data:      "work",
		})
		threads[i] = th
	}

	var wg sync.WaitGroup

	// Concurrent saves.
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := cm.Save(ctx, threads[idx])
			if err != nil {
				t.Errorf("Save %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent resumes.
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			th, err := cm.Resume(ctx, threads[idx].ID)
			if err != nil {
				t.Errorf("Resume %d: %v", idx, err)
			}
			if th.ID != threads[idx].ID {
				t.Errorf("Resume %d: got ID %q, want %q", idx, th.ID, threads[idx].ID)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent mixed operations: pause, status, listPaused.
	for i := range goroutines {
		wg.Add(3)
		go func(idx int) {
			defer wg.Done()
			cm.Pause(ctx, threads[idx], "concurrent pause")
		}(i)
		go func(idx int) {
			defer wg.Done()
			cm.Status(ctx, threads[idx].ID)
		}(i)
		go func() {
			defer wg.Done()
			cm.ListPaused(ctx)
		}()
	}
	wg.Wait()

	// Verify all threads are paused.
	paused, err := cm.ListPaused(ctx)
	if err != nil {
		t.Fatalf("ListPaused: %v", err)
	}
	if len(paused) != goroutines {
		t.Fatalf("expected %d paused threads, got %d", goroutines, len(paused))
	}
}

func TestCheckpointManager_PauseResumeCycle(t *testing.T) {
	cm, _ := newTestCheckpointManager()
	ctx := context.Background()

	th := newTestThread(t)

	// Pause.
	_, err := cm.Pause(ctx, th, "step 1 boundary")
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}

	// Resume.
	resumed, err := cm.Resume(ctx, th.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}

	// Continue work on resumed thread.
	resumed.Append(Event{
		ID:        "tool-call-3",
		Type:      EventToolCall,
		Timestamp: time.Now(),
		Data:      "next step",
	})

	// Pause again.
	_, err = cm.Pause(ctx, resumed, "step 2 boundary")
	if err != nil {
		t.Fatalf("second Pause: %v", err)
	}

	status, _ := cm.Status(ctx, th.ID)
	if status != StatusPaused {
		t.Fatalf("expected paused after second pause, got %q", status)
	}

	// Complete.
	_, err = cm.Complete(ctx, resumed)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	status, _ = cm.Status(ctx, th.ID)
	if status != StatusCompleted {
		t.Fatalf("expected completed, got %q", status)
	}

	// No longer in paused list.
	paused, _ := cm.ListPaused(ctx)
	if len(paused) != 0 {
		t.Fatalf("expected 0 paused after complete, got %d", len(paused))
	}
}

func TestCheckpointManager_ThreadStoreAll(t *testing.T) {
	store := NewThreadStore()

	// Empty store.
	all := store.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 threads, got %d", len(all))
	}

	// Add threads.
	store.Put(NewThreadWithID("a"))
	store.Put(NewThreadWithID("b"))
	store.Put(NewThreadWithID("c"))

	all = store.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 threads, got %d", len(all))
	}

	ids := make(map[string]bool)
	for _, th := range all {
		ids[th.ID] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !ids[want] {
			t.Errorf("missing thread %q in All()", want)
		}
	}
}
