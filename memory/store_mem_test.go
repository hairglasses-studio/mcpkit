package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewInMemoryStore(t *testing.T) {
	s := NewInMemoryStore()
	if s == nil {
		t.Fatal("NewInMemoryStore returned nil")
	}
	if s.entries == nil {
		t.Fatal("entries map not initialized")
	}
}

func TestSetGet_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entry := MemoryEntry{
		Key:   "test-key",
		Value: "test-value",
		Tier:  TierSemantic,
		Tags:  []string{"a", "b"},
	}

	if err := s.Set(ctx, entry); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok, err := s.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get returned false for existing key")
	}
	if got.Key != "test-key" {
		t.Errorf("Key = %q, want %q", got.Key, "test-key")
	}
	if got.Value != "test-value" {
		t.Errorf("Value = %q, want %q", got.Value, "test-value")
	}
	if got.Tier != TierSemantic {
		t.Errorf("Tier = %q, want %q", got.Tier, TierSemantic)
	}
}

func TestSet_PreservesCreatedAtOnUpdate(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entry := MemoryEntry{Key: "k", Value: "v1", Tier: TierEpisodic}
	if err := s.Set(ctx, entry); err != nil {
		t.Fatalf("first Set: %v", err)
	}

	got1, _, _ := s.Get(ctx, "k")
	createdAt := got1.CreatedAt

	// Small sleep to ensure UpdatedAt differs from CreatedAt
	time.Sleep(2 * time.Millisecond)

	entry.Value = "v2"
	if err := s.Set(ctx, entry); err != nil {
		t.Fatalf("second Set: %v", err)
	}

	got2, _, _ := s.Get(ctx, "k")
	if !got2.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt changed on update: was %v, now %v", createdAt, got2.CreatedAt)
	}
	if got2.Value != "v2" {
		t.Errorf("Value after update = %q, want %q", got2.Value, "v2")
	}
}

func TestSet_IncrementsVersionOnUpdate(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entry := MemoryEntry{Key: "k", Value: "v1", Tier: TierEpisodic}
	if err := s.Set(ctx, entry); err != nil {
		t.Fatalf("first Set: %v", err)
	}
	got1, _, _ := s.Get(ctx, "k")
	if got1.Version != 1 {
		t.Errorf("initial Version = %d, want 1", got1.Version)
	}

	entry.Value = "v2"
	if err := s.Set(ctx, entry); err != nil {
		t.Fatalf("second Set: %v", err)
	}
	got2, _, _ := s.Get(ctx, "k")
	if got2.Version != 2 {
		t.Errorf("Version after update = %d, want 2", got2.Version)
	}
}

func TestGet_MissingKey(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_, ok, err := s.Get(ctx, "no-such-key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if ok {
		t.Error("Get returned true for missing key")
	}
}

func TestGet_LazilyRemovesExpiredEntry(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	exp := time.Now().Add(10 * time.Millisecond)
	entry := MemoryEntry{
		Key:       "expiring",
		Value:     "soon",
		Tier:      TierEpisodic,
		ExpiresAt: &exp,
	}
	if err := s.Set(ctx, entry); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Confirm it's there before expiry
	_, ok, _ := s.Get(ctx, "expiring")
	if !ok {
		t.Fatal("entry should be present before expiry")
	}

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	_, ok, err := s.Get(ctx, "expiring")
	if err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if ok {
		t.Error("Get returned true for expired entry")
	}

	// Confirm the entry is actually gone from the map
	s.mu.Lock()
	_, exists := s.entries["expiring"]
	s.mu.Unlock()
	if exists {
		t.Error("expired entry was not lazily removed from map")
	}
}

func TestDelete_RemovesEntry(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entry := MemoryEntry{Key: "del-me", Value: "val", Tier: TierSemantic}
	_ = s.Set(ctx, entry)

	if err := s.Delete(ctx, "del-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, ok, _ := s.Get(ctx, "del-me")
	if ok {
		t.Error("entry still present after Delete")
	}
}

func TestDelete_NoopForMissingKey(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	// Should not panic or return an error
	if err := s.Delete(ctx, "phantom"); err != nil {
		t.Errorf("Delete of missing key returned error: %v", err)
	}
}

func TestList_TierFilter(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "e1", Value: "v", Tier: TierEpisodic})
	_ = s.Set(ctx, MemoryEntry{Key: "s1", Value: "v", Tier: TierSemantic})
	_ = s.Set(ctx, MemoryEntry{Key: "p1", Value: "v", Tier: TierProcedural})

	results, err := s.List(ctx, ListOptions{Tier: TierSemantic})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 1 || results[0].Key != "s1" {
		t.Errorf("expected [s1], got %v", keys(results))
	}
}

