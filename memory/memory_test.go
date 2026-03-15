package memory

import (
	"context"
	"sort"
	"testing"
	"time"
)

func TestInMemoryStore_SetGet(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entry := MemoryEntry{
		Key:   "test-key",
		Value: "test-value",
		Tier:  TierEpisodic,
	}
	if err := s.Set(ctx, entry); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok, err := s.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected entry to be found")
	}
	if got.Value != "test-value" {
		t.Errorf("Value = %q, want %q", got.Value, "test-value")
	}
	if got.Tier != TierEpisodic {
		t.Errorf("Tier = %q, want %q", got.Tier, TierEpisodic)
	}
}

func TestInMemoryStore_GetNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_, ok, err := s.Get(ctx, "no-such-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatal("expected entry not found")
	}
}

func TestInMemoryStore_Delete(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entry := MemoryEntry{Key: "del-key", Value: "v", Tier: TierSemantic}
	_ = s.Set(ctx, entry)

	if err := s.Delete(ctx, "del-key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, ok, err := s.Get(ctx, "del-key")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if ok {
		t.Fatal("expected entry gone after delete")
	}

	// Deleting non-existent key should not error
	if err := s.Delete(ctx, "never-existed"); err != nil {
		t.Errorf("Delete non-existent: unexpected error: %v", err)
	}
}

func TestInMemoryStore_List_FilterByTier(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entries := []MemoryEntry{
		{Key: "a", Value: "1", Tier: TierEpisodic},
		{Key: "b", Value: "2", Tier: TierSemantic},
		{Key: "c", Value: "3", Tier: TierEpisodic},
		{Key: "d", Value: "4", Tier: TierProcedural},
	}
	for _, e := range entries {
		_ = s.Set(ctx, e)
	}

	got, err := s.List(ctx, ListOptions{Tier: TierEpisodic})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 episodic entries, got %d", len(got))
	}
	for _, e := range got {
		if e.Tier != TierEpisodic {
			t.Errorf("unexpected tier %q in results", e.Tier)
		}
	}
}

func TestInMemoryStore_List_FilterByTags(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "a", Value: "1", Tier: TierSemantic, Tags: []string{"foo", "bar"}})
	_ = s.Set(ctx, MemoryEntry{Key: "b", Value: "2", Tier: TierSemantic, Tags: []string{"foo"}})
	_ = s.Set(ctx, MemoryEntry{Key: "c", Value: "3", Tier: TierSemantic, Tags: []string{"bar"}})
	_ = s.Set(ctx, MemoryEntry{Key: "d", Value: "4", Tier: TierSemantic, Tags: []string{"foo", "bar", "baz"}})

	// Both foo and bar must be present
	got, err := s.List(ctx, ListOptions{Tags: []string{"foo", "bar"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries with foo+bar, got %d", len(got))
	}
}

func TestInMemoryStore_List_FilterByPrefix(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "session:1", Value: "a", Tier: TierEpisodic})
	_ = s.Set(ctx, MemoryEntry{Key: "session:2", Value: "b", Tier: TierEpisodic})
	_ = s.Set(ctx, MemoryEntry{Key: "fact:1", Value: "c", Tier: TierSemantic})

	got, err := s.List(ctx, ListOptions{Prefix: "session:"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 session entries, got %d", len(got))
	}
	for _, e := range got {
		if len(e.Key) < 8 || e.Key[:8] != "session:" {
			t.Errorf("unexpected key %q with session: prefix filter", e.Key)
		}
	}
}

func TestInMemoryStore_List_Limit(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	for i := 0; i < 10; i++ {
		_ = s.Set(ctx, MemoryEntry{Key: string(rune('a'+i)), Value: "v", Tier: TierSemantic})
	}

	got, err := s.List(ctx, ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 entries with Limit=3, got %d", len(got))
	}
}

