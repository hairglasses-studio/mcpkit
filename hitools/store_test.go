//go:build !official_sdk

package hitools

import (
	"sort"
	"testing"
)

func TestInMemoryResponseStore_SaveAndLoad(t *testing.T) {
	store := NewInMemoryResponseStore()

	req := PendingRequest{
		ID:        "req-001",
		Input:     RequestInput{Question: "Continue?", Urgency: UrgencyHigh},
		CreatedAt: "2026-04-05T12:00:00Z",
	}

	if err := store.Save(req); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, found, err := store.Load("req-001")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Fatal("Load: expected to find request")
	}
	if loaded.ID != "req-001" {
		t.Errorf("ID = %q, want %q", loaded.ID, "req-001")
	}
	if loaded.Input.Question != "Continue?" {
		t.Errorf("Question = %q, want %q", loaded.Input.Question, "Continue?")
	}
}

func TestInMemoryResponseStore_LoadNotFound(t *testing.T) {
	store := NewInMemoryResponseStore()

	_, found, err := store.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Error("expected not found for nonexistent ID")
	}
}

func TestInMemoryResponseStore_Complete(t *testing.T) {
	store := NewInMemoryResponseStore()

	req := PendingRequest{
		ID:        "req-002",
		Input:     RequestInput{Question: "Approve?"},
		CreatedAt: "2026-04-05T12:00:00Z",
	}
	store.Save(req)

	output := RequestOutput{
		Status:    "accepted",
		Response:  "yes",
		Timestamp: "2026-04-05T12:01:00Z",
	}
	if err := store.Complete("req-002", output); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Should no longer be pending.
	_, found, _ := store.Load("req-002")
	if found {
		t.Error("completed request should not be in pending set")
	}

	// Should be in completed set.
	completed, ok := store.GetCompleted("req-002")
	if !ok {
		t.Fatal("expected completed response")
	}
	if completed.Response != "yes" {
		t.Errorf("Response = %q, want %q", completed.Response, "yes")
	}
}

func TestInMemoryResponseStore_ListPending(t *testing.T) {
	store := NewInMemoryResponseStore()

	store.Save(PendingRequest{ID: "a", CreatedAt: "2026-04-05T12:00:00Z"})
	store.Save(PendingRequest{ID: "b", CreatedAt: "2026-04-05T12:01:00Z"})
	store.Save(PendingRequest{ID: "c", CreatedAt: "2026-04-05T12:02:00Z"})

	ids, err := store.ListPending()
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	sort.Strings(ids)
	if len(ids) != 3 {
		t.Fatalf("ListPending = %d items, want 3", len(ids))
	}
	if ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Errorf("ListPending = %v, want [a b c]", ids)
	}
}

func TestInMemoryResponseStore_Delete(t *testing.T) {
	store := NewInMemoryResponseStore()

	store.Save(PendingRequest{ID: "del-001", CreatedAt: "2026-04-05T12:00:00Z"})

	if err := store.Delete("del-001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, found, _ := store.Load("del-001")
	if found {
		t.Error("deleted request should not be found")
	}
}

func TestInMemoryResponseStore_GetCompleted_NotFound(t *testing.T) {
	store := NewInMemoryResponseStore()

	_, ok := store.GetCompleted("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent completed ID")
	}
}

func TestInMemoryResponseStore_OverwriteSave(t *testing.T) {
	store := NewInMemoryResponseStore()

	store.Save(PendingRequest{ID: "ow-001", Input: RequestInput{Question: "v1"}, CreatedAt: "2026-04-05T12:00:00Z"})
	store.Save(PendingRequest{ID: "ow-001", Input: RequestInput{Question: "v2"}, CreatedAt: "2026-04-05T12:01:00Z"})

	loaded, found, _ := store.Load("ow-001")
	if !found {
		t.Fatal("expected to find overwritten request")
	}
	if loaded.Input.Question != "v2" {
		t.Errorf("Question = %q, want %q (should be overwritten)", loaded.Input.Question, "v2")
	}
}
