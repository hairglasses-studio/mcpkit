package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/session"
)

func TestMemStore_Refresh_NotFound(t *testing.T) {
	store := session.NewMemStore(session.Options{TTL: time.Minute})
	defer store.Close()

	err := store.Refresh(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error when refreshing nonexistent session")
	}
}

func TestMemStore_Close_Idempotent(t *testing.T) {
	store := session.NewMemStore(session.Options{TTL: time.Minute})

	if err := store.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic.
	if err := store.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestMemStore_Delete_Nonexistent(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	// Deleting a nonexistent session should not error.
	err := store.Delete(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}
}

func TestMemStore_GetExpiredSession(t *testing.T) {
	// Very short TTL, no eviction interval — manual expiry via Get.
	store := session.NewMemStore(session.Options{
		TTL: time.Millisecond,
	})
	defer store.Close()

	ctx := context.Background()
	sess, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 session, got %d", store.Len())
	}

	time.Sleep(5 * time.Millisecond)

	// Get should return not found and remove the expired session.
	_, ok, getErr := store.Get(ctx, sess.ID())
	if getErr != nil {
		t.Fatalf("Get: %v", getErr)
	}
	if ok {
		t.Fatal("expected expired session to not be found")
	}
}

func TestMemStore_EvictionInterval_MinMillisecond(t *testing.T) {
	// TTL so small that TTL/2 < Millisecond — should clamp to Millisecond.
	store := session.NewMemStore(session.Options{
		TTL: time.Microsecond, // TTL/2 = 500ns < 1ms
	})
	defer store.Close()

	ctx := context.Background()
	_, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Wait for eviction to run.
	time.Sleep(10 * time.Millisecond)

	// Session should be evicted (TTL is 1 microsecond).
	if store.Len() != 0 {
		t.Errorf("expected 0 sessions after eviction, got %d", store.Len())
	}
}

func TestMemStore_NoTTL_NoEviction(t *testing.T) {
	// Zero TTL, zero interval — no eviction goroutine.
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	ctx := context.Background()
	sess, _ := store.Create(ctx)

	time.Sleep(5 * time.Millisecond)

	// Session should still be there since there's no TTL.
	_, ok, _ := store.Get(ctx, sess.ID())
	if !ok {
		t.Fatal("expected session to persist with no TTL")
	}
}

func TestSession_ExpiresAt_Zero(t *testing.T) {
	store := session.NewMemStore(session.Options{}) // No TTL.
	defer store.Close()

	sess, _ := store.Create(context.Background())
	exp := sess.ExpiresAt()
	if !exp.IsZero() {
		t.Fatalf("expected zero ExpiresAt, got %v", exp)
	}
}

func TestSession_ExpiresAt_NonZero(t *testing.T) {
	store := session.NewMemStore(session.Options{TTL: time.Hour})
	defer store.Close()

	sess, _ := store.Create(context.Background())
	exp := sess.ExpiresAt()
	if exp.IsZero() {
		t.Fatal("expected non-zero ExpiresAt with TTL")
	}
	if time.Until(exp) < 59*time.Minute {
		t.Fatalf("expected ExpiresAt to be ~1h from now, got %v until", time.Until(exp))
	}
}

func TestSession_CreatedAt(t *testing.T) {
	before := time.Now()
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	sess, _ := store.Create(context.Background())
	after := time.Now()

	if sess.CreatedAt().Before(before) || sess.CreatedAt().After(after) {
		t.Fatalf("CreatedAt %v outside expected range [%v, %v]", sess.CreatedAt(), before, after)
	}
}

func TestMemStore_Evict_Manual(t *testing.T) {
	store := session.NewMemStore(session.Options{
		TTL: time.Millisecond,
		// No EvictionInterval — we call Evict manually.
	})
	defer store.Close()

	ctx := context.Background()
	store.Create(ctx)
	store.Create(ctx)

	if store.Len() != 2 {
		t.Fatalf("expected 2 sessions, got %d", store.Len())
	}

	time.Sleep(5 * time.Millisecond)
	store.Evict()

	if store.Len() != 0 {
		t.Fatalf("expected 0 sessions after manual eviction, got %d", store.Len())
	}
}
