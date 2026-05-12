package mcptest

import (
	"context"
	"crypto/sha256"
	"sort"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/middleware/truncate"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// measureP99 executes fn `iterations` times and returns the 99th percentile duration.
func measureP99(iterations int, fn func()) time.Duration {
	durations := make([]time.Duration, iterations)
	// Warm up
	for i := 0; i < 100; i++ {
		fn()
	}

	for i := 0; i < iterations; i++ {
		start := time.Now()
		fn()
		durations[i] = time.Since(start)
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	p99Index := int(float64(iterations) * 0.99)
	return durations[p99Index]
}

func burnCPU() {
	var data [1024]byte
	// Simulating realistic handler work (~50-100us)
	for i := 0; i < 50; i++ {
		_ = sha256.Sum256(data[:])
	}
}

func TestMiddlewareP99LatencyLimit(t *testing.T) {
	// Goal: Ensure no single middleware layer adds >5% p99 latency overhead vs raw handler.
	baseHandler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		burnCPU()
		// Return a small text result
		return &registry.CallToolResult{
			Content: []registry.Content{
				registry.MakeTextContent("Hello, World!"),
			},
		}, nil
	}

	td := registry.ToolDefinition{}
	ctx := context.Background()
	var req registry.CallToolRequest

	iterations := 2000

	// 1. Measure raw baseline
	baseP99 := measureP99(iterations, func() {
		_, _ = baseHandler(ctx, req)
	})

	// 2. Measure wrapped with a real middleware (truncate)
	mw := truncate.New(truncate.WithMaxBytes(4096))
	wrappedHandler := mw("test_tool", td, baseHandler)

	wrappedP99 := measureP99(iterations, func() {
		_, _ = wrappedHandler(ctx, req)
	})

	maxAllowed := time.Duration(float64(baseP99) * 1.05)

	t.Logf("Base P99:    %v", baseP99)
	t.Logf("Wrapped P99: %v", wrappedP99)
	t.Logf("Threshold:   %v (5%% overhead limit)", maxAllowed)

	if wrappedP99 > maxAllowed {
		delta := wrappedP99 - baseP99
		// Provide an absolute noise floor (e.g. 1500ns) to prevent flaky CI failures
		// when running on shared/noisy runners where CPU scheduling introduces jitter.
		if delta > 1500*time.Nanosecond {
			t.Errorf("Middleware added %v latency (base=%v, wrapped=%v), exceeding 5%% threshold of %v", delta, baseP99, wrappedP99, maxAllowed)
		} else {
			t.Logf("Middleware exceeded 5%% but absolute delta %v is within noise margin (<=1500ns). Passing.", delta)
		}
	}
}