func TestList_PrefixFilter(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "user:alice", Value: "v", Tier: TierSemantic})
	_ = s.Set(ctx, MemoryEntry{Key: "user:bob", Value: "v", Tier: TierSemantic})
	_ = s.Set(ctx, MemoryEntry{Key: "session:123", Value: "v", Tier: TierEpisodic})

	results, err := s.List(ctx, ListOptions{Prefix: "user:"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d: %v", len(results), keys(results))
	}
	for _, r := range results {
		if r.Key != "user:alice" && r.Key != "user:bob" {
			t.Errorf("unexpected key in prefix results: %q", r.Key)
		}
	}
}

func TestList_TagsFilter(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "k1", Value: "v", Tier: TierSemantic, Tags: []string{"foo", "bar"}})
	_ = s.Set(ctx, MemoryEntry{Key: "k2", Value: "v", Tier: TierSemantic, Tags: []string{"foo"}})
	_ = s.Set(ctx, MemoryEntry{Key: "k3", Value: "v", Tier: TierSemantic, Tags: []string{"bar"}})

	results, err := s.List(ctx, ListOptions{Tags: []string{"foo", "bar"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 1 || results[0].Key != "k1" {
		t.Errorf("expected [k1], got %v", keys(results))
	}
}

func TestList_Limit(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	for i := range 5 {
		_ = s.Set(ctx, MemoryEntry{Key: string(rune('a' + i)), Value: "v", Tier: TierEpisodic})
	}

	results, err := s.List(ctx, ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with Limit=2, got %d", len(results))
	}
}

func TestList_SkipsExpiredEntries(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	exp := time.Now().Add(10 * time.Millisecond)
	_ = s.Set(ctx, MemoryEntry{Key: "live", Value: "v", Tier: TierSemantic})
	_ = s.Set(ctx, MemoryEntry{Key: "dead", Value: "v", Tier: TierSemantic, ExpiresAt: &exp})

	time.Sleep(20 * time.Millisecond)

	results, err := s.List(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, r := range results {
		if r.Key == "dead" {
			t.Error("List returned expired entry")
		}
	}
	found := false
	for _, r := range results {
		if r.Key == "live" {
			found = true
		}
	}
	if !found {
		t.Error("List did not return live entry")
	}
}

func TestSearch_MatchesKey(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "golang-tips", Value: "some notes", Tier: TierSemantic})
	_ = s.Set(ctx, MemoryEntry{Key: "python-tips", Value: "other notes", Tier: TierSemantic})

	results, err := s.Search(ctx, "golang", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].Key != "golang-tips" {
		t.Errorf("expected [golang-tips], got %v", keys(results))
	}
}

func TestSearch_MatchesValue(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "entry1", Value: "important insight", Tier: TierSemantic})
	_ = s.Set(ctx, MemoryEntry{Key: "entry2", Value: "unrelated content", Tier: TierSemantic})

	results, err := s.Search(ctx, "insight", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].Key != "entry1" {
		t.Errorf("expected [entry1], got %v", keys(results))
	}
}

func TestSearch_MatchesTag(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "k1", Value: "v", Tier: TierSemantic, Tags: []string{"machine-learning"}})
	_ = s.Set(ctx, MemoryEntry{Key: "k2", Value: "v", Tier: TierSemantic, Tags: []string{"databases"}})

	results, err := s.Search(ctx, "machine", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].Key != "k1" {
		t.Errorf("expected [k1], got %v", keys(results))
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "GoLang-Notes", Value: "Advanced Go Programming", Tier: TierSemantic})

	for _, q := range []string{"golang", "GOLANG", "GoLang", "advanced go"} {
		results, err := s.Search(ctx, q, SearchOptions{})
		if err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
		if len(results) == 0 {
			t.Errorf("Search(%q) returned no results, expected a match", q)
		}
	}
}

func TestSearch_TierAndTagsFilters(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "alpha", Value: "needle", Tier: TierSemantic, Tags: []string{"important"}})
	_ = s.Set(ctx, MemoryEntry{Key: "beta", Value: "needle", Tier: TierEpisodic, Tags: []string{"important"}})
	_ = s.Set(ctx, MemoryEntry{Key: "gamma", Value: "needle", Tier: TierSemantic, Tags: []string{"other"}})

	results, err := s.Search(ctx, "needle", SearchOptions{
		Tier: TierSemantic,
		Tags: []string{"important"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].Key != "alpha" {
		t.Errorf("expected [alpha], got %v", keys(results))
	}
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	const goroutines = 20
	const ops = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range ops {
				key := string(rune('a' + (id % 5)))
				switch j % 3 {
				case 0:
					_ = s.Set(ctx, MemoryEntry{
						Key:   key,
						Value: "value",
						Tier:  TierEpisodic,
					})
				case 1:
					_, _, _ = s.Get(ctx, key)
				case 2:
					_ = s.Delete(ctx, key)
				}
			}
		}(i)
	}

	wg.Wait()
	// No race detector failure = test passes
}

// keys is a helper that extracts keys from a slice of MemoryEntry for readable errors.
func keys(entries []MemoryEntry) []string {
	ks := make([]string, len(entries))
	for i, e := range entries {
		ks[i] = e.Key
	}
	return ks
}
