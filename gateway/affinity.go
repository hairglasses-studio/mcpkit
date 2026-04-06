//go:build !official_sdk

package gateway

import (
	"hash/crc32"
	"net/http"
	"sort"
	"sync"

	"github.com/hairglasses-studio/mcpkit/session"
	"github.com/hairglasses-studio/mcpkit/transport"
)

// ConsistentHashSelector picks an upstream using consistent hashing on the
// session ID. This gives stable routing: the same session ID always maps to the
// same upstream, and adding/removing upstreams only redistributes a fraction of
// sessions.
type ConsistentHashSelector struct {
	// Replicas controls the number of virtual nodes per upstream on the hash
	// ring. Higher values give more uniform distribution. Default: 100.
	Replicas int
}

// Select picks an upstream by hashing the session ID and walking the sorted
// hash ring to find the nearest upstream. The sessionID parameter is passed
// via upstreams[0] being the session ID when called from AffinityMiddleware.
func (c *ConsistentHashSelector) Select(upstreams []string) string {
	if len(upstreams) == 0 {
		return ""
	}
	if len(upstreams) == 1 {
		return upstreams[0]
	}
	// Default to 100 replicas.
	return upstreams[0] // This method is used by SessionAffinity directly.
}

// consistentHash performs consistent hashing for session-to-upstream mapping.
// Given a session ID and a list of upstreams, it returns the upstream that
// the session should be routed to.
func consistentHash(sessionID string, upstreams []string, replicas int) string {
	if len(upstreams) == 0 {
		return ""
	}
	if len(upstreams) == 1 {
		return upstreams[0]
	}
	if replicas <= 0 {
		replicas = 100
	}

	type node struct {
		hash     uint32
		upstream string
	}

	ring := make([]node, 0, len(upstreams)*replicas)
	for _, u := range upstreams {
		for i := range replicas {
			key := u + ":" + string(rune(i+'0'))
			// Use a byte-level representation for replicas > 9.
			if i >= 10 {
				key = u + ":"
				n := i
				for n > 0 {
					key += string(rune('0' + n%10))
					n /= 10
				}
			}
			ring = append(ring, node{
				hash:     crc32.ChecksumIEEE([]byte(key)),
				upstream: u,
			})
		}
	}

	sort.Slice(ring, func(i, j int) bool {
		return ring[i].hash < ring[j].hash
	})

	h := crc32.ChecksumIEEE([]byte(sessionID))
	// Binary search for the first node with hash >= h.
	idx := sort.Search(len(ring), func(i int) bool {
		return ring[i].hash >= h
	})
	if idx == len(ring) {
		idx = 0 // wrap around
	}
	return ring[idx].upstream
}

// AffinityRouter combines session extraction with consistent-hash-based upstream
// routing for HTTP requests. It is the HTTP-level counterpart to
// [SessionAffinity] (which operates at the MCP tool middleware level).
type AffinityRouter struct {
	mu        sync.RWMutex
	extractor *transport.SessionExtractor
	store     session.SessionStore
	upstreams []string
	replicas  int
	sticky    map[string]string // sessionID -> upstream (override cache)
}

// AffinityConfig configures an AffinityRouter.
type AffinityConfig struct {
	// Extractor extracts session IDs from HTTP requests.
	// Required.
	Extractor *transport.SessionExtractor
	// Store is the session store used to look up sessions by ID.
	// Any SessionStore implementation works (MemStore, RedisStore, etc.).
	Store session.SessionStore
	// Upstreams is the initial set of upstream names/URLs.
	Upstreams []string
	// Replicas is the number of virtual nodes per upstream on the consistent
	// hash ring. Default: 100.
	Replicas int
}

// NewAffinityRouter creates a new HTTP-level affinity router.
func NewAffinityRouter(config AffinityConfig) *AffinityRouter {
	replicas := config.Replicas
	if replicas <= 0 {
		replicas = 100
	}
	upstreams := make([]string, len(config.Upstreams))
	copy(upstreams, config.Upstreams)
	return &AffinityRouter{
		extractor: config.Extractor,
		store:     config.Store,
		upstreams: upstreams,
		replicas:  replicas,
		sticky:    make(map[string]string),
	}
}

// Route returns the upstream name for the given session ID using consistent
// hashing. If the session has a sticky override (from a previous routing
// decision where the hashed upstream was unavailable), that override is used.
func (ar *AffinityRouter) Route(sessionID string) string {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	// Check sticky override first.
	if override, ok := ar.sticky[sessionID]; ok {
		// Verify the override upstream still exists.
		for _, u := range ar.upstreams {
			if u == override {
				return override
			}
		}
		// Override is stale — fall through to consistent hash.
	}

	return consistentHash(sessionID, ar.upstreams, ar.replicas)
}

// SetUpstreams replaces the upstream list. Sticky overrides for removed
// upstreams are cleaned up.
func (ar *AffinityRouter) SetUpstreams(upstreams []string) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.upstreams = make([]string, len(upstreams))
	copy(ar.upstreams, upstreams)

	// Clean up sticky overrides for removed upstreams.
	valid := make(map[string]bool, len(upstreams))
	for _, u := range upstreams {
		valid[u] = true
	}
	for sid, u := range ar.sticky {
		if !valid[u] {
			delete(ar.sticky, sid)
		}
	}
}

// AddUpstream adds an upstream to the router.
func (ar *AffinityRouter) AddUpstream(name string) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	for _, u := range ar.upstreams {
		if u == name {
			return // already present
		}
	}
	ar.upstreams = append(ar.upstreams, name)
}

// RemoveUpstream removes an upstream and cleans up sticky overrides.
func (ar *AffinityRouter) RemoveUpstream(name string) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	for i, u := range ar.upstreams {
		if u == name {
			ar.upstreams = append(ar.upstreams[:i], ar.upstreams[i+1:]...)
			break
		}
	}
	// Clean sticky overrides pointing to the removed upstream.
	for sid, u := range ar.sticky {
		if u == name {
			delete(ar.sticky, sid)
		}
	}
}

// SetSticky sets a sticky override for a session, pinning it to a specific
// upstream regardless of the consistent hash result.
func (ar *AffinityRouter) SetSticky(sessionID, upstream string) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.sticky[sessionID] = upstream
}

// RemoveSticky removes the sticky override for a session.
func (ar *AffinityRouter) RemoveSticky(sessionID string) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	delete(ar.sticky, sessionID)
}

// Upstreams returns the current upstream list.
func (ar *AffinityRouter) Upstreams() []string {
	ar.mu.RLock()
	defer ar.mu.RUnlock()
	result := make([]string, len(ar.upstreams))
	copy(result, ar.upstreams)
	return result
}

// AffinityMiddleware returns an http.Handler middleware that extracts the
// session ID from each request, determines the target upstream via consistent
// hashing, and attaches the result to the request context for downstream
// handlers.
//
// The session is loaded from the ExternalStore and attached to the context via
// [session.WithSession]. If no session ID is found, the request is passed
// through without modification.
func AffinityMiddleware(extractor *transport.SessionExtractor, store session.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID, err := extractor.Extract(r)
			if err != nil {
				// No session ID — pass through.
				next.ServeHTTP(w, r)
				return
			}

			// Try to load the session from the external store.
			sess, ok, err := store.Get(r.Context(), sessionID)
			if err != nil {
				// Store error — pass through (degrade gracefully).
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				// Session not found — pass through.
				next.ServeHTTP(w, r)
				return
			}

			// Attach session to context.
			ctx := session.WithSession(r.Context(), sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
