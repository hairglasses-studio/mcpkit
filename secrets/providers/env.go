// Package providers implements secret providers for various backends.
package providers

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/secrets"
)

// EnvProvider reads secrets from environment variables.
type EnvProvider struct {
	prefix   string
	priority int
}

// EnvOption configures the EnvProvider.
type EnvOption func(*EnvProvider)

// WithPrefix sets a prefix filter for environment variables.
func WithPrefix(prefix string) EnvOption {
	return func(p *EnvProvider) { p.prefix = prefix }
}

// WithEnvPriority sets the provider priority.
func WithEnvPriority(priority int) EnvOption {
	return func(p *EnvProvider) { p.priority = priority }
}

// NewEnvProvider creates a new environment variable provider.
func NewEnvProvider(opts ...EnvOption) *EnvProvider {
	p := &EnvProvider{priority: 100}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *EnvProvider) Name() string { return "env" }

func (p *EnvProvider) Get(_ context.Context, key string) (*secrets.Secret, error) {
	lookupKey := key
	if p.prefix != "" && !strings.HasPrefix(key, p.prefix) {
		lookupKey = p.prefix + key
	}

	value := os.Getenv(lookupKey)
	if value == "" {
		value = os.Getenv(strings.ToUpper(lookupKey))
		if value == "" {
			return nil, secrets.ErrSecretNotFound
		}
		lookupKey = strings.ToUpper(lookupKey)
	}

	return &secrets.Secret{
		Key:    key,
		Value:  value,
		Source: "env:" + lookupKey,
	}, nil
}

func (p *EnvProvider) List(_ context.Context) ([]string, error) {
	var keys []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if p.prefix == "" {
			keys = append(keys, key)
		} else if strings.HasPrefix(key, p.prefix) {
			keys = append(keys, strings.TrimPrefix(key, p.prefix))
		}
	}
	return keys, nil
}

func (p *EnvProvider) Exists(_ context.Context, key string) (bool, error) {
	lookupKey := key
	if p.prefix != "" && !strings.HasPrefix(key, p.prefix) {
		lookupKey = p.prefix + key
	}
	_, exists := os.LookupEnv(lookupKey)
	if !exists {
		_, exists = os.LookupEnv(strings.ToUpper(lookupKey))
	}
	return exists, nil
}

func (p *EnvProvider) Priority() int    { return p.priority }
func (p *EnvProvider) IsAvailable() bool { return true }
func (p *EnvProvider) Close() error      { return nil }

func (p *EnvProvider) Health(_ context.Context) secrets.ProviderHealth {
	return secrets.ProviderHealth{
		Name:      p.Name(),
		Available: true,
		LastCheck: time.Now(),
	}
}

var _ secrets.SecretProvider = (*EnvProvider)(nil)
