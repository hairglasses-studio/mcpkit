package rdcycle

import (
	"testing"
	"time"
)

func TestCircuitBreakerDefaults(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)
	if cb.noProgressThreshold != 3 {
		t.Errorf("expected default threshold 3, got %d", cb.noProgressThreshold)
	}
	if cb.cooldownDuration != 30*time.Minute {
		t.Errorf("expected default cooldown 30m, got %v", cb.cooldownDuration)
	}
}

func TestCircuitBreakerInitialState(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	if cb.State() != BreakerClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
	if !cb.CanExecute() {
		t.Error("expected CanExecute=true when closed")
	}
}

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	cb.RecordResult(false, false)
	if cb.State() != BreakerClosed {
		t.Fatalf("should still be closed after 1 failure, got %s", cb.State())
	}

	cb.RecordResult(false, false)
	if cb.State() != BreakerOpen {
		t.Fatalf("should be open after 2 failures, got %s", cb.State())
	}

	if cb.CanExecute() {
		t.Error("should not execute when open (cooldown not elapsed)")
	}
}

func TestCircuitBreakerErrRepeatedCountsAsNoProgress(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Hour)

	// Even with progress=true, errRepeated overrides.
	cb.RecordResult(true, true)
	cb.RecordResult(true, true)
	if cb.State() != BreakerOpen {
		t.Fatalf("errRepeated should count as no progress, got %s", cb.State())
	}
}

func TestCircuitBreakerProgressResetsCounter(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)

	cb.RecordResult(false, false)
	cb.RecordResult(false, false)
	// One success should reset.
	cb.RecordResult(true, false)
	if cb.NoProgressCount() != 0 {
		t.Errorf("expected count reset to 0, got %d", cb.NoProgressCount())
	}

	// Need 3 more failures to open.
	cb.RecordResult(false, false)
	cb.RecordResult(false, false)
	if cb.State() != BreakerClosed {
		t.Fatal("should still be closed with only 2 failures after reset")
	}
}

func TestCircuitBreakerCooldownTransition(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	cb.RecordResult(false, false)
	if cb.State() != BreakerOpen {
		t.Fatal("should be open")
	}

	// Wait for cooldown.
	time.Sleep(20 * time.Millisecond)

	if !cb.CanExecute() {
		t.Error("should be able to execute after cooldown")
	}
	if cb.State() != BreakerHalfOpen {
		t.Errorf("expected half_open after cooldown, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	cb.RecordResult(false, false) // open
	time.Sleep(20 * time.Millisecond)
	cb.CanExecute() // transitions to half-open

	cb.RecordResult(true, false) // success closes
	if cb.State() != BreakerClosed {
		t.Errorf("expected closed after half-open success, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	cb.RecordResult(false, false) // open
	time.Sleep(20 * time.Millisecond)
	cb.CanExecute() // half-open

	cb.RecordResult(false, false) // failure reopens
	if cb.State() != BreakerOpen {
		t.Errorf("expected open after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Hour)

	cb.RecordResult(false, false) // open
	cb.Reset()

	if cb.State() != BreakerClosed {
		t.Errorf("expected closed after reset, got %s", cb.State())
	}
	if cb.NoProgressCount() != 0 {
		t.Errorf("expected 0 count after reset, got %d", cb.NoProgressCount())
	}
	if !cb.CanExecute() {
		t.Error("should execute after reset")
	}
}

func TestBreakerStateString(t *testing.T) {
	tests := []struct {
		state BreakerState
		want  string
	}{
		{BreakerClosed, "closed"},
		{BreakerOpen, "open"},
		{BreakerHalfOpen, "half_open"},
		{BreakerState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("BreakerState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestCircuitBreakerConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(100, time.Hour)
	done := make(chan struct{})

	for range 10 {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := range 50 {
				cb.CanExecute()
				cb.RecordResult(j%2 == 0, false)
				cb.State()
				cb.NoProgressCount()
			}
		}()
	}
	for range 10 {
		<-done
	}
}