func TestInMemoryStore_Search(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	_ = s.Set(ctx, MemoryEntry{Key: "golang-intro", Value: "Go is a compiled language", Tier: TierSemantic, Tags: []string{"programming"}})
	_ = s.Set(ctx, MemoryEntry{Key: "python-intro", Value: "Python is interpreted", Tier: TierSemantic, Tags: []string{"programming"}})
	_ = s.Set(ctx, MemoryEntry{Key: "recipe-pasta", Value: "Boil water, add pasta", Tier: TierProcedural, Tags: []string{"cooking"}})

	// Search by value content
	got, err := s.Search(ctx, "compiled", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Key != "golang-intro" {
		t.Errorf("expected golang-intro, got %v", got)
	}

	// Case-insensitive key search
	got, err = s.Search(ctx, "GOLANG", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Key != "golang-intro" {
		t.Errorf("case-insensitive key search: expected golang-intro, got %v", got)
	}

	// Search by tag
	got, err = s.Search(ctx, "cooking", SearchOptions{})
	if err != nil {
		t.Fatalf("Search by tag: %v", err)
	}
	if len(got) != 1 || got[0].Key != "recipe-pasta" {
		t.Errorf("tag search: expected recipe-pasta, got %v", got)
	}

	// Tier filter combined with search
	got, err = s.Search(ctx, "programming", SearchOptions{Tier: TierSemantic})
	if err != nil {
		t.Fatalf("Search with tier filter: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 semantic programming entries, got %d", len(got))
	}
}

func TestInMemoryStore_Search_Limit(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	for i := 0; i < 5; i++ {
		_ = s.Set(ctx, MemoryEntry{Key: string(rune('a'+i)), Value: "match", Tier: TierSemantic})
	}

	got, err := s.Search(ctx, "match", SearchOptions{Limit: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 results with Limit=2, got %d", len(got))
	}
}

func TestInMemoryStore_Expiration(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	past := time.Now().Add(-1 * time.Second)
	entry := MemoryEntry{
		Key:       "expires-key",
		Value:     "gone",
		Tier:      TierEpisodic,
		ExpiresAt: &past,
	}
	_ = s.Set(ctx, entry)

	// Get should return not found for expired entry
	_, ok, err := s.Get(ctx, "expires-key")
	if err != nil {
		t.Fatalf("Get expired: %v", err)
	}
	if ok {
		t.Fatal("expected expired entry to return not found")
	}

	// Verify lazy delete also removes from List
	results, err := s.List(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, r := range results {
		if r.Key == "expires-key" {
			t.Error("expired entry should not appear in List results")
		}
	}
}

func TestInMemoryStore_Version(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	entry := MemoryEntry{Key: "v-key", Value: "v1", Tier: TierSemantic}
	_ = s.Set(ctx, entry)

	got, _, _ := s.Get(ctx, "v-key")
	if got.Version != 1 {
		t.Errorf("initial Version = %d, want 1", got.Version)
	}

	entry.Value = "v2"
	_ = s.Set(ctx, entry)
	got, _, _ = s.Get(ctx, "v-key")
	if got.Version != 2 {
		t.Errorf("after first update Version = %d, want 2", got.Version)
	}

	entry.Value = "v3"
	_ = s.Set(ctx, entry)
	got, _, _ = s.Get(ctx, "v-key")
	if got.Version != 3 {
		t.Errorf("after second update Version = %d, want 3", got.Version)
	}

	// CreatedAt should be preserved across updates
	first, _, _ := s.Get(ctx, "v-key")
	entry.Value = "v4"
	_ = s.Set(ctx, entry)
	second, _, _ := s.Get(ctx, "v-key")
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Error("CreatedAt should not change on update")
	}
}

func TestMemoryRegistry_RegisterAndGet(t *testing.T) {
	r := NewMemoryRegistry()
	store := NewInMemoryStore()
	r.Register("mystore", store)

	got, ok := r.Get("mystore")
	if !ok {
		t.Fatal("expected store to be found")
	}
	if got != store {
		t.Error("retrieved store does not match registered store")
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Fatal("expected nonexistent store to not be found")
	}
}

func TestMemoryRegistry_Default(t *testing.T) {
	store := NewInMemoryStore()
	r := NewMemoryRegistry(Config{DefaultStore: store})

	got := r.Default()
	if got == nil {
		t.Fatal("Default() returned nil")
	}
	if got != store {
		t.Error("Default() store does not match configured store")
	}
}

func TestMemoryRegistry_Default_NoneConfigured(t *testing.T) {
	r := NewMemoryRegistry()
	if r.Default() != nil {
		t.Error("Default() should return nil when no default configured")
	}
}

func TestMemoryRegistry_List(t *testing.T) {
	r := NewMemoryRegistry()
	r.Register("alpha", NewInMemoryStore())
	r.Register("beta", NewInMemoryStore())
	r.Register("gamma", NewInMemoryStore())

	names := r.List()
	sort.Strings(names)

	if len(names) != 3 {
		t.Fatalf("expected 3 stores, got %d", len(names))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestMemoryRegistry_RegisterReplaces(t *testing.T) {
	r := NewMemoryRegistry()
	s1 := NewInMemoryStore()
	s2 := NewInMemoryStore()

	r.Register("store", s1)
	r.Register("store", s2)

	got, _ := r.Get("store")
	if got != s2 {
		t.Error("second Register should replace the first")
	}
}
