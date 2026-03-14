package secrets

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// DefaultCacheTTL is the default time-to-live for cached secrets.
const DefaultCacheTTL = 5 * time.Minute

// Manager coordinates secret retrieval from multiple providers with caching.
type Manager struct {
	mu        sync.RWMutex
	providers []SecretProvider
	cache     map[string]*cachedSecret
	cacheTTL  time.Duration
	closed    bool
}

type cachedSecret struct {
	secret    *Secret
	expiresAt time.Time
}

// ManagerOption configures the Manager.
type ManagerOption func(*Manager)

// WithCacheTTL sets the cache time-to-live for secrets.
func WithCacheTTL(ttl time.Duration) ManagerOption {
	return func(m *Manager) { m.cacheTTL = ttl }
}

// WithProviders sets the initial providers for the manager.
func WithProviders(providers ...SecretProvider) ManagerOption {
	return func(m *Manager) { m.providers = append(m.providers, providers...) }
}

// NewManager creates a new secrets manager.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		providers: make([]SecretProvider, 0),
		cache:     make(map[string]*cachedSecret),
		cacheTTL:  DefaultCacheTTL,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.sortProviders()
	return m
}

// AddProvider adds a secret provider to the manager.
func (m *Manager) AddProvider(p SecretProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, p)
	m.sortProviders()
}

// RemoveProvider removes a provider by name.
func (m *Manager) RemoveProvider(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.providers {
		if p.Name() == name {
			m.providers = append(m.providers[:i], m.providers[i+1:]...)
			return true
		}
	}
	return false
}

func (m *Manager) sortProviders() {
	sort.Slice(m.providers, func(i, j int) bool {
		return m.providers[i].Priority() < m.providers[j].Priority()
	})
}

// Get retrieves a secret by key, checking cache first, then providers in priority order.
func (m *Manager) Get(ctx context.Context, key string) (*Secret, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, errors.New("manager is closed")
	}
	if cached, ok := m.cache[key]; ok && time.Now().Before(cached.expiresAt) {
		m.mu.RUnlock()
		return cached.secret, nil
	}
	providers := make([]SecretProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	var lastErr error
	for _, p := range providers {
		if !p.IsAvailable() {
			continue
		}
		secret, err := p.Get(ctx, key)
		if err == nil {
			m.cacheSecret(key, secret)
			return secret, nil
		}
		if !errors.Is(err, ErrSecretNotFound) {
			lastErr = fmt.Errorf("%s: %w", p.Name(), err)
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrSecretNotFound
}

// GetString retrieves a secret value as a string, returning empty string if not found.
func (m *Manager) GetString(ctx context.Context, key string) string {
	secret, err := m.Get(ctx, key)
	if err != nil {
		return ""
	}
	return secret.Value
}

// MustGet retrieves a secret, panicking if not found or on error.
func (m *Manager) MustGet(ctx context.Context, key string) *Secret {
	secret, err := m.Get(ctx, key)
	if err != nil {
		panic(fmt.Sprintf("required secret %q not found: %v", key, err))
	}
	return secret
}

// GetWithFallback retrieves a secret, returning the fallback value if not found.
func (m *Manager) GetWithFallback(ctx context.Context, key, fallback string) string {
	secret, err := m.Get(ctx, key)
	if err != nil {
		return fallback
	}
	return secret.Value
}

// GetMultiple retrieves multiple secrets at once.
func (m *Manager) GetMultiple(ctx context.Context, keys []string) (map[string]*Secret, error) {
	results := make(map[string]*Secret)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, len(keys))

	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			secret, err := m.Get(ctx, k)
			if err != nil && !errors.Is(err, ErrSecretNotFound) {
				errs <- fmt.Errorf("%s: %w", k, err)
				return
			}
			if secret != nil {
				mu.Lock()
				results[k] = secret
				mu.Unlock()
			}
		}(key)
	}
	wg.Wait()
	close(errs)

	var combinedErr error
	for err := range errs {
		combinedErr = errors.Join(combinedErr, err)
	}
	return results, combinedErr
}

// Exists checks if a secret exists in any provider.
func (m *Manager) Exists(ctx context.Context, key string) bool {
	m.mu.RLock()
	if cached, ok := m.cache[key]; ok && time.Now().Before(cached.expiresAt) {
		m.mu.RUnlock()
		return true
	}
	providers := make([]SecretProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	for _, p := range providers {
		if !p.IsAvailable() {
			continue
		}
		exists, err := p.Exists(ctx, key)
		if err == nil && exists {
			return true
		}
	}
	return false
}

// List returns all available secret keys from all providers.
func (m *Manager) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	providers := make([]SecretProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	seen := make(map[string]bool)
	var keys []string
	for _, p := range providers {
		if !p.IsAvailable() {
			continue
		}
		pKeys, err := p.List(ctx)
		if err != nil {
			continue
		}
		for _, key := range pKeys {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *Manager) cacheSecret(key string, secret *Secret) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[key] = &cachedSecret{
		secret:    secret,
		expiresAt: time.Now().Add(m.cacheTTL),
	}
}

// InvalidateCache removes a specific key from the cache.
func (m *Manager) InvalidateCache(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, key)
}

// ClearCache removes all entries from the cache.
func (m *Manager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]*cachedSecret)
}

// Refresh forces a fresh lookup of a secret, bypassing cache.
func (m *Manager) Refresh(ctx context.Context, key string) (*Secret, error) {
	m.InvalidateCache(key)
	return m.Get(ctx, key)
}

// Health returns the health status of all providers.
func (m *Manager) Health(ctx context.Context) []ProviderHealth {
	m.mu.RLock()
	providers := make([]SecretProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.RUnlock()

	var results []ProviderHealth
	for _, p := range providers {
		health := ProviderHealth{
			Name:      p.Name(),
			Available: p.IsAvailable(),
			LastCheck: time.Now(),
		}
		if hc, ok := p.(HealthChecker); ok {
			health = hc.Health(ctx)
		}
		results = append(results, health)
	}
	return results
}

// Providers returns a list of configured provider names.
func (m *Manager) Providers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, len(m.providers))
	for i, p := range m.providers {
		names[i] = p.Name()
	}
	return names
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Size    int           `json:"size"`
	Valid   int           `json:"valid"`
	Expired int           `json:"expired"`
	TTL     time.Duration `json:"ttl_seconds"`
}

// CacheStats returns statistics about the cache.
func (m *Manager) CacheStats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var valid, expired int
	now := time.Now()
	for _, cached := range m.cache {
		if now.Before(cached.expiresAt) {
			valid++
		} else {
			expired++
		}
	}
	return CacheStats{
		Size:    len(m.cache),
		Valid:   valid,
		Expired: expired,
		TTL:     m.cacheTTL,
	}
}

// Close releases resources and closes all providers.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	var errs error
	for _, p := range m.providers {
		if err := p.Close(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p.Name(), err))
		}
	}
	m.cache = nil
	m.providers = nil
	return errs
}
