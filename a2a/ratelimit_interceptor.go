package a2a

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimitInterceptor enforces per-agent rate limits on A2A requests.
// Each agent URL gets its own token bucket with configurable rate and burst.
type RateLimitInterceptor struct {
	mu       sync.RWMutex
	limiters map[string]*tokenBucket
	config   RateLimitConfig
}

// RateLimitConfig configures per-agent rate limiting.
type RateLimitConfig struct {
	// DefaultRate is tokens per second for unknown agents.
	DefaultRate float64
	// DefaultBurst is the maximum burst size.
	DefaultBurst int
}

// NewRateLimitInterceptor creates a rate limit interceptor.
func NewRateLimitInterceptor(cfg RateLimitConfig) *RateLimitInterceptor {
	if cfg.DefaultRate <= 0 {
		cfg.DefaultRate = 10.0
	}
	if cfg.DefaultBurst <= 0 {
		cfg.DefaultBurst = 20
	}
	return &RateLimitInterceptor{
		limiters: make(map[string]*tokenBucket),
		config:   cfg,
	}
}

// Allow checks if a request to the given agent URL is allowed.
// Returns nil if allowed, error if rate limit exceeded.
func (r *RateLimitInterceptor) Allow(agentURL string) error {
	r.mu.RLock()
	limiter, ok := r.limiters[agentURL]
	r.mu.RUnlock()

	if !ok {
		r.mu.Lock()
		limiter, ok = r.limiters[agentURL]
		if !ok {
			limiter = newTokenBucket(r.config.DefaultRate, r.config.DefaultBurst)
			r.limiters[agentURL] = limiter
		}
		r.mu.Unlock()
	}

	if !limiter.allow() {
		return fmt.Errorf("rate limit exceeded for agent %s (%.0f req/s, burst %d)",
			agentURL, r.config.DefaultRate, r.config.DefaultBurst)
	}
	return nil
}

// SetAgentRate configures a custom rate for a specific agent.
func (r *RateLimitInterceptor) SetAgentRate(agentURL string, rate float64, burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[agentURL] = newTokenBucket(rate, burst)
}

// RateLimitedClient wraps an A2A Client with rate limiting.
type RateLimitedClient struct {
	inner   *Client
	limiter *RateLimitInterceptor
}

// NewRateLimitedClient wraps a client with rate limiting.
func NewRateLimitedClient(inner *Client, limiter *RateLimitInterceptor) *RateLimitedClient {
	return &RateLimitedClient{inner: inner, limiter: limiter}
}

// SendTask sends a task with rate limiting.
func (rc *RateLimitedClient) SendTask(ctx context.Context, params TaskSendParams) (*Task, error) {
	if err := rc.limiter.Allow(rc.inner.baseURL); err != nil {
		return nil, err
	}
	return rc.inner.SendTask(ctx, params)
}

// GetTask fetches task status with rate limiting.
func (rc *RateLimitedClient) GetTask(ctx context.Context, taskID string) (*Task, error) {
	if err := rc.limiter.Allow(rc.inner.baseURL); err != nil {
		return nil, err
	}
	return rc.inner.GetTask(ctx, taskID)
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	mu       sync.Mutex
	rate     float64 // tokens per second
	burst    int
	tokens   float64
	lastTime time.Time
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		rate:     rate,
		burst:    burst,
		tokens:   float64(burst),
		lastTime: time.Now(),
	}
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}
	tb.lastTime = now

	if tb.tokens >= 1.0 {
		tb.tokens--
		return true
	}
	return false
}
