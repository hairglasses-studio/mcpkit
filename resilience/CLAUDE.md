# resilience

Fault-tolerance primitives. Depends only on `registry`.

## Components

- **CircuitBreaker** (`circuit.go`): states Closed→Open→HalfOpen, configurable thresholds/timeouts, `CircuitBreakerRegistry` for per-service instances
- **RateLimiter** (`ratelimit.go`): token bucket with `Wait(ctx)`, `RateLimitRegistry` for per-service instances
- **CacheEntry[T]** (`cache.go`): generic TTL cache with `GetOrFetch(ctx, fetchFn)`, serialized concurrent fetches
- **ErrorRecovery** (`error_recovery.go`): 12-Factor Agent Factor 9 — catches tool errors, classifies them (TIMEOUT/NETWORK/RATE_LIMITED/PERMISSION/NOT_FOUND/CIRCUIT_OPEN/CANCELLED/TRANSIENT), formats compact LLM-readable messages with recovery hints, auto-retries with configurable `ShouldRetry`/`MaxRetries`/`RetryDelay`, escalation callback when retries exhausted. Always returns `(*CallToolResult, nil)` — never `(nil, error)`.
- **Middleware** (`middleware.go`): `RateLimitMiddleware(reg)` and `CircuitBreakerMiddleware(reg)` — both no-op if tool has no `CircuitBreakerGroup`

All types use `sync.RWMutex` for thread safety.
