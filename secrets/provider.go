package secrets

import (
	"context"
	"errors"
	"time"
)

// Common errors for secret providers.
var (
	ErrSecretNotFound  = errors.New("secret not found")
	ErrProviderError   = errors.New("provider error")
	ErrInvalidKey      = errors.New("invalid secret key")
	ErrProviderTimeout = errors.New("provider timeout")
)

// Secret represents a retrieved secret with metadata.
type Secret struct {
	Key       string    `json:"key"`
	Value     string    `json:"value,omitempty"`
	Source    string    `json:"source"`
	ExpiresAt time.Time `json:"expires_at"`
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsExpired returns true if the secret has expired.
func (s *Secret) IsExpired() bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(s.ExpiresAt)
}

// Masked returns the secret value with masking for safe logging.
func (s *Secret) Masked() string {
	if len(s.Value) <= 4 {
		return "****"
	}
	return s.Value[:2] + "****" + s.Value[len(s.Value)-2:]
}

// SecretProvider is the interface that all secret providers must implement.
type SecretProvider interface {
	Name() string
	Get(ctx context.Context, key string) (*Secret, error)
	List(ctx context.Context) ([]string, error)
	Exists(ctx context.Context, key string) (bool, error)
	Priority() int
	IsAvailable() bool
	Close() error
}

// WritableProvider extends SecretProvider with write capabilities.
type WritableProvider interface {
	SecretProvider
	Set(ctx context.Context, key, value string, opts ...SetOption) error
	Delete(ctx context.Context, key string) error
}

// SetOption configures secret creation/update behavior.
type SetOption func(*SetOptions)

// SetOptions holds options for setting secrets.
type SetOptions struct {
	TTL         time.Duration
	Description string
	Tags        map[string]string
	Version     string
}

// WithTTL sets a time-to-live for the secret.
func WithTTL(ttl time.Duration) SetOption {
	return func(o *SetOptions) { o.TTL = ttl }
}

// WithDescription sets a description for the secret.
func WithDescription(desc string) SetOption {
	return func(o *SetOptions) { o.Description = desc }
}

// WithTags sets metadata tags for the secret.
func WithTags(tags map[string]string) SetOption {
	return func(o *SetOptions) { o.Tags = tags }
}

// ProviderHealth represents the health status of a provider.
type ProviderHealth struct {
	Name      string        `json:"name"`
	Available bool          `json:"available"`
	Latency   time.Duration `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
	LastCheck time.Time     `json:"last_check"`
}

// HealthChecker can report its health status.
type HealthChecker interface {
	Health(ctx context.Context) ProviderHealth
}
