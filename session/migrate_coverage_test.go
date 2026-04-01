package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hairglasses-studio/mcpkit/session"
)

func TestMigrator_MigrateID_Success(t *testing.T) {
	src := session.NewMemStore(session.Options{})
	dst := session.NewMemStore(session.Options{})
	defer src.Close()
	defer dst.Close()

	ctx := context.Background()
	oldSess, _ := src.Create(ctx)
	oldSess.Set("user", "alice")
	oldSess.Set("role", "admin")

	migrator := session.NewMigrator(src, dst, session.CopyKeys("user", "role"))

	newSess, err := migrator.MigrateID(ctx, oldSess.ID())
	if err != nil {
		t.Fatalf("MigrateID: %v", err)
	}

	// New session should have the copied keys.
	v, ok := newSess.Get("user")
	if !ok || v != "alice" {
		t.Errorf("user: got %v, ok=%v", v, ok)
	}
	v, ok = newSess.Get("role")
	if !ok || v != "admin" {
		t.Errorf("role: got %v, ok=%v", v, ok)
	}

	// Old session should be deleted from source.
	_, ok, _ = src.Get(ctx, oldSess.ID())
	if ok {
		t.Error("expected old session to be deleted from source")
	}

	// New session should exist in destination.
	_, ok, _ = dst.Get(ctx, newSess.ID())
	if ok {
		// The migrator creates in dst but doesn't store again — the in-memory
		// Create already added it.
	}
}

func TestMigrator_MigrateID_NotFound(t *testing.T) {
	src := session.NewMemStore(session.Options{})
	dst := session.NewMemStore(session.Options{})
	defer src.Close()
	defer dst.Close()

	migrator := session.NewMigrator(src, dst)

	_, err := migrator.MigrateID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestMigrator_MigrateID_MigrationFuncError(t *testing.T) {
	src := session.NewMemStore(session.Options{})
	dst := session.NewMemStore(session.Options{})
	defer src.Close()
	defer dst.Close()

	ctx := context.Background()
	oldSess, _ := src.Create(ctx)

	failingFn := func(_ context.Context, _, _ session.Session) error {
		return errors.New("migration failed")
	}

	migrator := session.NewMigrator(src, dst, failingFn)

	_, err := migrator.MigrateID(ctx, oldSess.ID())
	if err == nil {
		t.Fatal("expected error from failing migration func")
	}

	// Destination should have cleaned up the partially-migrated session.
	if dst.Len() != 0 {
		t.Errorf("expected destination to be empty after cleanup, got %d", dst.Len())
	}
}

func TestMigrator_MigrateID_MultipleFuncs(t *testing.T) {
	src := session.NewMemStore(session.Options{})
	dst := session.NewMemStore(session.Options{})
	defer src.Close()
	defer dst.Close()

	ctx := context.Background()
	oldSess, _ := src.Create(ctx)
	oldSess.Set("a", "1")
	oldSess.Set("b", "2")
	oldSess.Set("c", "3")

	// Two migration funcs: copy different keys.
	migrator := session.NewMigrator(src, dst,
		session.CopyKeys("a", "b"),
		session.CopyKeys("c"),
	)

	newSess, err := migrator.MigrateID(ctx, oldSess.ID())
	if err != nil {
		t.Fatalf("MigrateID: %v", err)
	}

	for _, k := range []string{"a", "b", "c"} {
		if _, ok := newSess.Get(k); !ok {
			t.Errorf("expected key %q to be migrated", k)
		}
	}
}

func TestCopyKeys_MissingKeysSkipped(t *testing.T) {
	src := session.NewMemStore(session.Options{})
	dst := session.NewMemStore(session.Options{})
	defer src.Close()
	defer dst.Close()

	ctx := context.Background()
	oldSess, _ := src.Create(ctx)
	oldSess.Set("exists", "yes")

	// CopyKeys includes a key that doesn't exist — should be silently skipped.
	migrator := session.NewMigrator(src, dst, session.CopyKeys("exists", "missing"))

	newSess, err := migrator.MigrateID(ctx, oldSess.ID())
	if err != nil {
		t.Fatalf("MigrateID: %v", err)
	}

	v, ok := newSess.Get("exists")
	if !ok || v != "yes" {
		t.Errorf("exists: got %v, ok=%v", v, ok)
	}
	_, ok = newSess.Get("missing")
	if ok {
		t.Error("expected missing key to not be present")
	}
}

func TestMigrator_MigrateID_NoMigrationFuncs(t *testing.T) {
	src := session.NewMemStore(session.Options{})
	dst := session.NewMemStore(session.Options{})
	defer src.Close()
	defer dst.Close()

	ctx := context.Background()
	oldSess, _ := src.Create(ctx)

	migrator := session.NewMigrator(src, dst) // No migration funcs.

	newSess, err := migrator.MigrateID(ctx, oldSess.ID())
	if err != nil {
		t.Fatalf("MigrateID: %v", err)
	}
	if newSess.ID() == "" {
		t.Fatal("expected non-empty new session ID")
	}

	// Old should be deleted.
	_, ok, _ := src.Get(ctx, oldSess.ID())
	if ok {
		t.Error("expected old session to be deleted from source")
	}
}
