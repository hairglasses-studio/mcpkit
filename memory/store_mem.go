package memory

import (
	"context"
	"strings"
	"sync"
	"time"
)

// InMemoryStore is a thread-safe, in-process implementation of Store.
// It stores entries in a Go map and supports expiration, versioning,
// prefix/tag/tier filtering, and case-insensitive text search.
type InMemoryStore struct {
	mu      sync.RWMutex
	entries map[string]MemoryEntry
}

// NewInMemoryStore creates an empty InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		entries: make(map[string]MemoryEntry),
	}
}

// Get retrieves an entry by key. Expired entries are lazily removed and
// reported as not found.
func (s *InMemoryStore) Get(_ context.Context, key string) (MemoryEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return MemoryEntry{}, false, nil
	}
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		delete(s.entries, key)
		return MemoryEntry{}, false, nil
	}
	return entry, true, nil
}

// Set creates or updates an entry. On creation, CreatedAt is set to now;
// on update, the existing CreatedAt is preserved. UpdatedAt is always set
// to now and Version is incremented on every call.
func (s *InMemoryStore) Set(_ context.Context, entry MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	existing, exists := s.entries[entry.Key]
	if exists {
		entry.CreatedAt = existing.CreatedAt
		entry.Version = existing.Version + 1
	} else {
		entry.CreatedAt = now
		entry.Version = 1
	}
	entry.UpdatedAt = now

	s.entries[entry.Key] = entry
	return nil
}

// Delete removes an entry by key. No error is returned if the key does not exist.
func (s *InMemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

// List returns entries that match all criteria in opts. Expired entries are
// skipped. If opts.Limit > 0, at most that many entries are returned.
func (s *InMemoryStore) List(_ context.Context, opts ListOptions) ([]MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var results []MemoryEntry

	for _, entry := range s.entries {
		// Skip expired
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			delete(s.entries, entry.Key)
			continue
		}
		// Tier filter
		if opts.Tier != "" && entry.Tier != opts.Tier {
			continue
		}
		// Prefix filter
		if opts.Prefix != "" && !strings.HasPrefix(entry.Key, opts.Prefix) {
			continue
		}
		// Tags filter — all specified tags must be present
		if len(opts.Tags) > 0 && !hasAllTags(entry.Tags, opts.Tags) {
			continue
		}
		results = append(results, entry)
		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}

// Search performs a case-insensitive substring search across Key, Value, and
// Tags of each entry. Tier and Tags filters from opts are also applied. Expired
// entries are skipped.
func (s *InMemoryStore) Search(_ context.Context, query string, opts SearchOptions) ([]MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	lq := strings.ToLower(query)
	var results []MemoryEntry

	for _, entry := range s.entries {
		// Skip expired
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			delete(s.entries, entry.Key)
			continue
		}
		// Tier filter
		if opts.Tier != "" && entry.Tier != opts.Tier {
			continue
		}
		// Tags filter
		if len(opts.Tags) > 0 && !hasAllTags(entry.Tags, opts.Tags) {
			continue
		}
		// Text match
		if !matchesQuery(entry, lq) {
			continue
		}
		results = append(results, entry)
		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}

// hasAllTags reports whether entryTags contains every tag in required.
func hasAllTags(entryTags, required []string) bool {
	tagSet := make(map[string]struct{}, len(entryTags))
	for _, t := range entryTags {
		tagSet[t] = struct{}{}
	}
	for _, req := range required {
		if _, ok := tagSet[req]; !ok {
			return false
		}
	}
	return true
}

// matchesQuery reports whether entry's key, value, or any tag contains lq
// (which must already be lower-cased).
func matchesQuery(entry MemoryEntry, lq string) bool {
	if strings.Contains(strings.ToLower(entry.Key), lq) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Value), lq) {
		return true
	}
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), lq) {
			return true
		}
	}
	return false
}
