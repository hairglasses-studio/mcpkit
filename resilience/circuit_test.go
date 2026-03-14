package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testMetrics struct {
	stateChanges []struct{ name, from, to string }
	rejections   int
}

func (m *testMetrics) OnStateChange(name string, from, to CircuitState) {
	m.stateChanges = append(m.stateChanges, struct{ name, from, to string }{name, from.String(), to.String()})
}

func (m *testMetrics) OnRejection(name string) {
	m.rejections++
}

func TestCircuitBreakerClosedToOpen(t *testing.T) {
	m := &testMetrics{}
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreaker("test", cfg, m)

	if cb.State() != CircuitClosed {
		t.Fatalf("initial state = %v, want closed", cb.State())
	}

	errFail := errors.New("fail")
	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func(_ context.Context) error {
			return errFail
		})
	}

	if cb.State() != CircuitOpen {
		t.Fatalf("state after 3 failures = %v, want open", cb.State())
	}
	if len(m.stateChanges) != 1 || m.stateChanges[0].to != "open" {
		t.Errorf("expected state change to open, got %v", m.stateChanges)
	}
}

func TestCircuitBreakerOpenRejects(t *testing.T) {
	m := &testMetrics{}
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreaker("test", cfg, m)

	// Trip the breaker
	cb.Execute(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	err := cb.Execute(context.Background(), func(_ context.Context) error {
		t.Fatal("should not be called when circuit is open")
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if m.rejections != 1 {
		t.Errorf("expected 1 rejection, got %d", m.rejections)
	}
}

func TestCircuitBreakerOpenToHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          10 * time.Millisecond,
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreaker("test", cfg, nil)

	// Trip the breaker
	cb.Execute(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	time.Sleep(15 * time.Millisecond)

	// Should transition to half-open and allow one call
	err := cb.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error in half-open, got %v", err)
	}

	// Should be closed after success
	if cb.State() != CircuitClosed {
		t.Fatalf("state after half-open success = %v, want closed", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreaker("test", cfg, nil)

	cb.Execute(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	if cb.State() != CircuitOpen {
		t.Fatalf("expected open, got %v", cb.State())
	}

	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Fatalf("expected closed after reset, got %v", cb.State())
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	reg := NewCircuitBreakerRegistry(nil)

	cb1 := reg.Get("service_a")
	cb2 := reg.Get("service_a")
	if cb1 != cb2 {
		t.Fatal("Get() returned different breakers for same service")
	}

	cb3 := reg.Get("service_b")
	if cb3 == cb1 {
		t.Fatal("Get() returned same breaker for different services")
	}
}

func TestCircuitBreakerRegistryStatus(t *testing.T) {
	reg := NewCircuitBreakerRegistry(nil)
	reg.Get("svc_a")
	reg.Get("svc_b")

	status := reg.Status()
	if len(status) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(status))
	}
	if status["svc_a"] != "closed" || status["svc_b"] != "closed" {
		t.Errorf("expected all closed, got %v", status)
	}
}

func TestExecuteWithResult(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig(), nil)

	result, err := ExecuteWithResult(cb, context.Background(), func(_ context.Context) (string, error) {
		return "hello", nil
	})
	if err != nil || result != "hello" {
		t.Fatalf("expected (hello, nil), got (%q, %v)", result, err)
	}
}

func TestCircuitBreakerNilMetrics(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig(), nil)

	// Should not panic with nil metrics
	cb.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})
}
