// Package resilience provides fault-tolerance primitives for MCP tool handlers.
//
// It includes a three-state [CircuitBreaker] (closed/open/half-open) with
// configurable failure and success thresholds, a token-bucket [RateLimiter]
// with per-context blocking, a generic [CacheEntry] with TTL-based
// expiry and serialized concurrent fetches, and an [ErrorRecoveryMiddleware]
// that catches tool errors, formats them for LLM context windows, and
// supports automatic retry with escalation (12-Factor Agent Factor 9).
//
// Named instances are managed by [CircuitBreakerRegistry] and
// [RateLimitRegistry] so each upstream or tool group can have independent
// limits.
//
// Middleware integration: [RateLimitMiddleware], [CircuitBreakerMiddleware],
// and [ErrorRecoveryMiddleware] wrap any tool handler and are drop-in
// additions to the registry middleware chain.
//
// Example:
//
//	cb := resilience.NewCircuitBreaker("payments", resilience.DefaultCircuitBreakerConfig())
//	err := cb.Call(ctx, func() error {
//	    return chargeCard(ctx, amount)
//	})
package resilience
