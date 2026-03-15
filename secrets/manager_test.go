package secrets

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockProvider implements SecretProvider and HealthChecker for testing.
type mockProvider struct {
	mu          sync.Mutex
	name        string
	priority    int
	available   bool
	secrets     map[string]*Secret
	closeCalled bool
	closeErr    error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Get(_ context.Context, key string) (*Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.secrets[key]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return s, nil
}

func (m *mockProvider) List(_ context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockProvider) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.secrets[key]
	return ok, nil
}

func (m *mockProvider) Priority() int    { return m.priority }
func (m *mockProvider) IsAvailable() bool { return m.available }

func (m *mockProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	return m.closeErr
}

func (m *mockProvider) Health(_ context.Context) ProviderHealth {
	return ProviderHealth{
		Name:      m.name,
		Available: m.available,
		LastCheck: time.Now(),
	}
}

func (m *mockProvider) setSecret(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets[key] = &Secret{Key: key, Value: value, Source: m.name}
}

func newMockProvider(name string, priority int) *mockProvider {
	return &mockProvider{
		name:      name,
		priority:  priority,
		available: true,
		secrets:   make(map[string]*Secret),
	}
}

// --- Tests ---

func TestNewManager_Defaults(t *testing.T) {
	m := NewManager()
	if m.cacheTTL != DefaultCacheTTL {
		t.Errorf("expected default cacheTTL %v, got %v", DefaultCacheTTL, m.cacheTTL)
	}
	if len(m.providers) != 0 {
		t.Errorf("expected zero providers, got %d", len(m.providers))
	}
}

func TestManager_Get_FromProvider(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("api_key", "secret123")

	m := NewManager(WithProviders(p))
	secret, err := m.Get(context.Background(), "api_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret.Value != "secret123" {
		t.Errorf("expected value %q, got %q", "secret123", secret.Value)
	}
}

