package rdcycle

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileArtifactStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	a := Artifact{ID: "test-1", Type: "scan", Content: map[string]any{"key": "val"}, CreatedAt: "2026-01-01T00:00:00Z"}
	if err := store.Save(a); err != nil {
		t.Fatal(err)
	}

	got, ok := store.Get("test-1")
	if !ok {
		t.Fatal("expected to find artifact")
	}
	if got.ID != "test-1" || got.Type != "scan" {
		t.Errorf("got ID=%q Type=%q", got.ID, got.Type)
	}
}

func TestFileArtifactStoreGetMissing(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected not found for missing artifact")
	}
}

func TestFileArtifactStoreOverwrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	a1 := Artifact{ID: "ow-1", Type: "scan", Content: map[string]any{"v": "1"}}
	a2 := Artifact{ID: "ow-1", Type: "plan", Content: map[string]any{"v": "2"}}

	store.Save(a1)
	store.Save(a2)

	got, ok := store.Get("ow-1")
	if !ok {
		t.Fatal("expected to find artifact")
	}
	if got.Type != "plan" {
		t.Errorf("expected type=plan after overwrite, got %q", got.Type)
	}
}

func TestFileArtifactStoreListAll(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	store.Save(Artifact{ID: "a1", Type: "scan"})
	store.Save(Artifact{ID: "a2", Type: "plan"})
	store.Save(Artifact{ID: "a3", Type: "scan"})

	all := store.List("")
	if len(all) != 3 {
		t.Errorf("expected 3 artifacts, got %d", len(all))
	}
}

func TestFileArtifactStoreListByType(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	store.Save(Artifact{ID: "b1", Type: "scan"})
	store.Save(Artifact{ID: "b2", Type: "plan"})
	store.Save(Artifact{ID: "b3", Type: "scan"})

	scans := store.List("scan")
	if len(scans) != 2 {
		t.Errorf("expected 2 scan artifacts, got %d", len(scans))
	}
}

func TestFileArtifactStoreListEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	result := store.List("")
	if result != nil {
		t.Errorf("expected nil for empty store, got %v", result)
	}
}

func TestFileArtifactStoreSanitizesID(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	a := Artifact{ID: "scan/foo:bar", Type: "scan"}
	if err := store.Save(a); err != nil {
		t.Fatal(err)
	}

	got, ok := store.Get("scan/foo:bar")
	if !ok {
		t.Fatal("expected to find artifact with sanitized ID")
	}
	if got.ID != "scan/foo:bar" {
		t.Errorf("ID should be preserved in content, got %q", got.ID)
	}
}

func TestFileArtifactStoreCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "artifacts")
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Save(Artifact{ID: "x1", Type: "test"}); err != nil {
		t.Fatal(err)
	}

	got, ok := store.Get("x1")
	if !ok || got.ID != "x1" {
		t.Error("expected to find artifact in nested dir")
	}
}

func TestFileArtifactStoreDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if store.Dir() != dir {
		t.Errorf("expected dir=%q, got %q", dir, store.Dir())
	}
}

func TestFileArtifactStoreConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileArtifactStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				id := fmt.Sprintf("concurrent-%d-%d", n, j)
				store.Save(Artifact{ID: id, Type: "test"})
				store.Get(id)
				store.List("")
			}
		}(i)
	}
	wg.Wait()
}
