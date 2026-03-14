# resilience

Fault-tolerance primitives. Depends only on `registry`.

## Components

- **CircuitBreaker** (`circuit.go`): states Closed→Open→HalfOpen, configurable thresholds/timeouts, `CircuitBreakerRegistry` for per-service instances
- **RateLimiter** (`ratelimit.go`): token bucket with `Wait(ctx)`, `RateLimitRegistry` for per-service instances
- **CacheEntry[T]** (`cache.go`): generic TTL cache with `GetOrFetch(ctx, fetchFn)`, serialized concurrent fetches
- **Middleware** (`middleware.go`): `RateLimitMiddleware(reg)` and `CircuitBreakerMiddleware(reg)` — both no-op if tool has no `CircuitBreakerGroup`

All types use `sync.RWMutex` for thread safety.
