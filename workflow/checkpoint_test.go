package workflow

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestNewMemoryCheckpointStore_Valid(t *testing.T) {
	t.Parallel()
	store := NewMemoryCheckpointStore()
	if store == nil {
		t.Fatal("NewMemoryCheckpointStore returned nil")
	}
	if store.checkpoints == nil {
		t.Fatal("internal checkpoints map is nil")
	}
}

func TestCheckpointStore_SaveLoad_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	state := NewState()
	state.Data["x"] = 42
	state.Metadata["env"] = "test"

	now := time.Now().UTC()
	cp := Checkpoint{
		RunID:       "run-123",
		CurrentNode: "node-b",
		Step:        7,
		State:       state,
		SavedAt:     now,
	}

	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, found, err := store.Load(ctx, "run-123")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !found {
		t.Fatal("Load: checkpoint not found after Save")
	}
	if loaded.RunID != "run-123" {
		t.Errorf("RunID = %q; want %q", loaded.RunID, "run-123")
	}
	if loaded.CurrentNode != "node-b" {
		t.Errorf("CurrentNode = %q; want %q", loaded.CurrentNode, "node-b")
	}
	if loaded.Step != 7 {
		t.Errorf("Step = %d; want 7", loaded.Step)
	}
	if loaded.State.Data["x"] != 42 {
		t.Errorf("State.Data[x] = %v; want 42", loaded.State.Data["x"])
	}
	if loaded.State.Metadata["env"] != "test" {
		t.Errorf("State.Metadata[env] = %q; want %q", loaded.State.Metadata["env"], "test")
	}
}

func TestCheckpointStore_Load_MissingKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	_, found, err := store.Load(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if found {
		t.Error("Load: expected found=false for missing key")
	}
}

func TestCheckpointStore_Delete_RemovesEntry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	cp := Checkpoint{RunID: "del-run", Step: 1, State: NewState(), SavedAt: time.Now()}
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify it is there.
	_, found, _ := store.Load(ctx, "del-run")
	if !found {
		t.Fatal("checkpoint not found before Delete")
	}

	if err := store.Delete(ctx, "del-run"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, found, err := store.Load(ctx, "del-run")
	if err != nil {
		t.Fatalf("Load after Delete error: %v", err)
	}
	if found {
		t.Error("Load: expected found=false after Delete")
	}
}

func TestCheckpointStore_Delete_NonexistentIsNoError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	// Deleting a key that was never saved must not error.
	if err := store.Delete(ctx, "ghost"); err != nil {
		t.Errorf("Delete of nonexistent key returned error: %v", err)
	}
}

func TestCheckpointStore_List_ReturnsMultiple(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	ids := []string{"r1", "r2", "r3"}
	for _, id := range ids {
		cp := Checkpoint{RunID: id, Step: 1, State: NewState(), SavedAt: time.Now()}
		if err := store.Save(ctx, cp); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("List returned %d entries; want 3", len(listed))
	}

	sort.Strings(listed)
	sort.Strings(ids)
	for i, want := range ids {
		if listed[i] != want {
			t.Errorf("List[%d] = %q; want %q", i, listed[i], want)
		}
	}
}

func TestCheckpointStore_List_EmptyReturnsEmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if listed == nil {
		t.Error("List on empty store returned nil; want empty (non-nil) slice")
	}
	if len(listed) != 0 {
		t.Errorf("List returned %d entries; want 0", len(listed))
	}
}

func TestCheckpointStore_Save_OverwritesExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	cp1 := Checkpoint{RunID: "ow-run", CurrentNode: "first", Step: 1, State: NewState(), SavedAt: time.Now()}
	if err := store.Save(ctx, cp1); err != nil {
		t.Fatalf("Save (first): %v", err)
	}

	cp2 := Checkpoint{RunID: "ow-run", CurrentNode: "second", Step: 99, State: NewState(), SavedAt: time.Now()}
	if err := store.Save(ctx, cp2); err != nil {
		t.Fatalf("Save (second): %v", err)
	}

	loaded, found, err := store.Load(ctx, "ow-run")
	if err != nil || !found {
		t.Fatalf("Load: found=%v err=%v", found, err)
	}
	if loaded.CurrentNode != "second" {
		t.Errorf("CurrentNode = %q; want %q (overwrite)", loaded.CurrentNode, "second")
	}
	if loaded.Step != 99 {
		t.Errorf("Step = %d; want 99 (overwrite)", loaded.Step)
	}

	// There must still be only one entry.
	listed, _ := store.List(ctx)
	if len(listed) != 1 {
		t.Errorf("List after overwrite returned %d entries; want 1", len(listed))
	}
}

func TestCheckpointStore_ConcurrentSaveLoad(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		i := i
		go func() {
			defer wg.Done()
			runID := "concurrent-run"
			cp := Checkpoint{
				RunID:   runID,
				Step:    i,
				State:   NewState(),
				SavedAt: time.Now(),
			}
			// Alternate between writers and readers.
			if i%2 == 0 {
				_ = store.Save(ctx, cp)
			} else {
				_, _, _ = store.Load(ctx, runID)
			}
		}()
	}

	wg.Wait()

	// After concurrent access the store must be in a consistent state:
	// Load must not panic or error.
	_, _, err := store.Load(ctx, "concurrent-run")
	if err != nil {
		t.Errorf("Load after concurrent access: %v", err)
	}
}

func TestCheckpointStore_ConcurrentDistinctKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryCheckpointStore()

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		i := i
		go func() {
			defer wg.Done()
			runID := "key-" + string(rune('A'+i%26))
			cp := Checkpoint{RunID: runID, Step: i, State: NewState(), SavedAt: time.Now()}
			_ = store.Save(ctx, cp)
		}()
	}

	wg.Wait()

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List after concurrent saves: %v", err)
	}
	if len(listed) == 0 {
		t.Error("List returned 0 entries after concurrent saves")
	}
}
