//go:build !official_sdk

package prefetch

import (
	"context"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// DefaultCacheTTL is the default duration that pre-fetched data remains valid.
const DefaultCacheTTL = 5 * time.Minute

// DefaultMaxConcurrent is the default limit for parallel prefetch operations.
const DefaultMaxConcurrent = 4

// contextKey is an unexported type for context keys in this package.
type contextKey string

// PrefetchProvider fetches a piece of context data and determines applicability.
type PrefetchProvider struct {
	// Fetch retrieves the data. Called once per TTL window.
	// The context carries deadlines from the original tool call.
	Fetch func(ctx context.Context) (any, error)

	// ShouldPrefetch determines if this data should be pre-loaded
	// for the given tool. Return true to include in pre-fetch.
	// If nil, the provider is included for all tools.
	ShouldPrefetch func(toolName string) bool
}

// Config controls prefetch middleware behavior.
type Config struct {
	// Providers are functions that fetch data on-demand.
	// Key = data identifier, Value = fetcher + filter.
	Providers map[string]PrefetchProvider

	// CacheTTL is the duration that pre-fetched data remains valid.
	// After TTL expires, the next tool call triggers a fresh fetch.
	// Default: 5 minutes.
	CacheTTL time.Duration

	// MaxConcurrent limits parallel prefetch operations.
	// Default: 4.
	MaxConcurrent int
}

// DefaultConfig returns a Config with production defaults and no providers.
func DefaultConfig() Config {
	return Config{
		Providers:     make(map[string]PrefetchProvider),
		CacheTTL:      DefaultCacheTTL,
		MaxConcurrent: DefaultMaxConcurrent,
	}
}

// cacheEntry holds a cached prefetch result with its expiry time.
type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// Middleware returns a registry.Middleware that pre-loads context data before
// tool execution.
//
// For each tool call, providers whose ShouldPrefetch returns true (or is nil)
// are executed concurrently up to MaxConcurrent. Results are cached per TTL
// and injected into the tool's context. Handlers retrieve data with
// PrefetchFromContext.
//
// Provider errors are silently ignored (graceful degradation): the tool still
// executes, it just won't have that piece of pre-fetched data in its context.
func Middleware(cfg Config) registry.Middleware {
	ttl := cfg.CacheTTL
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}

	maxConc := cfg.MaxConcurrent
	if maxConc <= 0 {
		maxConc = DefaultMaxConcurrent
	}

	// Shared cache protected by RWMutex.
	var mu sync.RWMutex
	cache := make(map[string]cacheEntry)

	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		// If no providers, pass through unchanged — zero overhead.
		if len(cfg.Providers) == 0 {
			return next
		}

		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			// Determine which providers apply to this tool.
			type fetchJob struct {
				key      string
				provider PrefetchProvider
			}

			var jobs []fetchJob
			now := time.Now()

			mu.RLock()
			for key, provider := range cfg.Providers {
				if provider.ShouldPrefetch != nil && !provider.ShouldPrefetch(name) {
					continue
				}
				// Check cache.
				if entry, ok := cache[key]; ok && now.Before(entry.expiresAt) {
					// Cache hit — inject into context directly.
					ctx = context.WithValue(ctx, contextKey(key), entry.value)
					continue
				}
				// Cache miss or expired — schedule fetch.
				jobs = append(jobs, fetchJob{key: key, provider: provider})
			}
			mu.RUnlock()

			// Execute fetches concurrently with bounded parallelism.
			if len(jobs) > 0 {
				type fetchResult struct {
					key   string
					value any
					err   error
				}

				results := make([]fetchResult, len(jobs))
				sem := make(chan struct{}, maxConc)
				var wg sync.WaitGroup

				for i, job := range jobs {
					wg.Add(1)
					go func(idx int, j fetchJob) {
						defer wg.Done()
						sem <- struct{}{}
						defer func() { <-sem }()

						val, err := j.provider.Fetch(ctx)
						results[idx] = fetchResult{
							key:   j.key,
							value: val,
							err:   err,
						}
					}(i, job)
				}
				wg.Wait()

				// Process results: cache successes and inject into context.
				mu.Lock()
				for _, r := range results {
					if r.err != nil {
						// Graceful degradation: skip failed providers.
						continue
					}
					cache[r.key] = cacheEntry{
						value:     r.value,
						expiresAt: now.Add(ttl),
					}
					ctx = context.WithValue(ctx, contextKey(r.key), r.value)
				}
				mu.Unlock()
			}

			return next(ctx, req)
		}
	}
}

// PrefetchFromContext retrieves pre-fetched data from context with type safety.
// Returns the zero value of T and false if the key is not present or the type
// does not match.
func PrefetchFromContext[T any](ctx context.Context, key string) (T, bool) {
	val := ctx.Value(contextKey(key))
	if val == nil {
		var zero T
		return zero, false
	}
	typed, ok := val.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return typed, true
}

// Option configures the prefetch middleware via functional options.
type Option func(*Config)

// WithProvider adds a named provider with the given fetch function and filter.
func WithProvider(key string, fetch func(ctx context.Context) (any, error), shouldPrefetch func(toolName string) bool) Option {
	return func(c *Config) {
		if c.Providers == nil {
			c.Providers = make(map[string]PrefetchProvider)
		}
		c.Providers[key] = PrefetchProvider{
			Fetch:          fetch,
			ShouldPrefetch: shouldPrefetch,
		}
	}
}

// WithCacheTTL sets the TTL for cached pre-fetched data.
func WithCacheTTL(d time.Duration) Option {
	return func(c *Config) { c.CacheTTL = d }
}

// WithMaxConcurrent sets the maximum number of parallel prefetch operations.
func WithMaxConcurrent(n int) Option {
	return func(c *Config) { c.MaxConcurrent = n }
}

// New returns a registry.Middleware configured with functional options,
// starting from DefaultConfig().
func New(opts ...Option) registry.Middleware {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return Middleware(cfg)
}
