package session_test

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/session"
)

func TestMigrateStatefulSessions(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	ctx := context.Background()

	// Simulate existing stateful sessions (e.g., from a legacy map).
	legacy := map[string]map[string]any{
		"old-sess-1": {"user": "alice", "role": "admin"},
		"old-sess-2": {"user": "bob", "role": "viewer"},
	}

	result, err := session.MigrateStatefulSessions(ctx, store, legacy)
	if err != nil {
		t.Fatalf("MigrateStatefulSessions: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 migrated sessions, got %d", len(result))
	}

	for oldID, newID := range result {
		if oldID == "" || newID == "" {
			t.Fatal("expected non-empty IDs")
		}
		sess, ok, err := store.Get(ctx, newID)
		if err != nil {
			t.Fatalf("Get(%q): %v", newID, err)
		}
		if !ok {
			t.Fatalf("session %q not found after migration", newID)
		}
		// Check that values were copied.
		origData := legacy[oldID]
		for k, wantV := range origData {
			gotV, exists := sess.Get(k)
			if !exists {
				t.Errorf("key %q not found in migrated session", k)
				continue
			}
			if gotV != wantV {
				t.Errorf("key %q: got %v, want %v", k, gotV, wantV)
			}
		}
	}
}

func TestMigrateStatefulSessions_Empty(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	result, err := session.MigrateStatefulSessions(context.Background(), store, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result))
	}
}
