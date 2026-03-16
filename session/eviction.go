package session

import (
	"context"
	"time"
)

// EvictionPolicy defines how a session should be evaluated for eviction.
type EvictionPolicy interface {
	// ShouldEvict returns true if the session should be removed from the store.
	ShouldEvict(sess Session) bool
}

// TTLPolicy evicts sessions that have exceeded a fixed TTL since creation.
type TTLPolicy struct {
	TTL time.Duration
}

// ShouldEvict returns true if the session was created more than TTL ago.
func (p TTLPolicy) ShouldEvict(sess Session) bool {
	if p.TTL <= 0 {
		return false
	}
	return time.Since(sess.CreatedAt()) > p.TTL
}

// IdlePolicy evicts sessions whose ExpiresAt has passed (i.e. idle TTL).
type IdlePolicy struct{}

// ShouldEvict returns true if the session's ExpiresAt is set and in the past.
func (p IdlePolicy) ShouldEvict(sess Session) bool {
	exp := sess.ExpiresAt()
	if exp.IsZero() {
		return false
	}
	return time.Now().After(exp)
}

// StatePolicy evicts sessions that are in a terminal state (expired or closed).
type StatePolicy struct{}

// ShouldEvict returns true if the session state is not active.
func (p StatePolicy) ShouldEvict(sess Session) bool {
	st := sess.State()
	return st == StateExpired || st == StateClosed
}

// CompositePolicy combines multiple policies with OR semantics — a session is
// evicted if any policy says it should be.
type CompositePolicy struct {
	Policies []EvictionPolicy
}

// ShouldEvict returns true if any constituent policy returns true.
func (c CompositePolicy) ShouldEvict(sess Session) bool {
	for _, p := range c.Policies {
		if p.ShouldEvict(sess) {
			return true
		}
	}
	return false
}

// RunEviction iterates over a snapshot of sessions and deletes any that the
// policy marks for eviction. store must implement a List method; if not
// available, use MemStore directly.
//
// For MemStore, prefer the built-in eviction loop which holds the lock
// internally. This helper is useful for custom store implementations.
func RunEviction(ctx context.Context, store SessionStore, sessions []Session, policy EvictionPolicy) error {
	for _, sess := range sessions {
		if policy.ShouldEvict(sess) {
			if err := store.Delete(ctx, sess.ID()); err != nil {
				return err
			}
		}
	}
	return nil
}
