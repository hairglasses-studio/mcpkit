package session

import (
	"context"
	"time"
)

// RedisClient is the minimal Redis interface needed for session persistence.
// It accepts string values (JSON-serialized sessions) and is compatible with
// go-redis, rueidis, or any Redis client library. Users provide their own
// implementation — mcpkit has no hard dependency on any Redis package.
//
// This is a higher-level alternative to [RedisAdapter] (which uses []byte).
// The two interfaces serve different use cases: RedisClient is simpler for
// JSON-only workflows; RedisAdapter supports binary (gob) encoding.
type RedisClient interface {
	// Get retrieves a string value by key.
	// Returns ("", ErrNotFound) if the key does not exist.
	Get(ctx context.Context, key string) (string, error)
	// Set stores a string value with an optional TTL (0 = no expiry).
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	// Del deletes one or more keys.
	Del(ctx context.Context, keys ...string) error
}

// RedisOption configures a RedisStringStore.
type RedisOption func(*RedisStringStore)

// WithPrefix sets the key prefix for all session keys.
// Default: "mcp:session:".
func WithPrefix(prefix string) RedisOption {
	return func(s *RedisStringStore) {
		s.prefix = prefix
	}
}

// WithTTL sets the session TTL for expiration.
// Default: 0 (no expiry).
func WithTTL(ttl time.Duration) RedisOption {
	return func(s *RedisStringStore) {
		s.ttl = ttl
	}
}

// RedisStringStore implements ExternalStore using the string-based [RedisClient]
// interface. It serializes sessions as JSON strings and is suitable for
// stateless HTTP servers behind load balancers.
//
// For binary (gob) encoding or finer control over Expire/Ping, use
// [RedisStore] with a [RedisAdapter] instead.
type RedisStringStore struct {
	client RedisClient
	prefix string
	ttl    time.Duration
}

// NewRedisStringStore creates a new RedisStringStore with the given client
// and functional options.
//
//	store := session.NewRedisStringStore(myRedisClient,
//	    session.WithPrefix("app:sess:"),
//	    session.WithTTL(30 * time.Minute),
//	)
func NewRedisStringStore(client RedisClient, opts ...RedisOption) *RedisStringStore {
	s := &RedisStringStore{
		client: client,
		prefix: "mcp:session:",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *RedisStringStore) key(id string) string {
	return s.prefix + id
}

// Create creates a new session and persists it to Redis.
func (s *RedisStringStore) Create(ctx context.Context) (Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	sess := newSession(id, s.ttl)
	if err := s.Save(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// Get retrieves a session from Redis by ID. Returns (nil, false, nil) if not found.
func (s *RedisStringStore) Get(ctx context.Context, id string) (Session, bool, error) {
	sess, err := s.Load(ctx, id)
	if err != nil {
		if err == ErrNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return sess, true, nil
}

// Delete removes a session from Redis.
func (s *RedisStringStore) Delete(ctx context.Context, id string) error {
	return s.client.Del(ctx, s.key(id))
}

// Refresh re-saves the session with a refreshed TTL. Since RedisClient only
// supports Get/Set/Del, this loads and re-saves the session.
func (s *RedisStringStore) Refresh(ctx context.Context, id string) error {
	if s.ttl == 0 {
		return nil
	}
	sess, err := s.Load(ctx, id)
	if err != nil {
		return err
	}
	return s.Save(ctx, sess)
}

// Save serializes and stores a session in Redis as JSON.
func (s *RedisStringStore) Save(ctx context.Context, sess Session) error {
	data, err := MarshalSession(sess)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(sess.ID()), string(data), s.ttl)
}

// Load retrieves and deserializes a session from Redis.
func (s *RedisStringStore) Load(ctx context.Context, id string) (Session, error) {
	data, err := s.client.Get(ctx, s.key(id))
	if err != nil {
		return nil, err
	}
	return UnmarshalSession([]byte(data))
}

// Ping checks Redis connectivity. Since RedisClient does not expose Ping,
// this performs a Get for a non-existent key — a successful ErrNotFound
// confirms the connection is alive.
func (s *RedisStringStore) Ping(ctx context.Context) error {
	_, err := s.client.Get(ctx, s.prefix+"__ping__")
	if err == ErrNotFound {
		return nil
	}
	return err
}

// Close is a no-op — connection lifecycle is managed externally by the caller.
func (s *RedisStringStore) Close() error {
	return nil
}
