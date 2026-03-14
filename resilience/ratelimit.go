package resilience

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	rate     float64
	burst    int
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter creates a limiter that allows rate requests per second
// with a burst capacity of burst.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		rate:     rate,
		burst:    burst,
		tokens:   float64(burst),
		lastTime: time.Now(),
	}
}

// Wait blocks until a token is available or ctx is cancelled.
func (l *RateLimiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		l.refill()
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration(float64(time.Second) / l.rate)
		l.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// Allow reports whether a token is available without blocking.
func (l *RateLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

func (l *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastTime).Seconds()
	l.tokens += elapsed * l.rate
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}
	l.lastTime = now
}

// RateLimitConfig defines rate limit parameters for a service.
type RateLimitConfig struct {
	Rate  float64
	Burst int
}

// RateLimitRegistry manages per-service rate limiters.
type RateLimitRegistry struct {
	mu            sync.RWMutex
	limiters      map[string]*RateLimiter
	defaultConfig RateLimitConfig
}

// NewRateLimitRegistry creates a new registry with the given default config.
// If no default is provided, uses 10 req/s with burst of 20.
func NewRateLimitRegistry(defaultConfig ...RateLimitConfig) *RateLimitRegistry {
	cfg := RateLimitConfig{Rate: 10, Burst: 20}
	if len(defaultConfig) > 0 {
		cfg = defaultConfig[0]
	}
	return &RateLimitRegistry{
		limiters:      make(map[string]*RateLimiter),
		defaultConfig: cfg,
	}
}

// Get returns the rate limiter for the named service,
// creating one with default config if it doesn't exist.
func (r *RateLimitRegistry) Get(service string) *RateLimiter {
	r.mu.RLock()
	lim, ok := r.limiters[service]
	r.mu.RUnlock()
	if ok {
		return lim
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if lim, ok = r.limiters[service]; ok {
		return lim
	}

	lim = NewRateLimiter(r.defaultConfig.Rate, r.defaultConfig.Burst)
	r.limiters[service] = lim
	return lim
}

// Configure sets a custom rate limit for a service, replacing any existing limiter.
func (r *RateLimitRegistry) Configure(service string, rate float64, burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[service] = NewRateLimiter(rate, burst)
}
