package ralph

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
	if !cb.CanExecute() {
		t.Error("expected CanExecute=true when closed")
	}
}

func TestCircuitBreaker_OpensOnNoProgressThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 3,
		SameErrorThreshold:  10,
		CooldownDuration:    time.Hour,
	})
	for i := 0; i < 2; i++ {
		cb.RecordResult(false, "")
		if cb.State() != CircuitClosed {
			t.Errorf("iteration %d: expected still closed", i+1)
		}
	}
	cb.RecordResult(false, "")
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after threshold, got %s", cb.State())
	}
	if cb.CanExecute() {
		t.Error("expected CanExecute=false when open with long cooldown")
	}
}

func TestCircuitBreaker_OpensOnSameErrorThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 100,
		SameErrorThreshold:  3,
		CooldownDuration:    time.Hour,
	})
	for i := 0; i < 2; i++ {
		cb.RecordResult(false, "tool not found")
	}
	if cb.State() != CircuitClosed {
		t.Error("expected closed before threshold")
	}
	cb.RecordResult(false, "tool not found")
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after same-error threshold, got %s", cb.State())
	}
}

func TestCircuitBreaker_DifferentErrorsResetStreak(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 100,
		SameErrorThreshold:  3,
		CooldownDuration:    time.Hour,
	})
	cb.RecordResult(false, "error-a")
	cb.RecordResult(false, "error-a")
	// Different error resets the streak.
	cb.RecordResult(false, "error-b")
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after streak reset, got %s", cb.State())
	}
}

func TestCircuitBreaker_ProgressResetsCounters(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 3,
		SameErrorThreshold:  3,
		CooldownDuration:    time.Hour,
	})
	cb.RecordResult(false, "err")
	cb.RecordResult(false, "err")
	// Progress should reset.
	cb.RecordResult(true, "")
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after progress, got %s", cb.State())
	}
	// Two more no-progress should not trip (counters reset).
	cb.RecordResult(false, "err")
	cb.RecordResult(false, "err")
	if cb.State() != CircuitClosed {
		t.Errorf("expected still closed after partial no-progress, got %s", cb.State())
	}
}

func TestCircuitBreaker_CooldownTransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 1,
		SameErrorThreshold:  10,
		CooldownDuration:    1 * time.Millisecond,
	})
	cb.RecordResult(false, "")
	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}
	// Wait for cooldown.
	time.Sleep(5 * time.Millisecond)
	if !cb.CanExecute() {
		t.Error("expected CanExecute=true after cooldown")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected half-open after cooldown, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenClosesOnProgress(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 1,
		SameErrorThreshold:  10,
		CooldownDuration:    1 * time.Millisecond,
	})
	cb.RecordResult(false, "")
	time.Sleep(5 * time.Millisecond)
	cb.CanExecute() // transitions to half-open
	cb.RecordResult(true, "")
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after progress in half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenReOpensOnFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 1,
		SameErrorThreshold:  10,
		CooldownDuration:    1 * time.Millisecond,
	})
	cb.RecordResult(false, "")
	time.Sleep(5 * time.Millisecond)
	cb.CanExecute() // transitions to half-open
	cb.RecordResult(false, "")
	if cb.State() != CircuitOpen {
		t.Errorf("expected re-opened after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreaker_CooldownRemaining(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 1,
		SameErrorThreshold:  10,
		CooldownDuration:    time.Hour,
	})
	if cb.CooldownRemaining() != 0 {
		t.Error("expected 0 when closed")
	}
	cb.RecordResult(false, "")
	remaining := cb.CooldownRemaining()
	if remaining <= 0 || remaining > time.Hour {
		t.Errorf("unexpected cooldown remaining: %v", remaining)
	}
}

func TestCircuitBreaker_OpenReason(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 2,
		SameErrorThreshold:  10,
		CooldownDuration:    time.Hour,
	})
	if cb.OpenReason() != "" {
		t.Error("expected empty reason when closed")
	}
	cb.RecordResult(false, "")
	cb.RecordResult(false, "")
	reason := cb.OpenReason()
	if reason == "" {
		t.Error("expected non-empty reason when open")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 1,
		SameErrorThreshold:  10,
		CooldownDuration:    time.Hour,
	})
	cb.RecordResult(false, "")
	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}
	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after reset, got %s", cb.State())
	}
	if !cb.CanExecute() {
		t.Error("expected CanExecute=true after reset")
	}
	if cb.OpenReason() != "" {
		t.Error("expected empty reason after reset")
	}
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	if cfg.NoProgressThreshold != 3 {
		t.Errorf("expected NoProgressThreshold=3, got %d", cfg.NoProgressThreshold)
	}
	if cfg.SameErrorThreshold != 5 {
		t.Errorf("expected SameErrorThreshold=5, got %d", cfg.SameErrorThreshold)
	}
	if cfg.CooldownDuration != 30*time.Minute {
		t.Errorf("expected CooldownDuration=30m, got %v", cfg.CooldownDuration)
	}
}

func TestCircuitBreaker_ZeroConfigUsesDefaults(t *testing.T) {
	// NewCircuitBreaker with zero config should apply defaults.
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	// Trip using the default no-progress threshold (3).
	for i := 0; i < 3; i++ {
		cb.RecordResult(false, "")
	}
	if cb.State() != CircuitOpen {
		t.Errorf("expected open with default threshold, got %s", cb.State())
	}
}
