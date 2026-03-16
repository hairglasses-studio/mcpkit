package ralph

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the current state of the circuit breaker.
type CircuitState string

const (
	// CircuitClosed is the normal operating state — requests pass through.
	CircuitClosed CircuitState = "closed"
	// CircuitHalfOpen is the probing state after a cooldown — one attempt is allowed.
	CircuitHalfOpen CircuitState = "half_open"
	// CircuitOpen is the tripped state — requests are blocked until cooldown expires.
	CircuitOpen CircuitState = "open"
)

// CircuitBreakerConfig holds the tuning parameters for the circuit breaker.
type CircuitBreakerConfig struct {
	// NoProgressThreshold is the number of consecutive no-progress iterations
	// before opening the circuit. Default 3.
	NoProgressThreshold int
	// SameErrorThreshold is the number of consecutive iterations with the same
	// error key before opening the circuit. Default 5.
	SameErrorThreshold int
	// CooldownDuration is how long the circuit stays open before transitioning
	// to half-open. Default 30 minutes.
	CooldownDuration time.Duration
}

// DefaultCircuitBreakerConfig returns a CircuitBreakerConfig with sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		NoProgressThreshold: 3,
		SameErrorThreshold:  5,
		CooldownDuration:    30 * time.Minute,
	}
}

// CircuitBreaker is a 3-state circuit breaker that prevents runaway loops.
// It monitors progress and error patterns, opening when thresholds are exceeded.
type CircuitBreaker struct {
	mu              sync.Mutex
	config          CircuitBreakerConfig
	state           CircuitState
	noProgressCount int
	sameErrorCount  int
	lastErrorKey    string
	openedAt        time.Time
	openReason      string
}

// NewCircuitBreaker creates a new CircuitBreaker with the given config.
// Zero values in cfg are replaced with defaults.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.NoProgressThreshold <= 0 {
		cfg.NoProgressThreshold = DefaultCircuitBreakerConfig().NoProgressThreshold
	}
	if cfg.SameErrorThreshold <= 0 {
		cfg.SameErrorThreshold = DefaultCircuitBreakerConfig().SameErrorThreshold
	}
	if cfg.CooldownDuration <= 0 {
		cfg.CooldownDuration = DefaultCircuitBreakerConfig().CooldownDuration
	}
	return &CircuitBreaker{
		config: cfg,
		state:  CircuitClosed,
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// CanExecute reports whether execution should proceed.
// Returns false when the circuit is OPEN and the cooldown has not elapsed.
// Transitions the circuit from OPEN to HALF_OPEN when the cooldown expires.
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == CircuitOpen {
		if time.Since(cb.openedAt) >= cb.config.CooldownDuration {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	}
	return true
}

// CooldownRemaining returns how long until the circuit transitions to half-open.
// Returns 0 if the circuit is not open or the cooldown has already elapsed.
func (cb *CircuitBreaker) CooldownRemaining() time.Duration {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state != CircuitOpen {
		return 0
	}
	remaining := cb.config.CooldownDuration - time.Since(cb.openedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RecordResult updates the circuit breaker state based on the outcome of one iteration.
// hasProgress should be true when meaningful work was done (e.g. a task was completed).
// errorKey should be a stable string identifying the error class (empty string if no error).
func (cb *CircuitBreaker) RecordResult(hasProgress bool, errorKey string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if hasProgress {
		// Progress resets all counters and closes the circuit.
		cb.noProgressCount = 0
		cb.sameErrorCount = 0
		cb.lastErrorKey = ""
		if cb.state == CircuitHalfOpen {
			cb.state = CircuitClosed
		}
		return
	}

	// No progress: increment the no-progress counter.
	cb.noProgressCount++

	// Track same-error streak.
	if errorKey != "" {
		if errorKey == cb.lastErrorKey {
			cb.sameErrorCount++
		} else {
			// Different error breaks the streak.
			cb.sameErrorCount = 1
			cb.lastErrorKey = errorKey
		}
	} else {
		// No error key provided; reset same-error tracking.
		cb.sameErrorCount = 0
		cb.lastErrorKey = ""
	}

	// Check thresholds.
	if cb.noProgressCount >= cb.config.NoProgressThreshold {
		cb.trip(fmt.Sprintf("no progress for %d consecutive iterations", cb.noProgressCount))
		return
	}
	if errorKey != "" && cb.sameErrorCount >= cb.config.SameErrorThreshold {
		cb.trip(fmt.Sprintf("same error %q repeated %d times", errorKey, cb.sameErrorCount))
		return
	}

	// Half-open failure: reopen the circuit.
	if cb.state == CircuitHalfOpen {
		cb.trip("probe iteration failed in half-open state")
	}
}

// trip opens the circuit and records the reason. Caller must hold mu.
func (cb *CircuitBreaker) trip(reason string) {
	cb.state = CircuitOpen
	cb.openedAt = time.Now()
	cb.openReason = reason
}

// Reset returns the circuit to the closed state and clears all counters.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.noProgressCount = 0
	cb.sameErrorCount = 0
	cb.lastErrorKey = ""
	cb.openedAt = time.Time{}
	cb.openReason = ""
}

// OpenReason returns a human-readable explanation of why the circuit was opened.
// Returns an empty string if the circuit is not open.
func (cb *CircuitBreaker) OpenReason() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state != CircuitOpen && cb.state != CircuitHalfOpen {
		return ""
	}
	return cb.openReason
}
