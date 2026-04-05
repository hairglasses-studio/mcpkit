package resilience

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	lim := NewRateLimiter(100, 5)

	for i := range 5 {
		if !lim.Allow() {
			t.Fatalf("Allow() returned false at request %d, expected true", i+1)
		}
	}
	if lim.Allow() {
		t.Fatal("Allow() returned true after burst exhausted, expected false")
	}
}

func TestRateLimiterWait(t *testing.T) {
	lim := NewRateLimiter(1000, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := lim.Wait(ctx); err != nil {
		t.Fatalf("Wait() returned error: %v", err)
	}

	if err := lim.Wait(ctx); err != nil {
		t.Fatalf("Wait() returned error on second call: %v", err)
	}
}

func TestRateLimiterWaitContextCancelled(t *testing.T) {
	lim := NewRateLimiter(0.1, 1)
	lim.tokens = 0

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := lim.Wait(ctx)
	if err == nil {
		t.Fatal("Wait() returned nil, expected context error")
	}
}

func TestRateLimitRegistryGet(t *testing.T) {
	r := NewRateLimitRegistry()

	lim1 := r.Get("service_a")
	lim2 := r.Get("service_a")
	if lim1 != lim2 {
		t.Fatal("Get() returned different limiters for same service")
	}

	lim3 := r.Get("service_b")
	if lim3 == nil {
		t.Fatal("Get() returned nil for new service")
	}
	if lim3 == lim1 {
		t.Fatal("Get() returned same limiter for different services")
	}
}

func TestRateLimitRegistryConfigure(t *testing.T) {
	r := NewRateLimitRegistry()

	r.Configure("custom", 5.0, 10)
	lim := r.Get("custom")
	if lim.rate != 5.0 {
		t.Fatalf("rate = %f, want 5.0", lim.rate)
	}
	if lim.burst != 10 {
		t.Fatalf("burst = %d, want 10", lim.burst)
	}
}

func TestRateLimiterRefill(t *testing.T) {
	lim := NewRateLimiter(1000, 5)

	for range 5 {
		lim.Allow()
	}
	if lim.Allow() {
		t.Fatal("should be exhausted")
	}

	time.Sleep(10 * time.Millisecond)
	if !lim.Allow() {
		t.Fatal("should have refilled after sleep")
	}
}
