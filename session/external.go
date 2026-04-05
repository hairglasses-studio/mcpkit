package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"maps"
	"time"
)

// ErrNotFound is returned when a session is not found in the store.
var ErrNotFound = errors.New("session: not found")

// ErrStoreClosed is returned when an operation is performed on a closed store.
var ErrStoreClosed = errors.New("session: store closed")

// ExternalStore is the interface for external (e.g. Redis) session storage backends.
// It extends SessionStore with serialization support for distributed deployments.
type ExternalStore interface {
	SessionStore
	// Save serializes and persists a session to external storage.
	Save(ctx context.Context, s Session) error
	// Load deserializes a session from external storage by ID.
	Load(ctx context.Context, id string) (Session, error)
	// Ping checks connectivity to the external store.
	Ping(ctx context.Context) error
}

// SerializableSession is a JSON-serializable snapshot of a session.
type SerializableSession struct {
	ID        string         `json:"id"`
	Data      map[string]any `json:"data"`
	State     State          `json:"state"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt time.Time      `json:"expires_at"`
}

// Snapshot creates a serializable snapshot of the session.
func Snapshot(s Session) *SerializableSession {
	ss := &SerializableSession{
		ID:        s.ID(),
		State:     s.State(),
		CreatedAt: s.CreatedAt(),
		ExpiresAt: s.ExpiresAt(),
		Data:      make(map[string]any),
	}
	// Concrete sessions expose data via Get; for MemStore sessions we can
	// type-assert to access internal data for snapshots.
	if ms, ok := s.(*session); ok {
		ms.mu.RLock()
		maps.Copy(ss.Data, ms.data)
		ms.mu.RUnlock()
	}
	return ss
}

// Restore recreates a session from a serializable snapshot.
func Restore(ss *SerializableSession) Session {
	s := &session{
		id:        ss.ID,
		data:      make(map[string]any),
		state:     ss.State,
		createdAt: ss.CreatedAt,
		expiresAt: ss.ExpiresAt,
	}
	maps.Copy(s.data, ss.Data)
	return s
}

// MarshalSession serializes a session to JSON bytes.
func MarshalSession(s Session) ([]byte, error) {
	return json.Marshal(Snapshot(s))
}

// UnmarshalSession deserializes a session from JSON bytes.
func UnmarshalSession(data []byte) (Session, error) {
	var ss SerializableSession
	if err := json.Unmarshal(data, &ss); err != nil {
		return nil, err
	}
	return Restore(&ss), nil
}

func init() {
	// Register common types stored in session Data (map[string]any) so gob
	// can encode/decode them through the any interface.
	gob.Register("")
	gob.Register(0)
	gob.Register(0.0)
	gob.Register(false)
	gob.Register([]any{})
	gob.Register(map[string]any{})
}

// MarshalSessionGob serializes a session to gob-encoded bytes.
func MarshalSessionGob(s Session) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(Snapshot(s)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalSessionGob deserializes a session from gob-encoded bytes.
func UnmarshalSessionGob(data []byte) (Session, error) {
	var ss SerializableSession
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&ss); err != nil {
		return nil, err
	}
	return Restore(&ss), nil
}

// RedisAdapter is a thin interface representing the Redis commands needed
// by ExternalSessionStore. This allows users to inject any Redis client
// (go-redis, rueidis, etc.) without a direct dependency.
type RedisAdapter interface {
	// Set stores a value with an optional TTL (0 = no expiry).
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Get retrieves a value by key. Returns (nil, ErrNotFound) if missing.
	Get(ctx context.Context, key string) ([]byte, error)
	// Del deletes one or more keys.
	Del(ctx context.Context, keys ...string) error
	// Expire resets the TTL for a key.
	Expire(ctx context.Context, key string, ttl time.Duration) error
	// Ping checks the connection.
	Ping(ctx context.Context) error
}

// RedisStore implements ExternalStore using a RedisAdapter.
type RedisStore struct {
	client RedisAdapter
	opts   Options
	keyPfx string
}

// NewRedisStore creates a new RedisStore.
// keyPrefix is prepended to all session keys (e.g. "mcp:session:").
func NewRedisStore(client RedisAdapter, opts Options, keyPrefix string) *RedisStore {
	if keyPrefix == "" {
		keyPrefix = "mcp:session:"
	}
	return &RedisStore{
		client: client,
		opts:   opts,
		keyPfx: keyPrefix,
	}
}

func (r *RedisStore) key(id string) string {
	return r.keyPfx + id
}

// Create creates a new session and persists it to Redis.
func (r *RedisStore) Create(ctx context.Context) (Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	s := newSession(id, r.opts.TTL)
	if err := r.Save(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Get retrieves a session from Redis by ID.
func (r *RedisStore) Get(ctx context.Context, id string) (Session, bool, error) {
	s, err := r.Load(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return s, true, nil
}

// Delete removes a session from Redis.
func (r *RedisStore) Delete(ctx context.Context, id string) error {
	return r.client.Del(ctx, r.key(id))
}

// Refresh resets the TTL of a session in Redis.
func (r *RedisStore) Refresh(ctx context.Context, id string) error {
	if r.opts.TTL == 0 {
		return nil
	}
	return r.client.Expire(ctx, r.key(id), r.opts.TTL)
}

// Save serializes and stores a session in Redis.
func (r *RedisStore) Save(ctx context.Context, s Session) error {
	data, err := MarshalSession(s)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, r.key(s.ID()), data, r.opts.TTL)
}

// Load retrieves and deserializes a session from Redis.
func (r *RedisStore) Load(ctx context.Context, id string) (Session, error) {
	data, err := r.client.Get(ctx, r.key(id))
	if err != nil {
		return nil, err
	}
	return UnmarshalSession(data)
}

// Ping checks Redis connectivity.
func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx)
}

// Close is a no-op for RedisStore (connection lifecycle managed externally).
func (r *RedisStore) Close() error {
	return nil
}
