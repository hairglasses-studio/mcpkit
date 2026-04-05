package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// State represents the lifecycle state of a session.
type State int

const (
	// StateActive indicates the session is alive and usable.
	StateActive State = iota
	// StateClosed indicates the session has been explicitly closed.
	StateClosed
	// StateExpired indicates the session has exceeded its TTL.
	StateExpired
)

// Session is the interface for a single user session.
type Session interface {
	// ID returns the unique session identifier.
	ID() string
	// State returns the current lifecycle state.
	State() State
	// CreatedAt returns when the session was created.
	CreatedAt() time.Time
	// ExpiresAt returns when the session will expire. Zero means no expiry.
	ExpiresAt() time.Time
	// Get retrieves a value by key.
	Get(key string) (any, bool)
	// Set stores a key-value pair.
	Set(key string, val any)
	// Delete removes a value by key.
	Delete(key string)
	// Close marks the session as closed. Idempotent.
	Close() error
}

// session is the concrete implementation of Session.
type session struct {
	mu        sync.RWMutex
	id        string
	data      map[string]any
	state     State
	createdAt time.Time
	expiresAt time.Time // zero means no expiry
}

// newSession creates a new session with the given ID and TTL.
func newSession(id string, ttl time.Duration) *session {
	s := &session{
		id:        id,
		data:      make(map[string]any),
		state:     StateActive,
		createdAt: time.Now(),
	}
	if ttl > 0 {
		s.expiresAt = time.Now().Add(ttl)
	}
	return s
}

func (s *session) ID() string {
	return s.id
}

func (s *session) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *session) CreatedAt() time.Time {
	return s.createdAt
}

func (s *session) ExpiresAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expiresAt
}

func (s *session) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *session) Set(key string, val any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}

func (s *session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateClosed
	return nil
}

func (s *session) isExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(s.expiresAt)
}

func (s *session) refresh(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ttl > 0 {
		s.expiresAt = time.Now().Add(ttl)
	}
}

// SessionStore is the interface for session storage backends.
type SessionStore interface {
	// Create creates a new session and returns it.
	Create(ctx context.Context) (Session, error)
	// Get retrieves a session by ID. Returns (nil, false, nil) if not found.
	Get(ctx context.Context, id string) (Session, bool, error)
	// Delete removes a session by ID.
	Delete(ctx context.Context, id string) error
	// Refresh resets the TTL of a session.
	Refresh(ctx context.Context, id string) error
	// Close shuts down the store, stopping any background goroutines.
	Close() error
}

// Options configures a MemStore.
type Options struct {
	// TTL is the session lifetime. Zero means sessions never expire.
	TTL time.Duration
	// EvictionInterval controls how often expired sessions are swept.
	// Defaults to TTL/2 when TTL is set, otherwise no background sweeper runs.
	EvictionInterval time.Duration
	// Context, when set, stops the eviction loop when cancelled. This provides
	// an alternative to calling Close() for lifecycle management.
	Context context.Context
}

// MemStore is an in-memory implementation of Store with optional TTL eviction.
type MemStore struct {
	mu       sync.RWMutex
	sessions map[string]*session
	opts     Options
	done     chan struct{}
}

// NewMemStore creates a new in-memory session store.
func NewMemStore(opts Options) *MemStore {
	ms := &MemStore{
		sessions: make(map[string]*session),
		opts:     opts,
		done:     make(chan struct{}),
	}

	interval := opts.EvictionInterval
	if interval == 0 && opts.TTL > 0 {
		interval = max(opts.TTL/2, time.Millisecond)
	}

	if interval > 0 {
		go ms.evictLoop(interval, opts.Context)
	}

	return ms
}

func (ms *MemStore) evictLoop(interval time.Duration, ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var ctxDone <-chan struct{}
	if ctx != nil {
		ctxDone = ctx.Done()
	}

	for {
		select {
		case <-ticker.C:
			ms.Evict()
		case <-ms.done:
			return
		case <-ctxDone:
			return
		}
	}
}

// Evict removes all expired sessions from the store.
func (ms *MemStore) Evict() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for id, s := range ms.sessions {
		if s.isExpired() {
			delete(ms.sessions, id)
		}
	}
}

// Create creates a new session with a random ID.
func (ms *MemStore) Create(_ context.Context) (Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	s := newSession(id, ms.opts.TTL)
	ms.mu.Lock()
	ms.sessions[id] = s
	ms.mu.Unlock()
	return s, nil
}

// Get retrieves a session by ID. Returns (nil, false, nil) if not found or expired.
func (ms *MemStore) Get(_ context.Context, id string) (Session, bool, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	s, ok := ms.sessions[id]
	if !ok {
		return nil, false, nil
	}
	if s.isExpired() {
		delete(ms.sessions, id)
		return nil, false, nil
	}
	return s, true, nil
}

// Delete removes a session by ID.
func (ms *MemStore) Delete(_ context.Context, id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.sessions, id)
	return nil
}

// Refresh resets the TTL for a session.
func (ms *MemStore) Refresh(_ context.Context, id string) error {
	ms.mu.RLock()
	s, ok := ms.sessions[id]
	ms.mu.RUnlock()
	if !ok {
		return errors.New("session not found")
	}
	s.refresh(ms.opts.TTL)
	return nil
}

// Len returns the number of sessions in the store (including expired ones
// not yet evicted).
func (ms *MemStore) Len() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return len(ms.sessions)
}

// Close stops the background eviction goroutine.
func (ms *MemStore) Close() error {
	select {
	case <-ms.done:
		// already closed
	default:
		close(ms.done)
	}
	return nil
}

// generateID creates a cryptographically random 16-byte hex session ID.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// contextKey is the unexported type for context keys in this package.
type contextKey struct{}

// WithSession returns a new context with the given session attached.
func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// FromContext retrieves the session from the context.
// Returns (nil, false) if no session is present.
func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(contextKey{}).(Session)
	return s, ok
}
