package rdcycle

import (
	"fmt"
	"sync"
	"testing"
)

func TestInMemoryArtifactStore_SaveAndGet(t *testing.T) {
	t.Parallel()
	store := NewInMemoryArtifactStore()

	a := Artifact{
		ID:        "test-1",
		Type:      "scan",
		CreatedAt: "2026-01-01T00:00:00Z",
		Content:   map[string]any{"key": "value"},
	}

	if err := store.Save(a); err != nil {
		t.Fatalf("Save: unexpected error: %v", err)
	}

	got, ok := store.Get("test-1")
	if !ok {
		t.Fatal("Get: expected artifact to be found")
	}
	if got.ID != a.ID {
		t.Errorf("Get ID: want %q, got %q", a.ID, got.ID)
	}
	if got.Type != a.Type {
		t.Errorf("Get Type: want %q, got %q", a.Type, got.Type)
	}
}

func TestInMemoryArtifactStore_GetMissing(t *testing.T) {
	t.Parallel()
	store := NewInMemoryArtifactStore()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("Get: expected false for nonexistent artifact")
	}
}

func TestInMemoryArtifactStore_Overwrite(t *testing.T) {
	t.Parallel()
	store := NewInMemoryArtifactStore()

	a1 := Artifact{ID: "x", Type: "scan", Content: map[string]any{"v": 1}}
	a2 := Artifact{ID: "x", Type: "plan", Content: map[string]any{"v": 2}}

	_ = store.Save(a1)
	_ = store.Save(a2)

	got, ok := store.Get("x")
	if !ok {
		t.Fatal("Get: expected artifact after overwrite")
	}
	if got.Type != "plan" {
		t.Errorf("Overwrite Type: want %q, got %q", "plan", got.Type)
	}
}

func TestInMemoryArtifactStore_ListAll(t *testing.T) {
	t.Parallel()
	store := NewInMemoryArtifactStore()

	for i := 0; i < 3; i++ {
		_ = store.Save(Artifact{ID: fmt.Sprintf("a%d", i), Type: "scan"})
	}
	for i := 0; i < 2; i++ {
		_ = store.Save(Artifact{ID: fmt.Sprintf("b%d", i), Type: "plan"})
	}

	all := store.List("")
	if len(all) != 5 {
		t.Errorf("List all: want 5, got %d", len(all))
	}
}

func TestInMemoryArtifactStore_ListByType(t *testing.T) {
	t.Parallel()
	store := NewInMemoryArtifactStore()

	for i := 0; i < 3; i++ {
		_ = store.Save(Artifact{ID: fmt.Sprintf("scan-%d", i), Type: "scan"})
	}
	_ = store.Save(Artifact{ID: "plan-0", Type: "plan"})

	scans := store.List("scan")
	if len(scans) != 3 {
		t.Errorf("List scan: want 3, got %d", len(scans))
	}

	plans := store.List("plan")
	if len(plans) != 1 {
		t.Errorf("List plan: want 1, got %d", len(plans))
	}

	empty := store.List("verify")
	if len(empty) != 0 {
		t.Errorf("List verify: want 0, got %d", len(empty))
	}
}

func TestInMemoryArtifactStore_ListEmpty(t *testing.T) {
	t.Parallel()
	store := NewInMemoryArtifactStore()
	all := store.List("")
	if all != nil {
		t.Errorf("List empty store: want nil, got %v", all)
	}
}

func TestInMemoryArtifactStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	store := NewInMemoryArtifactStore()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.Save(Artifact{
				ID:      fmt.Sprintf("concurrent-%d", i),
				Type:    "scan",
				Content: map[string]any{"i": i},
			})
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.List("scan")
		}()
	}
	wg.Wait()

	all := store.List("")
	if len(all) != 20 {
		t.Errorf("concurrent Save: want 20 artifacts, got %d", len(all))
	}
}
