// Package memory provides agent memory with pluggable storage backends.
//
// It follows the registry pattern: a MemoryRegistry manages named Store
// implementations. Stores can be retrieved by name or via context injection
// using the Middleware helper.
package memory

import (
	"context"
	"sync"
	"time"
)

// Tier classifies the type of memory entry.
type Tier string

const (
	// TierEpisodic stores event-specific memories (what happened when).
	TierEpisodic Tier = "episodic"
	// TierSemantic stores factual/conceptual knowledge.
	TierSemantic Tier = "semantic"
	// TierProcedural stores how-to knowledge and learned behaviors.
	TierProcedural Tier = "procedural"
)

// MemoryEntry is a single record stored in a memory Store.
type MemoryEntry struct {
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Tier      Tier              `json:"tier"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
	Version   int               `json:"version"`
}

// ListOptions controls filtering when listing entries from a Store.
type ListOptions struct {
	// Tier filters entries to a specific memory tier. Empty means all tiers.
	Tier Tier
	// Tags filters entries that have all specified tags.
	Tags []string
	// Prefix filters entries whose key starts with this string.
	Prefix string
	// Limit caps the number of results. 0 means unlimited.
	Limit int
}

// SearchOptions controls filtering when searching entries in a Store.
type SearchOptions struct {
	// Tier filters results to a specific memory tier. Empty means all tiers.
	Tier Tier
	// Tags filters results that have all specified tags.
	Tags []string
	// Limit caps the number of results. 0 means unlimited.
	Limit int
}

// Store is the interface that memory backend implementations satisfy.
type Store interface {
	// Get retrieves an entry by key. Returns (entry, true, nil) if found,
	// (zero, false, nil) if not found or expired, or (zero, false, err) on error.
	Get(ctx context.Context, key string) (MemoryEntry, bool, error)
	// Set creates or updates an entry.
	Set(ctx context.Context, entry MemoryEntry) error
	// Delete removes an entry by key. No error if not found.
	Delete(ctx context.Context, key string) error
	// List returns entries matching the given options.
	List(ctx context.Context, opts ListOptions) ([]MemoryEntry, error)
	// Search performs a text search across entries, returning matches.
	Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryEntry, error)
}

// Config configures a MemoryRegistry.
type Config struct {
	// DefaultStore is registered under the name "default" when provided.
	DefaultStore Store
}

// MemoryRegistry manages named Store instances.
type MemoryRegistry struct {
	mu     sync.RWMutex
	stores map[string]Store
}

// NewMemoryRegistry creates a new MemoryRegistry. If a Config with a
// DefaultStore is provided, that store is registered under the name "default".
func NewMemoryRegistry(configs ...Config) *MemoryRegistry {
	r := &MemoryRegistry{stores: make(map[string]Store)}
	if len(configs) > 0 && configs[0].DefaultStore != nil {
		r.stores["default"] = configs[0].DefaultStore
	}
	return r
}

// Register adds a Store under the given name, replacing any existing entry.
func (r *MemoryRegistry) Register(name string, store Store) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stores[name] = store
}

// Get retrieves the Store registered under name. Returns (store, true) if
// found, (nil, false) otherwise.
func (r *MemoryRegistry) Get(name string) (Store, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stores[name]
	return s, ok
}

// Default returns the Store registered as "default", or nil if none.
func (r *MemoryRegistry) Default() Store {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stores["default"]
}

// List returns the names of all registered stores.
func (r *MemoryRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.stores))
	for name := range r.stores {
		names = append(names, name)
	}
	return names
}
