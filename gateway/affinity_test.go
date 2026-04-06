//go:build !official_sdk

package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/session"
	"github.com/hairglasses-studio/mcpkit/transport"
)

// newTestMemStore creates a MemStore for use in AffinityRouter tests.
func newTestMemStore(t *testing.T) *session.MemStore {
	t.Helper()
	store := session.NewMemStore(session.Options{TTL: time.Hour})
	t.Cleanup(func() { store.Close() })
	return store
}

func TestConsistentHash_EmptyUpstreams(t *testing.T) {
	t.Parallel()
	result := consistentHash("any-session", nil, 100)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestConsistentHash_SingleUpstream(t *testing.T) {
	t.Parallel()
	result := consistentHash("any-session", []string{"only-one"}, 100)
	if result != "only-one" {
		t.Errorf("expected 'only-one', got %q", result)
	}
}

func TestConsistentHash_Deterministic(t *testing.T) {
	t.Parallel()
	upstreams := []string{"us-east-1", "us-west-2", "eu-west-1"}

	// Same session ID should always map to the same upstream.
	first := consistentHash("session-xyz", upstreams, 100)
	for range 100 {
		result := consistentHash("session-xyz", upstreams, 100)
		if result != first {
			t.Fatalf("inconsistent hash: first=%q, got=%q", first, result)
		}
	}
}

func TestConsistentHash_Distribution(t *testing.T) {
	t.Parallel()
	upstreams := []string{"a", "b", "c"}
	counts := make(map[string]int)

	// Hash many session IDs and check distribution.
	for i := range 300 {
		sessionID := "session-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		result := consistentHash(sessionID, upstreams, 100)
		counts[result]++
	}

	// Each upstream should get at least some sessions.
	for _, u := range upstreams {
		if counts[u] == 0 {
			t.Errorf("upstream %q got 0 sessions", u)
		}
	}
}

func TestConsistentHash_StableOnRemoval(t *testing.T) {
	t.Parallel()
	upstreams3 := []string{"a", "b", "c"}
	upstreams2 := []string{"a", "c"} // "b" removed

	// Count how many sessions change their upstream when "b" is removed.
	changed := 0
	total := 100
	for i := range total {
		sessionID := "test-session-" + string(rune('A'+i%26))
		before := consistentHash(sessionID, upstreams3, 100)
		after := consistentHash(sessionID, upstreams2, 100)
		if before != after {
			changed++
		}
	}

	// With consistent hashing, only ~1/3 of sessions should move (those on "b").
	// Be generous: at most 60% should change.
	if changed > total*60/100 {
		t.Errorf("too many sessions changed: %d/%d", changed, total)
	}
}

func TestAffinityRouter_Route(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"backend-1", "backend-2", "backend-3"},
	})

	// Route should be deterministic.
	first := ar.Route("my-session")
	if first == "" {
		t.Fatal("expected non-empty upstream")
	}
	for range 50 {
		got := ar.Route("my-session")
		if got != first {
			t.Fatalf("route changed: first=%q, got=%q", first, got)
		}
	}
}

func TestAffinityRouter_StickyOverride(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a", "b", "c"},
	})

	// Set a sticky override.
	ar.SetSticky("pinned-session", "b")

	result := ar.Route("pinned-session")
	if result != "b" {
		t.Errorf("expected sticky 'b', got %q", result)
	}
}

func TestAffinityRouter_StickyOverrideStaleUpstream(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a", "c"},
	})

	// Set sticky to a non-existent upstream.
	ar.SetSticky("orphan-session", "b")

	// Should fall through to consistent hash since "b" is not in upstreams.
	result := ar.Route("orphan-session")
	if result == "b" {
		t.Error("expected fallback to consistent hash, not stale sticky")
	}
	if result == "" {
		t.Error("expected non-empty upstream from consistent hash")
	}
}

func TestAffinityRouter_SetUpstreams(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a", "b"},
	})

	// Set sticky to "b".
	ar.SetSticky("sess-1", "b")

	// Replace upstreams, removing "b".
	ar.SetUpstreams([]string{"a", "c"})

	ups := ar.Upstreams()
	if len(ups) != 2 || ups[0] != "a" || ups[1] != "c" {
		t.Errorf("unexpected upstreams: %v", ups)
	}

	// Sticky to "b" should be cleaned up.
	result := ar.Route("sess-1")
	if result == "b" {
		t.Error("sticky to removed upstream should have been cleaned")
	}
}

func TestAffinityRouter_AddUpstream(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a"},
	})

	ar.AddUpstream("b")
	ups := ar.Upstreams()
	if len(ups) != 2 {
		t.Fatalf("expected 2 upstreams, got %d", len(ups))
	}

	// Adding duplicate should be no-op.
	ar.AddUpstream("b")
	ups = ar.Upstreams()
	if len(ups) != 2 {
		t.Fatalf("expected 2 upstreams after dup add, got %d", len(ups))
	}
}

func TestAffinityRouter_RemoveUpstream(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a", "b", "c"},
	})

	ar.SetSticky("s1", "b")
	ar.SetSticky("s2", "b")
	ar.SetSticky("s3", "c")

	ar.RemoveUpstream("b")
	ups := ar.Upstreams()
	if len(ups) != 2 {
		t.Fatalf("expected 2 upstreams, got %d", len(ups))
	}

	// Sticky overrides for "b" should be cleaned.
	result := ar.Route("s1")
	if result == "b" {
		t.Error("sticky to removed upstream 'b' should have been cleaned")
	}

	// "s3" sticky to "c" should still work.
	result = ar.Route("s3")
	if result != "c" {
		t.Errorf("expected sticky 'c' for s3, got %q", result)
	}
}

