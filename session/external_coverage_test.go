package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

// errorRedis is a mock that returns errors on specific operations.
type errorRedis struct {
	setErr    error
	getErr    error
	delErr    error
	expireErr error
	data      map[string][]byte
}

func newErrorRedis() *errorRedis {
	return &errorRedis{data: make(map[string][]byte)}
}

func (e *errorRedis) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	if e.setErr != nil {
		return e.setErr
	}
	e.data[key] = value
	return nil
}

func (e *errorRedis) Get(_ context.Context, key string) ([]byte, error) {
	if e.getErr != nil {
		return nil, e.getErr
	}
	v, ok := e.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (e *errorRedis) Del(_ context.Context, keys ...string) error {
	if e.delErr != nil {
		return e.delErr
	}
	for _, k := range keys {
		delete(e.data, k)
	}
	return nil
}

func (e *errorRedis) Expire(_ context.Context, _ string, _ time.Duration) error {
	return e.expireErr
}

func (e *errorRedis) Ping(_ context.Context) error {
	return nil
}

func TestRedisStore_Create_SaveError(t *testing.T) {
	redis := newErrorRedis()
	redis.setErr = errors.New("redis write failed")
	store := NewRedisStore(redis, Options{TTL: time.Minute}, "test:")

	_, err := store.Create(context.Background())
	if err == nil {
		t.Fatal("expected error when Save fails")
	}
}

func TestRedisStore_Get_NonNotFoundError(t *testing.T) {
	redis := newErrorRedis()
	redis.getErr = errors.New("connection timeout")
	store := NewRedisStore(redis, Options{}, "test:")

	_, ok, err := store.Get(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error for non-ErrNotFound failure")
	}
	if ok {
		t.Fatal("expected ok=false on error")
	}
}

func TestRedisStore_Save_MarshalAndSet(t *testing.T) {
	redis := newErrorRedis()
	store := NewRedisStore(redis, Options{TTL: time.Minute}, "test:")

	s := newSession("save-test", time.Minute)
	s.Set("key", "val")

	if err := store.Save(context.Background(), s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify the data was stored.
	key := "test:save-test"
	if _, ok := redis.data[key]; !ok {
		t.Error("expected data to be stored in redis")
	}
}

func TestRedisStore_Save_SetError(t *testing.T) {
	redis := newErrorRedis()
	redis.setErr = errors.New("disk full")
	store := NewRedisStore(redis, Options{TTL: time.Minute}, "test:")

	s := newSession("save-err", time.Minute)
	if err := store.Save(context.Background(), s); err == nil {
		t.Fatal("expected error when Set fails")
	}
}

func TestUnmarshalSession_InvalidJSON(t *testing.T) {
	_, err := UnmarshalSession([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRunEviction_DeleteError(t *testing.T) {
	redis := newErrorRedis()
	redis.delErr = errors.New("delete failed")
	store := NewRedisStore(redis, Options{}, "test:")

	// Create a session that will be evicted.
	s := newSession("evict-err", 0)
	_ = s.Close() // Mark as closed.

	sessions := []Session{s}
	policy := StatePolicy{}

	err := RunEviction(context.Background(), store, sessions, policy)
	if err == nil {
		t.Fatal("expected error when Delete fails during eviction")
	}
}

func TestRunEviction_NoSessionsToEvict(t *testing.T) {
	store := NewMemStore(Options{})
	defer store.Close()

	// Active sessions — none should be evicted.
	s1 := newSession("active1", time.Hour)
	s2 := newSession("active2", time.Hour)

	sessions := []Session{s1, s2}
	policy := StatePolicy{}

	if err := RunEviction(context.Background(), store, sessions, policy); err != nil {
		t.Fatalf("RunEviction: %v", err)
	}
}

func TestSnapshot_NonConcreteSession(t *testing.T) {
	// Restore creates a *session (concrete), but test that Snapshot handles
	// sessions created via Restore.
	ss := &SerializableSession{
		ID:        "snap-restore",
		State:     StateActive,
		CreatedAt: time.Now(),
		Data:      map[string]any{"k": "v"},
	}
	s := Restore(ss)

	snap := Snapshot(s)
	if snap.ID != "snap-restore" {
		t.Errorf("ID: got %q", snap.ID)
	}
	if snap.Data["k"] != "v" {
		t.Errorf("Data[k]: got %v", snap.Data["k"])
	}
}
