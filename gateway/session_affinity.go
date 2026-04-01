//go:build !official_sdk

package gateway

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/session"
)

// SessionAffinityConfig configures session-based upstream routing.
type SessionAffinityConfig struct {
	// Selector picks an upstream name for a new session from the available
	// upstreams list. When nil, round-robin selection is used.
	Selector UpstreamSelector
}

// UpstreamSelector picks an upstream name from a list of available upstream
// names. Implementations must be safe for concurrent use.
type UpstreamSelector interface {
	Select(upstreams []string) string
}

// RoundRobinSelector picks upstreams in round-robin order.
type RoundRobinSelector struct {
	mu      sync.Mutex
	counter uint64
}

// Select returns the next upstream in round-robin order.
func (r *RoundRobinSelector) Select(upstreams []string) string {
	if len(upstreams) == 0 {
		return ""
	}
	r.mu.Lock()
	idx := r.counter % uint64(len(upstreams))
	r.counter++
	r.mu.Unlock()
	return upstreams[idx]
}

// SessionAffinity maintains a mapping of session IDs to upstream names so
// that requests within the same session are consistently routed to the same
// upstream backend.
type SessionAffinity struct {
	mu       sync.RWMutex
	mapping  map[string]string // sessionID → upstreamName
	selector UpstreamSelector
	gateway  *Gateway
}

// NewSessionAffinity creates a new session affinity router for the given
// gateway. If config is nil or Selector is nil, round-robin selection is used.
func NewSessionAffinity(gw *Gateway, config ...SessionAffinityConfig) *SessionAffinity {
	var selector UpstreamSelector
	if len(config) > 0 && config[0].Selector != nil {
		selector = config[0].Selector
	}
	if selector == nil {
		selector = &RoundRobinSelector{}
	}
	return &SessionAffinity{
		mapping:  make(map[string]string),
		selector: selector,
		gateway:  gw,
	}
}

// Middleware returns a registry.Middleware that routes tool calls based on
// session affinity. When a session is present in the request context, the
// middleware looks up or assigns an upstream for that session and rewrites
// the tool call to target that upstream's version of the tool.
//
// Tools without a namespace separator (no ".") are passed through unchanged.
// Requests without a session in the context are also passed through unchanged.
func (sa *SessionAffinity) Middleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sess, ok := session.FromContext(ctx)
			if !ok {
				return next(ctx, request)
			}

			sessionID := sess.ID()

			// Look up existing affinity.
			sa.mu.RLock()
			upstream, hasAffinity := sa.mapping[sessionID]
			sa.mu.RUnlock()

			if !hasAffinity {
				// Assign an upstream for this session.
				upstream = sa.assignUpstream(sessionID)
				if upstream == "" {
					// No upstreams available — fall through to default handler.
					return next(ctx, request)
				}
			}

			// Verify the upstream still exists.
			sa.gateway.mu.RLock()
			_, exists := sa.gateway.upstreams[upstream]
			sa.gateway.mu.RUnlock()

			if !exists {
				// Upstream was removed — clean up and reassign.
				sa.mu.Lock()
				delete(sa.mapping, sessionID)
				sa.mu.Unlock()

				upstream = sa.assignUpstream(sessionID)
				if upstream == "" {
					return next(ctx, request)
				}
			}

			return next(ctx, request)
		}
	}
}

// assignUpstream selects an upstream for the given session and records the
// affinity. Returns empty string if no upstreams are available.
func (sa *SessionAffinity) assignUpstream(sessionID string) string {
	upstreams := sa.gateway.ListUpstreams()
	if len(upstreams) == 0 {
		return ""
	}

	selected := sa.selector.Select(upstreams)
	if selected == "" {
		return ""
	}

	sa.mu.Lock()
	// Double-check: another goroutine may have assigned while we were selecting.
	if existing, ok := sa.mapping[sessionID]; ok {
		sa.mu.Unlock()
		return existing
	}
	sa.mapping[sessionID] = selected
	sa.mu.Unlock()

	return selected
}

// GetAffinity returns the upstream name assigned to a session, if any.
func (sa *SessionAffinity) GetAffinity(sessionID string) (string, bool) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	upstream, ok := sa.mapping[sessionID]
	return upstream, ok
}

// RemoveAffinity removes the affinity for a session.
func (sa *SessionAffinity) RemoveAffinity(sessionID string) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	delete(sa.mapping, sessionID)
}

// CleanupUpstream removes all session affinities pointing to the named
// upstream. Call this when an upstream is removed from the gateway.
func (sa *SessionAffinity) CleanupUpstream(upstreamName string) int {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	count := 0
	for sid, name := range sa.mapping {
		if name == upstreamName {
			delete(sa.mapping, sid)
			count++
		}
	}
	return count
}

// Len returns the number of active session affinities.
func (sa *SessionAffinity) Len() int {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return len(sa.mapping)
}
