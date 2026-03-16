package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/session"
)

func TestMemStore_CreateAndGet(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	ctx := context.Background()
	sess, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.ID() == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.State() != session.StateActive {
		t.Fatalf("expected active state, got %v", sess.State())
	}

	got, ok, err := store.Get(ctx, sess.ID())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected to find session")
	}
	if got.ID() != sess.ID() {
		t.Fatalf("got ID %q, want %q", got.ID(), sess.ID())
	}
}

func TestMemStore_GetNotFound(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	_, ok, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}

func TestMemStore_Delete(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	ctx := context.Background()
	sess, _ := store.Create(ctx)

	if err := store.Delete(ctx, sess.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, ok, _ := store.Get(ctx, sess.ID())
	if ok {
		t.Fatal("expected session to be gone after Delete")
	}
}

func TestMemStore_TTLExpiry(t *testing.T) {
	store := session.NewMemStore(session.Options{
		TTL:              50 * time.Millisecond,
		EvictionInterval: 10 * time.Millisecond,
	})
	defer store.Close()

	ctx := context.Background()
	sess, _ := store.Create(ctx)

	// Should be active immediately.
	_, ok, _ := store.Get(ctx, sess.ID())
	if !ok {
		t.Fatal("expected active session")
	}

	// Wait for TTL to expire.
	time.Sleep(100 * time.Millisecond)

	_, ok, _ = store.Get(ctx, sess.ID())
	if ok {
		t.Fatal("expected session to be expired")
	}
}

func TestMemStore_Refresh(t *testing.T) {
	store := session.NewMemStore(session.Options{
		TTL: 100 * time.Millisecond,
	})
	defer store.Close()

	ctx := context.Background()
	sess, _ := store.Create(ctx)

	// Refresh before expiry.
	time.Sleep(60 * time.Millisecond)
	if err := store.Refresh(ctx, sess.ID()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// Session should still be active after original TTL would have elapsed.
	time.Sleep(60 * time.Millisecond)
	_, ok, _ := store.Get(ctx, sess.ID())
	if !ok {
		t.Fatal("expected session to still be active after refresh")
	}
}

func TestSession_GetSetDelete(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	sess, _ := store.Create(context.Background())

	sess.Set("key", "value")
	v, ok := sess.Get("key")
	if !ok {
		t.Fatal("expected key to be found")
	}
	if v != "value" {
		t.Fatalf("got %v, want %q", v, "value")
	}

	sess.Delete("key")
	_, ok = sess.Get("key")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestSession_Close(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	sess, _ := store.Create(context.Background())
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if sess.State() != session.StateClosed {
		t.Fatalf("expected closed state, got %v", sess.State())
	}
	// Idempotent.
	if err := sess.Close(); err != nil {
		t.Fatalf("Close (idempotent): %v", err)
	}
}

func TestContextRoundtrip(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	sess, _ := store.Create(context.Background())
	ctx := session.WithSession(context.Background(), sess)

	got, ok := session.FromContext(ctx)
	if !ok {
		t.Fatal("expected session in context")
	}
	if got.ID() != sess.ID() {
		t.Fatalf("got %q, want %q", got.ID(), sess.ID())
	}
}

func TestFromContext_Missing(t *testing.T) {
	_, ok := session.FromContext(context.Background())
	if ok {
		t.Fatal("expected no session in empty context")
	}
}

func TestMemStore_Len(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	ctx := context.Background()
	if store.Len() != 0 {
		t.Fatalf("expected 0, got %d", store.Len())
	}
	store.Create(ctx)
	store.Create(ctx)
	if store.Len() != 2 {
		t.Fatalf("expected 2, got %d", store.Len())
	}
}
