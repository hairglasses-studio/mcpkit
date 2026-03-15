package resilience_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/resilience"
)

func ExampleNewCircuitBreaker() {
	cfg := resilience.DefaultCircuitBreakerConfig()
	cb := resilience.NewCircuitBreaker("my-service", cfg, nil)

	fmt.Println(cb.State())

	// Execute a successful call — circuit stays closed.
	_ = cb.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})

	fmt.Println(cb.State())
	// Output:
	// closed
	// closed
}

func ExampleNewRateLimiter() {
	// Allow 100 req/s with burst of 5.
	lim := resilience.NewRateLimiter(100, 5)

	// First five calls should succeed immediately (burst tokens available).
	allowed := 0
	for i := 0; i < 5; i++ {
		if lim.Allow() {
			allowed++
		}
	}

	fmt.Println(allowed)
	// Output:
	// 5
}

func ExampleNewCircuitBreakerRegistry() {
	reg := resilience.NewCircuitBreakerRegistry(nil)

	cb := reg.Get("db")
	fmt.Println(cb.State())

	// Same key returns the same breaker.
	cb2 := reg.Get("db")
	fmt.Println(cb == cb2)
	// Output:
	// closed
	// true
}
