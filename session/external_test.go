package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockRedis is a simple in-memory mock of RedisAdapter for testing.
type mockRedis struct {
	data    map[string][]byte
	ttls    map[string]time.Duration
	pingErr error
}

func newMockRedis() *mockRedis {
	return &mockRedis{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Duration),
	}
}

func (m *mockRedis) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.data[key] = value
	m.ttls[key] = ttl
	return nil
}

func (m *mockRedis) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (m *mockRedis) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

func (m *mockRedis) Expire(_ context.Context, key string, ttl time.Duration) error {
	if _, ok := m.data[key]; !ok {
		return ErrNotFound
	}
	m.ttls[key] = ttl
	return nil
}

func (m *mockRedis) Ping(_ context.Context) error {
	return m.pingErr
}

func TestRedisStore_CreateAndGet(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{TTL: time.Minute}, "test:")

	ctx := context.Background()
	s, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.ID() == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Get should find it
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

func TestRedisStore_GetMissing(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{}, "test:")

	_, ok, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}

func TestRedisStore_Delete(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{TTL: time.Minute}, "test:")

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

func TestRedisStore_Refresh(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{TTL: time.Minute}, "test:")

	ctx := context.Background()
	s, _ := store.Create(ctx)

	if err := store.Refresh(ctx, s.ID()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// TTL should be refreshed in mock
	key := "test:" + s.ID()
	if redis.ttls[key] != time.Minute {
		t.Errorf("TTL not refreshed, got %v", redis.ttls[key])
	}
}

func TestRedisStore_RefreshNoTTL(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{TTL: 0}, "test:")

	ctx := context.Background()
	s, _ := store.Create(ctx)

	// No TTL means Refresh is a no-op
	if err := store.Refresh(ctx, s.ID()); err != nil {
		t.Fatalf("Refresh with no TTL: %v", err)
	}
}

func TestRedisStore_Ping(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{}, "")

	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	redis.pingErr = errors.New("connection refused")
	if err := store.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error")
	}
}

func TestRedisStore_DefaultKeyPrefix(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{TTL: time.Minute}, "")

	ctx := context.Background()
	s, _ := store.Create(ctx)

	// Default prefix should be "mcp:session:"
	expectedKey := "mcp:session:" + s.ID()
	if _, ok := redis.data[expectedKey]; !ok {
		t.Errorf("expected key %q in redis, got keys: %v", expectedKey, keysOf(redis.data))
	}
}

func TestRedisStore_Close(t *testing.T) {
	redis := newMockRedis()
	store := NewRedisStore(redis, Options{}, "")
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMarshalUnmarshalSession(t *testing.T) {
	s := newSession("test-id", time.Minute)
	s.Set("key1", "value1")
	s.Set("count", float64(42)) // JSON numbers are float64

	data, err := MarshalSession(s)
	if err != nil {
		t.Fatalf("MarshalSession: %v", err)
	}

	restored, err := UnmarshalSession(data)
	if err != nil {
		t.Fatalf("UnmarshalSession: %v", err)
	}

	if restored.ID() != s.ID() {
		t.Errorf("ID mismatch: got %q want %q", restored.ID(), s.ID())
	}
	if restored.State() != s.State() {
		t.Errorf("State mismatch: got %v want %v", restored.State(), s.State())
	}

	v, ok := restored.Get("key1")
	if !ok || v != "value1" {
		t.Errorf("key1: got %v, ok=%v", v, ok)
	}
	v, ok = restored.Get("count")
	if !ok || v != float64(42) {
		t.Errorf("count: got %v, ok=%v", v, ok)
	}
}

func TestSnapshot(t *testing.T) {
	s := newSession("snap-id", time.Hour)
	s.Set("foo", "bar")

	ss := Snapshot(s)
	if ss.ID != "snap-id" {
		t.Errorf("ID: got %q", ss.ID)
	}
	if ss.Data["foo"] != "bar" {
		t.Errorf("Data[foo]: got %v", ss.Data["foo"])
	}
}

func TestRestore(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	ss := &SerializableSession{
		ID:        "restore-id",
		State:     StateActive,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
		Data:      map[string]any{"x": "y"},
	}
	s := Restore(ss)
	if s.ID() != "restore-id" {
		t.Errorf("ID: got %q", s.ID())
	}
	v, ok := s.Get("x")
	if !ok || v != "y" {
		t.Errorf("Get x: got %v, ok=%v", v, ok)
	}
}

func TestMarshalUnmarshalSessionGob(t *testing.T) {
	s := newSession("gob-test-id", time.Minute)
	s.Set("key1", "value1")
	s.Set("count", 42) // gob preserves int type (unlike JSON float64)

	data, err := MarshalSessionGob(s)
	if err != nil {
		t.Fatalf("MarshalSessionGob: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty gob data")
	}

	restored, err := UnmarshalSessionGob(data)
	if err != nil {
		t.Fatalf("UnmarshalSessionGob: %v", err)
	}

	if restored.ID() != s.ID() {
		t.Errorf("ID mismatch: got %q want %q", restored.ID(), s.ID())
	}

	v, ok := restored.Get("key1")
	if !ok || v != "value1" {
		t.Errorf("key1: got %v, ok=%v", v, ok)
	}
	v, ok = restored.Get("count")
	if !ok || v != 42 {
		t.Errorf("count: got %v (type %T), ok=%v", v, v, ok)
	}
}

func TestGobSessionPreservesData(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	s := newSession("preserve-id", 2*time.Hour)

	// Override internal timestamps for deterministic comparison.
	s.createdAt = now
	s.expiresAt = now.Add(2 * time.Hour)
	s.state = StateActive

	s.Set("str", "hello")
	s.Set("num", 3.14)
	s.Set("flag", true)

	data, err := MarshalSessionGob(s)
	if err != nil {
		t.Fatalf("MarshalSessionGob: %v", err)
	}

	restored, err := UnmarshalSessionGob(data)
	if err != nil {
		t.Fatalf("UnmarshalSessionGob: %v", err)
	}

	// Verify all fields survive the round-trip.
	if restored.ID() != "preserve-id" {
		t.Errorf("ID: got %q", restored.ID())
	}
	if restored.State() != StateActive {
		t.Errorf("State: got %v, want %v", restored.State(), StateActive)
	}
	if !restored.CreatedAt().Equal(now) {
		t.Errorf("CreatedAt: got %v, want %v", restored.CreatedAt(), now)
	}
	if !restored.ExpiresAt().Equal(now.Add(2 * time.Hour)) {
		t.Errorf("ExpiresAt: got %v, want %v", restored.ExpiresAt(), now.Add(2*time.Hour))
	}

	for _, tc := range []struct {
		key  string
		want any
	}{
		{"str", "hello"},
		{"num", 3.14},
		{"flag", true},
	} {
		v, ok := restored.Get(tc.key)
		if !ok {
			t.Errorf("%s: not found", tc.key)
			continue
		}
		if v != tc.want {
			t.Errorf("%s: got %v (%T), want %v (%T)", tc.key, v, v, tc.want, tc.want)
		}
	}
}

func keysOf(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