func TestAffinityRouter_RemoveSticky(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a", "b"},
	})

	ar.SetSticky("s1", "b")
	ar.RemoveSticky("s1")

	// Should fall back to consistent hash.
	result := ar.Route("s1")
	// Just verify it's valid, not necessarily "b".
	found := false
	for _, u := range []string{"a", "b"} {
		if result == u {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected upstream: %q", result)
	}
}

func TestAffinityRouter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a", "b", "c"},
	})

	var wg sync.WaitGroup
	const workers = 20
	const iterations = 100

	// Concurrent routing.
	for i := range workers {
		wg.Add(1)
		sessID := "conc-sess-" + string(rune('A'+i))
		go func(sid string) {
			defer wg.Done()
			for range iterations {
				_ = ar.Route(sid)
			}
		}(sessID)
	}

	// Concurrent mutations.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			ar.AddUpstream("d")
			ar.RemoveUpstream("d")
			ar.SetSticky("temp", "a")
			ar.RemoveSticky("temp")
			_ = ar.Upstreams()
		}
	}()

	wg.Wait()
}

// mockSessionStore implements session.SessionStore for testing AffinityMiddleware.
type mockSessionStore struct {
	sessions map[string]session.Session
	getErr   error
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]session.Session),
	}
}

func (m *mockSessionStore) Create(_ context.Context) (session.Session, error) {
	return nil, nil
}

func (m *mockSessionStore) Get(_ context.Context, id string) (session.Session, bool, error) {
	if m.getErr != nil {
		return nil, false, m.getErr
	}
	s, ok := m.sessions[id]
	return s, ok, nil
}

func (m *mockSessionStore) Delete(_ context.Context, _ string) error  { return nil }
func (m *mockSessionStore) Refresh(_ context.Context, _ string) error { return nil }
func (m *mockSessionStore) Close() error                              { return nil }

func TestAffinityMiddleware_SessionFound(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newMockSessionStore()

	// Add a mock session.
	store.sessions["test-sess-001"] = &testSession{id: "test-sess-001"}

	mw := AffinityMiddleware(ext, store)
	var gotSession bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := session.FromContext(r.Context())
		gotSession = ok && sess != nil && sess.ID() == "test-sess-001"
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Session-ID", "test-sess-001")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if !gotSession {
		t.Error("expected session to be attached to context")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAffinityMiddleware_NoSessionID(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newMockSessionStore()

	mw := AffinityMiddleware(ext, store)
	var gotSession bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := session.FromContext(r.Context())
		gotSession = ok
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if gotSession {
		t.Error("expected no session in context when no session ID")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAffinityMiddleware_SessionNotFound(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newMockSessionStore()

	mw := AffinityMiddleware(ext, store)
	var gotSession bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := session.FromContext(r.Context())
		gotSession = ok
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Session-ID", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if gotSession {
		t.Error("expected no session when session not found in store")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAffinityMiddleware_StoreError(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newMockSessionStore()
	store.getErr = context.DeadlineExceeded

	mw := AffinityMiddleware(ext, store)
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Session-ID", "some-sess")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if !called {
		t.Error("expected handler to be called even on store error (graceful degradation)")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAffinityRouter_EmptyUpstreams(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: nil,
	})

	result := ar.Route("any-session")
	if result != "" {
		t.Errorf("expected empty string for no upstreams, got %q", result)
	}
}

func TestAffinityRouter_RemoveNonexistentUpstream(t *testing.T) {
	t.Parallel()
	ext := transport.NewSessionExtractor()
	store := newTestMemStore(t)

	ar := NewAffinityRouter(AffinityConfig{
		Extractor: ext,
		Store:     store,
		Upstreams: []string{"a"},
	})

	// Should not panic.
	ar.RemoveUpstream("nonexistent")

	ups := ar.Upstreams()
	if len(ups) != 1 || ups[0] != "a" {
		t.Errorf("unexpected upstreams after removing nonexistent: %v", ups)
	}
}

func TestConsistentHash_CustomReplicas(t *testing.T) {
	t.Parallel()
	upstreams := []string{"x", "y", "z"}

	// With very low replicas, distribution may be uneven but should still be deterministic.
	result1 := consistentHash("test-session", upstreams, 1)
	result2 := consistentHash("test-session", upstreams, 1)
	if result1 != result2 {
		t.Errorf("inconsistent with low replicas: %q vs %q", result1, result2)
	}
}

func TestConsistentHash_ZeroReplicas(t *testing.T) {
	t.Parallel()
	// Zero replicas should default to 100.
	result := consistentHash("test", []string{"a", "b"}, 0)
	if result == "" {
		t.Error("expected non-empty result with zero replicas (should use default)")
	}
}

func TestConsistentHash_NegativeReplicas(t *testing.T) {
	t.Parallel()
	// Negative replicas should default to 100.
	result := consistentHash("test", []string{"a", "b"}, -1)
	if result == "" {
		t.Error("expected non-empty result with negative replicas")
	}
}
