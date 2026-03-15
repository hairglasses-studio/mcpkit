package roots

import (
	"context"
	"sync"
)

// CachedRootsClient wraps a RootsClient with a cache layer.
// Call Invalidate() when receiving notifications/roots/list_changed.
type CachedRootsClient struct {
	inner  RootsClient
	mu     sync.RWMutex
	cached []Root
	valid  bool
}

// NewCachedClient wraps an inner RootsClient with caching.
func NewCachedClient(inner RootsClient) *CachedRootsClient {
	return &CachedRootsClient{inner: inner}
}

// ListRoots returns cached roots if available, otherwise fetches from the inner client.
func (c *CachedRootsClient) ListRoots(ctx context.Context) ([]Root, error) {
	c.mu.RLock()
	if c.valid {
		roots := make([]Root, len(c.cached))
		copy(roots, c.cached)
		c.mu.RUnlock()
		return roots, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.valid {
		roots := make([]Root, len(c.cached))
		copy(roots, c.cached)
		return roots, nil
	}

	roots, err := c.inner.ListRoots(ctx)
	if err != nil {
		return nil, err
	}

	c.cached = make([]Root, len(roots))
	copy(c.cached, roots)
	c.valid = true

	return roots, nil
}

// Invalidate clears the cache, forcing the next ListRoots call to re-fetch.
func (c *CachedRootsClient) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.valid = false
	c.cached = nil
}
