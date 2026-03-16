package rdcycle

import (
	"sync"
	"time"
)

// BreakerState represents the current state of the circuit breaker.
type BreakerState int

const (
	BreakerClosed   BreakerState = iota // normal operation
	BreakerOpen                          // halted, waiting for cooldown
	BreakerHalfOpen                      // testing after cooldown
)

func (s BreakerState) String() string {
	switch s {
	case BreakerClosed:
		return "closed"
	case BreakerOpen:
		return "open"
	case BreakerHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements a three-state circuit breaker at the R&D cycle level.
// When consecutive cycles make no progress, the breaker opens to prevent runaway loops.
// After a cooldown period it transitions to half-open, allowing a single probe cycle.
type CircuitBreaker struct {
	mu                  sync.Mutex
	state               BreakerState
	noProgressCount     int
	noProgressThreshold int
	cooldownDuration    time.Duration
	openedAt            time.Time
}

// NewCircuitBreaker creates a circuit breaker with the given thresholds.
// noProgressThreshold is the number of consecutive no-progress cycles before opening.
// cooldownDuration is how long to wait before transitioning from OPEN to HALF_OPEN.
func NewCircuitBreaker(noProgressThreshold int, cooldownDuration time.Duration) *CircuitBreaker {
	if noProgressThreshold <= 0 {
		noProgressThreshold = 3
	}
	if cooldownDuration <= 0 {
		cooldownDuration = 30 * time.Minute
	}
	return &CircuitBreaker{
		state:               BreakerClosed,
		noProgressThreshold: noProgressThreshold,
		cooldownDuration:    cooldownDuration,
	}
}

// CanExecute returns true if the breaker allows a cycle to run.
// When OPEN, it checks if cooldown has elapsed and transitions to HALF_OPEN.
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case BreakerClosed:
		return true
	case BreakerHalfOpen:
		return true
	case BreakerOpen:
		if time.Since(cb.openedAt) >= cb.cooldownDuration {
			cb.state = BreakerHalfOpen
			return true
		}
		return false
	}
	return false
}

// RecordResult records the outcome of a cycle. If progress is false, the no-progress
// counter increments. If errRepeated is true, it counts as no progress regardless.
// When the threshold is reached, the breaker opens. A successful cycle resets the counter.
func (cb *CircuitBreaker) RecordResult(progress bool, errRepeated bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !progress || errRepeated {
		cb.noProgressCount++
		if cb.noProgressCount >= cb.noProgressThreshold {
			cb.state = BreakerOpen
			cb.openedAt = time.Now()
		}
		return
	}

	// Successful cycle: reset counter and close breaker.
	cb.noProgressCount = 0
	cb.state = BreakerClosed
}

// State returns the current breaker state.
func (cb *CircuitBreaker) State() BreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Reset forces the breaker back to CLOSED with zero no-progress count.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = BreakerClosed
	cb.noProgressCount = 0
	cb.openedAt = time.Time{}
}

// NoProgressCount returns the current consecutive no-progress count.
func (cb *CircuitBreaker) NoProgressCount() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.noProgressCount
}
