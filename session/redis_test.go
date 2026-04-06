package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockRedisClient is an in-memory mock of RedisClient for testing.
type mockRedisClient struct {
	data   map[string]string
	ttls   map[string]time.Duration
	getErr error
	setErr error
	delErr error
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		data: make(map[string]string),
		ttls: make(map[string]time.Duration),
	}
}

func (m *mockRedisClient) Get(_ context.Context, key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	v, ok := m.data[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *mockRedisClient) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	m.ttls[key] = ttl
	return nil
}

func (m *mockRedisClient) Del(_ context.Context, keys ...string) error {
	if m.delErr != nil {
		return m.delErr
	}
	for _, k := range keys {
		delete(m.data, k)
		delete(m.ttls, k)
	}
	return nil
}

func TestRedisStringStore_CreateAndGet(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithTTL(time.Minute))

	ctx := context.Background()
	s, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.ID() == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Data should be stored in the mock.
	key := "mcp:session:" + s.ID()
	if _, ok := client.data[key]; !ok {
		t.Fatalf("expected key %q in mock, got keys: %v", key, keys(client.data))
	}

	// Get should find it.
	got, ok, err := store.Get(ctx, s.ID())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected session to be found")
	}
	if got.ID() != s.ID() {
		t.Errorf("ID mismatch: got %q want %q", got.ID(), s.ID())
	}
}

func TestRedisStringStore_GetMissing(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client)

	_, ok, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}

func TestRedisStringStore_GetError(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	client.getErr = errors.New("connection timeout")
	store := NewRedisStringStore(client)

	_, ok, err := store.Get(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error")
	}
	if ok {
		t.Fatal("expected ok=false on error")
	}
}

func TestRedisStringStore_Delete(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithTTL(time.Minute))

	ctx := context.Background()
	s, _ := store.Create(ctx)

	if err := store.Delete(ctx, s.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, ok, _ := store.Get(ctx, s.ID())
	if ok {
		t.Fatal("expected session to be deleted")
	}
}

func TestRedisStringStore_DeleteError(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	client.delErr = errors.New("delete failed")
	store := NewRedisStringStore(client)

	err := store.Delete(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRedisStringStore_Refresh(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithTTL(time.Minute))

	ctx := context.Background()
	s, _ := store.Create(ctx)

	// Refresh should re-save with the TTL.
	if err := store.Refresh(ctx, s.ID()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	key := "mcp:session:" + s.ID()
	if client.ttls[key] != time.Minute {
		t.Errorf("TTL not applied on refresh, got %v", client.ttls[key])
	}
}

func TestRedisStringStore_RefreshNoTTL(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client) // no TTL

	ctx := context.Background()
	s, _ := store.Create(ctx)

	// Refresh with no TTL is a no-op.
	if err := store.Refresh(ctx, s.ID()); err != nil {
		t.Fatalf("Refresh with no TTL: %v", err)
	}
}

func TestRedisStringStore_RefreshMissing(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithTTL(time.Minute))

	err := store.Refresh(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestRedisStringStore_SaveAndLoad(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithTTL(5*time.Minute))

	s := newSession("save-load-test", 5*time.Minute)
	s.Set("name", "alice")
	s.Set("score", float64(42))

	ctx := context.Background()
	if err := store.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, "save-load-test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID() != "save-load-test" {
		t.Errorf("ID mismatch: %q", loaded.ID())
	}

	v, ok := loaded.Get("name")
	if !ok || v != "alice" {
		t.Errorf("name: got %v, ok=%v", v, ok)
	}
	v, ok = loaded.Get("score")
	if !ok || v != float64(42) {
		t.Errorf("score: got %v, ok=%v", v, ok)
	}
}

func TestRedisStringStore_SaveError(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	client.setErr = errors.New("disk full")
	store := NewRedisStringStore(client)

	s := newSession("err-test", 0)
	if err := store.Save(context.Background(), s); err == nil {
		t.Fatal("expected error when Set fails")
	}
}

func TestRedisStringStore_LoadMissing(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client)

	_, err := store.Load(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRedisStringStore_LoadCorruptData(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	client.data["mcp:session:corrupt"] = "not-valid-json"
	store := NewRedisStringStore(client)

	_, err := store.Load(context.Background(), "corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt data")
	}
}

func TestRedisStringStore_CreateError(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	client.setErr = errors.New("redis write failed")
	store := NewRedisStringStore(client, WithTTL(time.Minute))

	_, err := store.Create(context.Background())
	if err == nil {
		t.Fatal("expected error when Save fails during Create")
	}
}

func TestRedisStringStore_Ping(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client)

	// Ping succeeds — Get returns ErrNotFound, which is treated as healthy.
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Ping fails — Get returns a real error.
	client.getErr = errors.New("connection refused")
	if err := store.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error")
	}
}

func TestRedisStringStore_PingKeyExists(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	// Simulate the ping key actually existing (unusual but valid).
	client.data["mcp:session:__ping__"] = "surprise"
	store := NewRedisStringStore(client)

	// Should return nil since Get succeeded (no error).
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping with existing key: %v", err)
	}
}

func TestRedisStringStore_Close(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestRedisStringStore_CustomPrefix(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithPrefix("app:sess:"), WithTTL(time.Minute))

	ctx := context.Background()
	s, _ := store.Create(ctx)

	expectedKey := "app:sess:" + s.ID()
	if _, ok := client.data[expectedKey]; !ok {
		t.Errorf("expected key %q, got keys: %v", expectedKey, keys(client.data))
	}
}

func TestRedisStringStore_DefaultPrefix(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithTTL(time.Minute))

	ctx := context.Background()
	s, _ := store.Create(ctx)

	expectedKey := "mcp:session:" + s.ID()
	if _, ok := client.data[expectedKey]; !ok {
		t.Errorf("expected default prefix key %q", expectedKey)
	}
}

func TestWithPrefix(t *testing.T) {
	t.Parallel()
	store := NewRedisStringStore(newMockRedisClient(), WithPrefix("custom:"))
	if store.prefix != "custom:" {
		t.Errorf("expected prefix 'custom:', got %q", store.prefix)
	}
}

func TestWithTTL(t *testing.T) {
	t.Parallel()
	store := NewRedisStringStore(newMockRedisClient(), WithTTL(5*time.Minute))
	if store.ttl != 5*time.Minute {
		t.Errorf("expected TTL 5m, got %v", store.ttl)
	}
}

func TestRedisStringStore_TTLApplied(t *testing.T) {
	t.Parallel()
	client := newMockRedisClient()
	store := NewRedisStringStore(client, WithTTL(10*time.Minute))

	ctx := context.Background()
	s, _ := store.Create(ctx)

	key := "mcp:session:" + s.ID()
	if client.ttls[key] != 10*time.Minute {
		t.Errorf("expected TTL 10m, got %v", client.ttls[key])
	}
}

func keys(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
