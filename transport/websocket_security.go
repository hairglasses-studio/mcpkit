package transport

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Default security limits.
const (
	// DefaultMaxMessageSize is the default maximum message size in bytes (1 MB).
	DefaultMaxMessageSize int64 = 1 << 20

	// DefaultMessageRateLimit is the default max messages per second per connection.
	DefaultMessageRateLimit = 100

	// DefaultIdleTimeout is the default idle timeout for connections.
	DefaultIdleTimeout = 5 * time.Minute
)

// WebSocketSecurityConfig configures WebSocket security policies.
type WebSocketSecurityConfig struct {
	// AllowedOrigins is the list of allowed Origin headers.
	// Empty = allow all (INSECURE, dev only).
	AllowedOrigins []string

	// MaxMessageSize in bytes (default: 1 MB).
	MaxMessageSize int64

	// MessageRateLimit is max messages per second per connection.
	MessageRateLimit int

	// IdleTimeout disconnects idle connections (default: 5 min).
	IdleTimeout time.Duration

	// AuthValidator validates the auth token from the upgrade request.
	// Return nil to allow, error to reject.
	AuthValidator func(token string) error

	// RequireAuth if true, rejects connections without an auth token.
	RequireAuth bool
}

// applyDefaults fills in zero-valued fields with secure defaults.
func (c *WebSocketSecurityConfig) applyDefaults() {
	if c.MaxMessageSize <= 0 {
		c.MaxMessageSize = DefaultMaxMessageSize
	}
	if c.MessageRateLimit <= 0 {
		c.MessageRateLimit = DefaultMessageRateLimit
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = DefaultIdleTimeout
	}
}

// ValidateOrigin checks the Origin header against allowed origins.
// An empty allowed list permits all origins (dev-only behaviour).
// A wildcard entry "*" in the allowed list also permits all origins.
func ValidateOrigin(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}

// SecureUpgradeHandler wraps an http.Handler with WebSocket security checks.
// It validates the Origin header and optional auth token before passing
// the request to the inner handler. Requests that fail validation are
// rejected with an appropriate HTTP status code.
func SecureUpgradeHandler(cfg WebSocketSecurityConfig, handler http.Handler) http.Handler {
	cfg.applyDefaults()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Origin validation.
		origin := r.Header.Get("Origin")
		if !ValidateOrigin(origin, cfg.AllowedOrigins) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		// 2. Authentication.
		if cfg.RequireAuth {
			token := extractAuthToken(r)
			if token == "" {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			if cfg.AuthValidator != nil {
				if err := cfg.AuthValidator(token); err != nil {
					http.Error(w, fmt.Sprintf("authentication failed: %v", err), http.StatusUnauthorized)
					return
				}
			}
		} else if cfg.AuthValidator != nil {
			// Auth not required, but validate if a token is present.
			token := extractAuthToken(r)
			if token != "" {
				if err := cfg.AuthValidator(token); err != nil {
					http.Error(w, fmt.Sprintf("authentication failed: %v", err), http.StatusUnauthorized)
					return
				}
			}
		}

		// 3. Store config in request context for downstream use.
		ctx := r.Context()
		ctx = withSecurityConfig(ctx, &cfg)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractAuthToken extracts a bearer token from the Authorization header,
// falling back to the "token" query parameter.
func extractAuthToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if auth != "" {
		return auth
	}
	return r.URL.Query().Get("token")
}

// securityConfigKey is the context key for the security config.
type securityConfigKey struct{}

// withSecurityConfig stores the config in the request context.
func withSecurityConfig(ctx context.Context, cfg *WebSocketSecurityConfig) context.Context {
	return context.WithValue(ctx, securityConfigKey{}, cfg)
}

// SecurityConfigFromContext retrieves the WebSocketSecurityConfig from a
// request context, if present.
func SecurityConfigFromContext(r *http.Request) *WebSocketSecurityConfig {
	v := r.Context().Value(securityConfigKey{})
	if cfg, ok := v.(*WebSocketSecurityConfig); ok {
		return cfg
	}
	return nil
}

// ConnectionRateLimiter tracks per-connection message rates using a
// sliding-window token bucket approach.
type ConnectionRateLimiter struct {
	mu        sync.Mutex
	maxPerSec int
	tokens    int
	lastReset time.Time
	clock     func() time.Time // for testing
}

// NewConnectionRateLimiter creates a rate limiter that allows maxPerSec
// messages per second.
func NewConnectionRateLimiter(maxPerSec int) *ConnectionRateLimiter {
	return &ConnectionRateLimiter{
		maxPerSec: maxPerSec,
		tokens:    maxPerSec,
		lastReset: time.Now(),
		clock:     time.Now,
	}
}

// Allow reports whether a message is allowed under the rate limit.
// It consumes one token if allowed.
func (rl *ConnectionRateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.clock()
	elapsed := now.Sub(rl.lastReset)

	// Refill tokens based on elapsed time.
	if elapsed >= time.Second {
		rl.tokens = rl.maxPerSec
		rl.lastReset = now
	}

	if rl.tokens <= 0 {
		return false
	}
	rl.tokens--
	return true
}

// Reset restores all tokens.
func (rl *ConnectionRateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.tokens = rl.maxPerSec
	rl.lastReset = rl.clock()
}

// SetClock overrides the time source for testing.
func (rl *ConnectionRateLimiter) SetClock(fn func() time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.clock = fn
}
