package resilience

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// CacheEntry is a thread-safe, generic TTL cache holding a single value.
//
// Deprecated: CacheEntry has no notion of an input key — a second call
// with different arguments returns the first call's cached value until
// the TTL expires. It exists only for callers that genuinely have a
// single, keyless cacheable fetch. For anything keyed on tool arguments
// (the common case — e.g. per-input response caching in gateway proxying),
// use KeyedCache instead.
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

// keyedCacheEntry is one bounded, TTL-tracked slot in a KeyedCache.
type keyedCacheEntry[T any] struct {
	key     string
	value   T
	err     error
	fetchAt time.Time
	elem    *list.Element // this entry's node in the LRU list
}

// KeyedCache is a thread-safe, generic TTL cache keyed on an exact-match
// string key, bounded to a maximum number of entries with LRU eviction.
// Unlike CacheEntry, a KeyedCache holds many independent cached values —
// each distinct key gets its own TTL and its own slot, so two calls with
// different inputs never collide.
//
// The zero value is not usable; construct with NewKeyedCache.
type KeyedCache[T any] struct {
	mu       sync.Mutex
	ttl      time.Duration
	maxSize  int
	entries  map[string]*keyedCacheEntry[T]
	order    *list.List // front = most recently used, back = least recently used
	fetching map[string]*sync.Mutex
}

// NewKeyedCache creates a keyed cache with the given TTL and maximum entry
// count. maxSize <= 0 is treated as 1 (a cache with no bound is never what
// callers actually want — it silently becomes a memory leak).
func NewKeyedCache[T any](ttl time.Duration, maxSize int) *KeyedCache[T] {
	if maxSize <= 0 {
		maxSize = 1
	}
	return &KeyedCache[T]{
		ttl:      ttl,
		maxSize:  maxSize,
		entries:  make(map[string]*keyedCacheEntry[T]),
		order:    list.New(),
		fetching: make(map[string]*sync.Mutex),
	}
}

// CacheKey builds the exact-match cache key mcpkit uses for per-input
// response caching: toolName + ":" + sha256(canonicalized argument JSON).
// Canonicalization sorts object keys recursively so argument key order
// never changes the resulting key.
func CacheKey(toolName string, args map[string]any) string {
	canonical := canonicalizeJSONValue(args)
	// canonicalizeJSONValue never errors on a map[string]any built from
	// decoded JSON, but args may contain non-JSON-native Go values from a
	// caller; fall back to a best-effort marshal of the raw map on error.
	data, err := json.Marshal(canonical)
	if err != nil {
		data, _ = json.Marshal(args)
	}
	sum := sha256.Sum256(data)
	return toolName + ":" + hex.EncodeToString(sum[:])
}

// canonicalizeJSONValue recursively rewrites maps into sorted key/value
// pairs so that two arguments objects that differ only in key order marshal
// to identical bytes.
func canonicalizeJSONValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]canonicalKV, 0, len(keys))
		for _, k := range keys {
			out = append(out, canonicalKV{Key: k, Value: canonicalizeJSONValue(val[k])})
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = canonicalizeJSONValue(item)
		}
		return out
	default:
		return val
	}
}

// canonicalKV marshals as a fixed two-field object so key order in the
// original map cannot change the emitted byte sequence.
type canonicalKV struct {
	Key   string `json:"k"`
	Value any    `json:"v"`
}

// GetOrFetch returns the cached value for key if fresh, or calls fetchFn to
// populate it. Concurrent calls for the *same* key are serialized (only one
// fetch runs at a time per key); calls for different keys proceed
// independently. A successful fetch may evict the least-recently-used entry
// if the cache is at its size bound.
func (c *KeyedCache[T]) GetOrFetch(ctx context.Context, key string, fetchFn func(context.Context) (T, error)) (T, error) {
	if v, ok := c.get(key); ok {
		return v.value, v.err
	}

	keyLock := c.lockFor(key)
	keyLock.Lock()
	defer keyLock.Unlock()

	if v, ok := c.get(key); ok {
		return v.value, v.err
	}

	value, err := fetchFn(ctx)
	c.set(key, value, err)
	return value, err
}

// get returns the live (non-expired) entry for key, touching it as
// most-recently-used on hit.
func (c *KeyedCache[T]) get(key string) (*keyedCacheEntry[T], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.fetchAt) >= c.ttl {
		return nil, false
	}
	c.order.MoveToFront(e.elem)
	return e, true
}

// set inserts or replaces the entry for key, evicting the least-recently-used
// entry first if the cache is full.
func (c *KeyedCache[T]) set(key string, value T, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[key]; ok {
		e.value, e.err, e.fetchAt = value, err, time.Now()
		c.order.MoveToFront(e.elem)
		return
	}

	for len(c.entries) >= c.maxSize {
		back := c.order.Back()
		if back == nil {
			break
		}
		evictKey := back.Value.(string)
		delete(c.entries, evictKey)
		delete(c.fetching, evictKey)
		c.order.Remove(back)
	}

	elem := c.order.PushFront(key)
	c.entries[key] = &keyedCacheEntry[T]{key: key, value: value, err: err, fetchAt: time.Now(), elem: elem}
}

// lockFor returns the per-key fetch mutex, creating it if absent.
func (c *KeyedCache[T]) lockFor(key string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.fetching[key]
	if !ok {
		l = &sync.Mutex{}
		c.fetching[key] = l
	}
	return l
}

// Get returns the cached value for key without fetching, and without the
// combined GetOrFetch error-caching semantics — useful when a caller wants
// to populate the cache selectively (e.g. only on success) via Set instead
// of caching whatever a fetch function returns. Returns zero value and
// false if key is absent or expired.
func (c *KeyedCache[T]) Get(key string) (T, bool) {
	if e, ok := c.get(key); ok {
		return e.value, true
	}
	var zero T
	return zero, false
}

// Set manually populates the cache for key, evicting the least-recently-used
// entry first if the cache is at its size bound.
func (c *KeyedCache[T]) Set(key string, value T) {
	c.set(key, value, nil)
}

// Len returns the current number of live entries (including any not yet
// expired but past due for eviction on next access).
func (c *KeyedCache[T]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Invalidate removes a single key from the cache, if present.
func (c *KeyedCache[T]) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return
	}
	delete(c.entries, key)
	c.order.Remove(e.elem)
}
