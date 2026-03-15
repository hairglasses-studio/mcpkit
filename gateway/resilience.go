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
}

// upstreamResilience holds the per-upstream resilience instances.
type upstreamResilience struct {
	cb      *resilience.CircuitBreaker
	limiter *resilience.RateLimiter
	timeout time.Duration
}

// newUpstreamResilience creates resilience instances from a policy.
// Returns nil if all policy fields are nil/zero.
func newUpstreamResilience(name string, policy UpstreamPolicy) *upstreamResilience {
	if policy.CircuitBreaker == nil && policy.RateLimit == nil && policy.CallTimeout == 0 {
		return nil
	}
	ur := &upstreamResilience{timeout: policy.CallTimeout}
	if policy.CircuitBreaker != nil {
		ur.cb = resilience.NewCircuitBreaker(name, *policy.CircuitBreaker, nil)
	}
	if policy.RateLimit != nil {
		ur.limiter = resilience.NewRateLimiter(policy.RateLimit.Rate, policy.RateLimit.Burst)
	}
	return ur
}

// wrapHandler wraps a proxy handler with rate limiting, timeout, and circuit
// breaking in that order (outermost to innermost).
func (ur *upstreamResilience) wrapHandler(upstreamName string, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
	if ur == nil {
		return next
	}
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		if ur.cb != nil {
			result, err := resilience.ExecuteWithResult(ur.cb, ctx, func(cbCtx context.Context) (*mcp.CallToolResult, error) {
				return next(cbCtx, request)
			})
			if errors.Is(err, resilience.ErrCircuitOpen) {
				return registry.MakeErrorResult(
					fmt.Sprintf("upstream %q circuit breaker is open", upstreamName),
				), nil
			}
			return result, err
		}

		return next(ctx, request)
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