func TestManager_Get_CacheHit(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("mykey", "myval")

	m := NewManager(WithProviders(p))

	s1, err := m.Get(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	// Second call should return the same value (from cache or provider)
	s2, err := m.Get(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if s1.Value != s2.Value {
		t.Errorf("expected same value on second Get, got %q vs %q", s1.Value, s2.Value)
	}
}

func TestManager_Get_CacheExpired(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("key1", "value_v1")

	m := NewManager(WithProviders(p), WithCacheTTL(1*time.Millisecond))

	s1, err := m.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	if s1.Value != "value_v1" {
		t.Errorf("expected value_v1, got %q", s1.Value)
	}

	// Let cache expire
	time.Sleep(5 * time.Millisecond)

	// Update the secret in the provider to prove we bypass cache
	p.setSecret("key1", "value_v2")

	s2, err := m.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if s2.Value != "value_v2" {
		t.Errorf("expected value_v2 after cache expiry, got %q", s2.Value)
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	p := newMockProvider("p1", 100)
	m := NewManager(WithProviders(p))

	_, err := m.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestManager_Get_ProviderPriority(t *testing.T) {
	// lower priority number = checked first
	low := newMockProvider("low_priority", 1)
	low.setSecret("shared_key", "from_low_priority")

	high := newMockProvider("high_priority", 200)
	high.setSecret("shared_key", "from_high_priority")

	m := NewManager(WithProviders(high, low)) // register in reverse order; should sort
	secret, err := m.Get(context.Background(), "shared_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret.Value != "from_low_priority" {
		t.Errorf("expected low-priority provider to win, got %q", secret.Value)
	}
}

func TestManager_Get_Closed(t *testing.T) {
	p := newMockProvider("p1", 100)
	m := NewManager(WithProviders(p))

	if err := m.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	_, err := m.Get(context.Background(), "any")
	if err == nil {
		t.Fatal("expected error after Close, got nil")
	}
}

func TestManager_GetString(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("str_key", "str_value")

	m := NewManager(WithProviders(p))

	got := m.GetString(context.Background(), "str_key")
	if got != "str_value" {
		t.Errorf("expected %q, got %q", "str_value", got)
	}

	missing := m.GetString(context.Background(), "missing_key")
	if missing != "" {
		t.Errorf("expected empty string on miss, got %q", missing)
	}
}

func TestManager_GetWithFallback(t *testing.T) {
	p := newMockProvider("p1", 100)
	m := NewManager(WithProviders(p))

	got := m.GetWithFallback(context.Background(), "missing", "fallback_value")
	if got != "fallback_value" {
		t.Errorf("expected fallback_value, got %q", got)
	}
}

func TestManager_MustGet_Panics(t *testing.T) {
	m := NewManager()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustGet to panic on missing secret")
		}
	}()
	m.MustGet(context.Background(), "nonexistent")
}

func TestManager_GetMultiple(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("key1", "val1")
	p.setSecret("key2", "val2")

	m := NewManager(WithProviders(p))

	results, err := m.GetMultiple(context.Background(), []string{"key1", "key2", "key3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results["key1"] == nil || results["key1"].Value != "val1" {
		t.Error("expected key1=val1")
	}
	if results["key2"] == nil || results["key2"].Value != "val2" {
		t.Error("expected key2=val2")
	}
	if results["key3"] != nil {
		t.Error("expected key3 to be absent")
	}
}

func TestManager_Exists(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("existing", "value")

	m := NewManager(WithProviders(p))

	if !m.Exists(context.Background(), "existing") {
		t.Error("expected Exists=true for provider-backed key")
	}

	// Cache the key first
	_, _ = m.Get(context.Background(), "existing")
	if !m.Exists(context.Background(), "existing") {
		t.Error("expected Exists=true for cached key")
	}

	if m.Exists(context.Background(), "missing") {
		t.Error("expected Exists=false for missing key")
	}
}

func TestManager_List(t *testing.T) {
	p1 := newMockProvider("p1", 1)
	p1.setSecret("alpha", "1")
	p1.setSecret("beta", "2")

	p2 := newMockProvider("p2", 2)
	p2.setSecret("beta", "overlap")
	p2.setSecret("gamma", "3")

	m := NewManager(WithProviders(p1, p2))

	keys, err := m.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect deduplication and sorted output
	want := []string{"alpha", "beta", "gamma"}
	if len(keys) != len(want) {
		t.Fatalf("expected %d keys, got %d: %v", len(want), len(keys), keys)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("keys[%d] = %q, want %q", i, keys[i], k)
		}
	}
}

func TestManager_InvalidateCache(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("key", "original")

	m := NewManager(WithProviders(p))

	s1, _ := m.Get(context.Background(), "key")
	if s1.Value != "original" {
		t.Errorf("expected original, got %q", s1.Value)
	}

	// Invalidate and change provider value
	m.InvalidateCache("key")
	p.setSecret("key", "updated")

	s2, err := m.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s2.Value != "updated" {
		t.Errorf("expected updated after InvalidateCache, got %q", s2.Value)
	}
}

func TestManager_ClearCache(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("k1", "v1")
	p.setSecret("k2", "v2")

	m := NewManager(WithProviders(p))

	_, _ = m.Get(context.Background(), "k1")
	_, _ = m.Get(context.Background(), "k2")

	stats := m.CacheStats()
	if stats.Size != 2 {
		t.Errorf("expected cache size 2 before clear, got %d", stats.Size)
	}

	m.ClearCache()

	stats = m.CacheStats()
	if stats.Size != 0 {
		t.Errorf("expected cache size 0 after ClearCache, got %d", stats.Size)
	}
}

func TestManager_Refresh(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("key", "v1")

	m := NewManager(WithProviders(p))

	s1, _ := m.Get(context.Background(), "key")
	if s1.Value != "v1" {
		t.Errorf("expected v1, got %q", s1.Value)
	}

	// Change provider value; without Refresh, cache would return v1
	p.setSecret("key", "v2")

	s2, err := m.Refresh(context.Background(), "key")
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if s2.Value != "v2" {
		t.Errorf("expected v2 after Refresh, got %q", s2.Value)
	}
}

func TestManager_AddRemoveProvider(t *testing.T) {
	m := NewManager()

	p1 := newMockProvider("alpha", 1)
	p2 := newMockProvider("beta", 2)

	m.AddProvider(p1)
	m.AddProvider(p2)

	names := m.Providers()
	if len(names) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(names))
	}

	removed := m.RemoveProvider("alpha")
	if !removed {
		t.Error("expected RemoveProvider to return true")
	}

	names = m.Providers()
	if len(names) != 1 || names[0] != "beta" {
		t.Errorf("expected [beta], got %v", names)
	}

	notRemoved := m.RemoveProvider("nonexistent")
	if notRemoved {
		t.Error("expected RemoveProvider to return false for nonexistent provider")
	}
}

func TestManager_CacheStats(t *testing.T) {
	p := newMockProvider("p1", 100)
	p.setSecret("s1", "v1")
	p.setSecret("s2", "v2")

	m := NewManager(WithProviders(p))

	_, _ = m.Get(context.Background(), "s1")
	_, _ = m.Get(context.Background(), "s2")

	stats := m.CacheStats()
	if stats.Size != 2 {
		t.Errorf("expected Size=2, got %d", stats.Size)
	}
	if stats.Valid != 2 {
		t.Errorf("expected Valid=2, got %d", stats.Valid)
	}
	if stats.Expired != 0 {
		t.Errorf("expected Expired=0, got %d", stats.Expired)
	}
}

func TestManager_Health(t *testing.T) {
	p1 := newMockProvider("p1", 1)
	p2 := newMockProvider("p2", 2)

	m := NewManager(WithProviders(p1, p2))

	healths := m.Health(context.Background())
	if len(healths) != 2 {
		t.Fatalf("expected 2 health entries, got %d", len(healths))
	}

	names := map[string]bool{}
	for _, h := range healths {
		names[h.Name] = true
		if !h.Available {
			t.Errorf("expected provider %q to be available", h.Name)
		}
	}
	if !names["p1"] || !names["p2"] {
		t.Errorf("expected health for p1 and p2, got %v", names)
	}
}

func TestManager_Close_Idempotent(t *testing.T) {
	p := newMockProvider("p1", 100)
	m := NewManager(WithProviders(p))

	if err := m.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("second Close failed (should be idempotent): %v", err)
	}
}
