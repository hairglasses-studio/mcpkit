package session

import (
	"context"
	"testing"
	"time"
)

func TestTTLPolicy(t *testing.T) {
	t.Run("zero_ttl_never_evicts", func(t *testing.T) {
		p := TTLPolicy{TTL: 0}
		s := newSession("s1", 0)
		if p.ShouldEvict(s) {
			t.Error("expected zero TTL policy to never evict")
		}
	})

	t.Run("evicts_old_session", func(t *testing.T) {
		p := TTLPolicy{TTL: time.Millisecond}
		// Create a session and make it look old by waiting
		s := newSession("s2", 0)
		time.Sleep(5 * time.Millisecond)
		if !p.ShouldEvict(s) {
			t.Error("expected old session to be evicted")
		}
	})

	t.Run("does_not_evict_fresh_session", func(t *testing.T) {
		p := TTLPolicy{TTL: time.Hour}
		s := newSession("s3", 0)
		if p.ShouldEvict(s) {
			t.Error("expected fresh session not to be evicted")
		}
	})
}

func TestIdlePolicy(t *testing.T) {
	t.Run("zero_expiry_no_evict", func(t *testing.T) {
		p := IdlePolicy{}
		s := newSession("s1", 0) // no TTL = zero expiry
		if p.ShouldEvict(s) {
			t.Error("expected zero-expiry session not to be evicted")
		}
	})

	t.Run("expired_session_evicted", func(t *testing.T) {
		p := IdlePolicy{}
		s := newSession("s2", time.Millisecond)
		time.Sleep(5 * time.Millisecond)
		if !p.ShouldEvict(s) {
			t.Error("expected expired session to be evicted")
		}
	})

	t.Run("active_session_no_evict", func(t *testing.T) {
		p := IdlePolicy{}
		s := newSession("s3", time.Hour)
		if p.ShouldEvict(s) {
			t.Error("expected active session not to be evicted")
		}
	})
}

func TestStatePolicy(t *testing.T) {
	t.Run("active_not_evicted", func(t *testing.T) {
		p := StatePolicy{}
		s := newSession("s1", 0)
		if p.ShouldEvict(s) {
			t.Error("expected active session not to be evicted")
		}
	})

	t.Run("closed_evicted", func(t *testing.T) {
		p := StatePolicy{}
		s := newSession("s2", 0)
		_ = s.Close()
		if !p.ShouldEvict(s) {
			t.Error("expected closed session to be evicted")
		}
	})

	t.Run("expired_state_evicted", func(t *testing.T) {
		p := StatePolicy{}
		s := newSession("s3", 0)
		s.mu.Lock()
		s.state = StateExpired
		s.mu.Unlock()
		if !p.ShouldEvict(s) {
			t.Error("expected expired-state session to be evicted")
		}
	})
}

func TestCompositePolicy(t *testing.T) {
	t.Run("no_policies_no_evict", func(t *testing.T) {
		cp := CompositePolicy{}
		s := newSession("s1", 0)
		if cp.ShouldEvict(s) {
			t.Error("expected empty composite policy not to evict")
		}
	})

	t.Run("any_policy_triggers_eviction", func(t *testing.T) {
		cp := CompositePolicy{
			Policies: []EvictionPolicy{
				TTLPolicy{TTL: time.Hour}, // won't evict
				StatePolicy{},              // won't evict active
			},
		}
		s := newSession("s2", 0)
		if cp.ShouldEvict(s) {
			t.Error("expected active session not to be evicted")
		}

		// Close the session — StatePolicy should now trigger
		_ = s.Close()
		if !cp.ShouldEvict(s) {
			t.Error("expected closed session to be evicted by StatePolicy")
		}
	})
}

func TestRunEviction(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore(Options{})
	defer store.Close()

	// Create two sessions.
	s1, _ := store.Create(ctx)
	s2, _ := store.Create(ctx)

	// Close s1 so StatePolicy evicts it.
	_ = s1.Close()

	sessions := []Session{s1, s2}
	policy := StatePolicy{}

	if err := RunEviction(ctx, store, sessions, policy); err != nil {
		t.Fatalf("RunEviction returned error: %v", err)
	}

	// s1 should be gone.
	if _, ok, _ := store.Get(ctx, s1.ID()); ok {
		t.Error("expected s1 to be deleted after eviction")
	}
	// s2 should still be present.
	if _, ok, _ := store.Get(ctx, s2.ID()); !ok {
		t.Error("expected s2 to still be present")
	}
}
