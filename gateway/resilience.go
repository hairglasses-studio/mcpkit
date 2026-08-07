//go:build !official_sdk

package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resilience"
)

// ResponseCacheConfig enables per-input-key response caching for an
// upstream's tool calls. Results are cached on an exact-match key of tool
// name + a hash of canonicalized arguments (resilience.CacheKey), so two
// calls with different arguments never collide — see
// docs/research/RESEARCH-AI-GATEWAY.md:434 (per-input caching, listed there
// as high-value/low-complexity). Only successful (non-error) results are
// cached; only idempotent tools should have this enabled.
type ResponseCacheConfig struct {
	// TTL is how long a cached response stays fresh.
	TTL time.Duration
	// MaxEntries bounds the cache size (LRU eviction). <=0 is treated as 1.
	MaxEntries int
}

// UpstreamPolicy configures per-upstream resilience. Nil fields disable
// that protection layer.
type UpstreamPolicy struct {
	// CircuitBreaker opens after repeated failures, preventing calls to a
	// failing upstream. Nil disables circuit breaking.
	CircuitBreaker *resilience.CircuitBreakerConfig

	// RateLimit restricts the request rate to the upstream. Nil disables
	// rate limiting.
	RateLimit *resilience.RateLimitConfig

	// CallTimeout is applied per proxied tool call. Zero means no per-call
	// timeout (the caller's context deadline still applies).
	CallTimeout time.Duration

	// ResponseCache enables per-input-key response caching. Nil disables
	// caching — opt-in only, zero behavior change when unset.
	ResponseCache *ResponseCacheConfig
}

// upstreamResilience holds the per-upstream resilience instances.
type upstreamResilience struct {
	cb      *resilience.CircuitBreaker
	limiter *resilience.RateLimiter
	timeout time.Duration
	cache   *resilience.KeyedCache[*mcp.CallToolResult]
}

// newUpstreamResilience creates resilience instances from a policy.
// Returns nil if all policy fields are nil/zero.
func newUpstreamResilience(name string, policy UpstreamPolicy) *upstreamResilience {
	if policy.CircuitBreaker == nil && policy.RateLimit == nil && policy.CallTimeout == 0 && policy.ResponseCache == nil {
		return nil
	}
	ur := &upstreamResilience{timeout: policy.CallTimeout}
	if policy.CircuitBreaker != nil {
		ur.cb = resilience.NewCircuitBreaker(name, *policy.CircuitBreaker, nil)
	}
	if policy.RateLimit != nil {
		ur.limiter = resilience.NewRateLimiter(policy.RateLimit.Rate, policy.RateLimit.Burst)
	}
	if policy.ResponseCache != nil {
		ur.cache = resilience.NewKeyedCache[*mcp.CallToolResult](policy.ResponseCache.TTL, policy.ResponseCache.MaxEntries)
	}
	return ur
}

// wrapHandler wraps a proxy handler with response caching, rate limiting,
// timeout, and circuit breaking in that order (outermost to innermost). A
// cache hit short-circuits everything inside it — no rate limit consumed,
// no upstream call made.
func (ur *upstreamResilience) wrapHandler(upstreamName string, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
	if ur == nil {
		return next
	}
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 0. Response cache check
		var cacheKey string
		if ur.cache != nil {
			cacheKey = resilience.CacheKey(request.Params.Name, request.GetArguments())
			if cached, ok := ur.cache.Get(cacheKey); ok {
				return cached, nil
			}
		}

		// 1. Rate limit check
		if ur.limiter != nil {
			if !ur.limiter.Allow() {
				return registry.MakeErrorResult(
					fmt.Sprintf("upstream %q rate limit exceeded", upstreamName),
				), nil
			}
		}

		// 2. Apply per-call timeout
		if ur.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, ur.timeout)
			defer cancel()
		}

		// 3. Circuit breaker
		var result *mcp.CallToolResult
		var err error
		if ur.cb != nil {
			result, err = resilience.ExecuteWithResult(ur.cb, ctx, func(cbCtx context.Context) (*mcp.CallToolResult, error) {
				return next(cbCtx, request)
			})
			if errors.Is(err, resilience.ErrCircuitOpen) {
				return registry.MakeErrorResult(
					fmt.Sprintf("upstream %q circuit breaker is open", upstreamName),
				), nil
			}
		} else {
			result, err = next(ctx, request)
		}

		// Only cache clean successes — a transient upstream error or an
		// MCP-level error result must not get served back on repeat.
		if ur.cache != nil && err == nil && result != nil && !result.IsError {
			ur.cache.Set(cacheKey, result)
		}
		return result, err
	}
}

// circuitState returns the circuit breaker state string, or empty if no
// circuit breaker is configured.
func (ur *upstreamResilience) circuitState() string {
	if ur == nil || ur.cb == nil {
		return ""
	}
	return ur.cb.State().String()
}
