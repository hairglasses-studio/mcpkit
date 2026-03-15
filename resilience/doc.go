// Package resilience provides fault-tolerance primitives for MCP tool handlers.
//
// It includes a three-state [CircuitBreaker] (closed/open/half-open) with
// configurable failure and success thresholds, a token-bucket [RateLimiter]
// with per-context blocking, and a generic [CacheEntry] with TTL-based
// expiry and serialized concurrent fetches. Named instances are managed by
// [CircuitBreakerRegistry] and [RateLimitRegistry] so each upstream or tool
// group can have independent limits.
//
// Middleware integration: [RateLimitMiddleware] and [CircuitBreakerMiddleware]
// wrap any tool whose [registry.ToolDefinition] carries a CircuitBreakerGroup,
// making them drop-in additions to the registry middleware chain.
//
// Example:
//
//	cb := resilience.NewCircuitBreaker("payments", resilience.DefaultCircuitBreakerConfig())
//	err := cb.Call(ctx, func() error {
//	    return chargeCard(ctx, amount)
//	})
package resilience
