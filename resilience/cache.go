package resilience

import (
	"context"
	"sync"
	"time"
)

// CacheEntry is a thread-safe, generic TTL cache holding a single value.
// It caches the result of an expensive fetch function and returns the
// cached value on subsequent calls until the TTL expires.
type CacheEntry[T any] struct {
	mu       sync.RWMutex
	value    T
	err      error
	fetched  bool
	fetchAt  time.Time
	ttl      time.Duration
	fetching sync.Mutex
}

// NewCache creates a cache entry with the given TTL.
func NewCache[T any](ttl time.Duration) *CacheEntry[T] {
	return &CacheEntry[T]{ttl: ttl}
}

// GetOrFetch returns the cached value if fresh, or calls fetchFn to populate it.
// Concurrent calls during a fetch are serialized — only one fetch runs at a time.
func (e *CacheEntry[T]) GetOrFetch(ctx context.Context, fetchFn func(context.Context) (T, error)) (T, error) {
	e.mu.RLock()
	if e.fetched && time.Since(e.fetchAt) < e.ttl {
		v, err := e.value, e.err
		e.mu.RUnlock()
		return v, err
	}
	e.mu.RUnlock()

	e.fetching.Lock()
	defer e.fetching.Unlock()

	e.mu.RLock()
	if e.fetched && time.Since(e.fetchAt) < e.ttl {
		v, err := e.value, e.err
		e.mu.RUnlock()
		return v, err
	}
	e.mu.RUnlock()

	value, err := fetchFn(ctx)

	e.mu.Lock()
	e.value = value
	e.err = err
	e.fetched = true
	e.fetchAt = time.Now()
	e.mu.Unlock()

	return value, err
}

// Get returns the cached value without fetching. Returns zero value and false
// if the cache is empty or expired.
func (e *CacheEntry[T]) Get() (T, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.fetched && time.Since(e.fetchAt) < e.ttl {
		return e.value, true
	}
	var zero T
	return zero, false
}

// Invalidate clears the cache, forcing the next GetOrFetch to call fetchFn.
func (e *CacheEntry[T]) Invalidate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fetched = false
}

// Set manually populates the cache with a value.
func (e *CacheEntry[T]) Set(value T) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.value = value
	e.err = nil
	e.fetched = true
	e.fetchAt = time.Now()
}
