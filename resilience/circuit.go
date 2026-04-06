package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Circuit breaker states.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failing fast
	CircuitHalfOpen                     // Testing recovery
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	HalfOpenMaxCalls int
	// OnCircuitOpen is called when the circuit transitions to the Open state.
	// Use this for human escalation, alerting, or logging. The callback receives
	// the circuit breaker name. It is called while holding the lock, so it
	// should return quickly and not block.
	OnCircuitOpen func(name string)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		HalfOpenMaxCalls: 1,
	}
}

// CircuitBreakerMetrics receives circuit breaker state change notifications.
// Implement this interface to plug in Prometheus, OTel, or any metrics system.
type CircuitBreakerMetrics interface {
	OnStateChange(name string, from, to CircuitState)
	OnRejection(name string)
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	name          string
	config        CircuitBreakerConfig
	metrics       CircuitBreakerMetrics
	mu            sync.RWMutex
	state         CircuitState
	failures      int
	successes     int
	lastFailure   time.Time
	halfOpenCalls int
}

// NewCircuitBreaker creates a new circuit breaker. Metrics is optional (can be nil).
func NewCircuitBreaker(name string, config CircuitBreakerConfig, metrics CircuitBreakerMetrics) *CircuitBreaker {
	return &CircuitBreaker{
		name:    name,
		config:  config,
		metrics: metrics,
		state:   CircuitClosed,
	}
}

// Execute runs the function through the circuit breaker.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if !cb.canExecute() {
		if cb.metrics != nil {
			cb.metrics.OnRejection(cb.name)
		}
		return fmt.Errorf("%s: %w", cb.name, ErrCircuitOpen)
	}

	err := fn(ctx)
	cb.recordResult(err)
	return err
}

// ExecuteWithResult runs a function that returns a result through the circuit breaker.
func ExecuteWithResult[T any](cb *CircuitBreaker, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if !cb.canExecute() {
		if cb.metrics != nil {
			cb.metrics.OnRejection(cb.name)
		}
		return zero, fmt.Errorf("%s: %w", cb.name, ErrCircuitOpen)
	}

	result, err := fn(ctx)
	cb.recordResult(err)
	return result, err
}

func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.config.Timeout {
			cb.transition(CircuitHalfOpen)
			cb.halfOpenCalls = 1
			return true
		}
		return false
	case CircuitHalfOpen:
		if cb.halfOpenCalls < cb.config.HalfOpenMaxCalls {
			cb.halfOpenCalls++
			return true
		}
		return false
	}
	return false
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.successes = 0
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.transition(CircuitOpen)
		}
	case CircuitHalfOpen:
		cb.transition(CircuitOpen)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case CircuitClosed:
		cb.failures = 0
	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transition(CircuitClosed)
		}
	}
}

func (cb *CircuitBreaker) transition(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenCalls = 0

	if cb.metrics != nil {
		cb.metrics.OnStateChange(cb.name, oldState, newState)
	}

	// Fire the OnCircuitOpen callback when transitioning to Open state.
	if newState == CircuitOpen && cb.config.OnCircuitOpen != nil {
		cb.config.OnCircuitOpen(cb.name)
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state != CircuitClosed {
		cb.transition(CircuitClosed)
	}
}

// CircuitBreakerRegistry manages multiple circuit breakers.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
	metrics  CircuitBreakerMetrics
}

// NewCircuitBreakerRegistry creates a new registry with default config.
// Metrics is optional (can be nil).
func NewCircuitBreakerRegistry(metrics CircuitBreakerMetrics, config ...CircuitBreakerConfig) *CircuitBreakerRegistry {
	cfg := DefaultCircuitBreakerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   cfg,
		metrics:  metrics,
	}
}

// Get returns or creates a circuit breaker for the given service.
func (r *CircuitBreakerRegistry) Get(service string) *CircuitBreaker {
	r.mu.RLock()
	cb, exists := r.breakers[service]
	r.mu.RUnlock()
	if exists {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, exists = r.breakers[service]; exists {
		return cb
	}

	cb = NewCircuitBreaker(service, r.config, r.metrics)
	r.breakers[service] = cb
	return cb
}

// Configure sets the config for a specific service.
func (r *CircuitBreakerRegistry) Configure(service string, config CircuitBreakerConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, exists := r.breakers[service]; exists {
		cb.config = config
	} else {
		r.breakers[service] = NewCircuitBreaker(service, config, r.metrics)
	}
}

// Status returns the state of all circuit breakers.
func (r *CircuitBreakerRegistry) Status() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	status := make(map[string]string)
	for name, cb := range r.breakers {
		status[name] = cb.State().String()
	}
	return status
}
