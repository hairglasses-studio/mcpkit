//go:build !official_sdk

package mcptest

import (
	"runtime"
	"testing"
)

// AssertMaxAllocs runs fn `runs` times and fails t if the average allocations
// per run exceed maxAllocs. It is a thin wrapper around testing.AllocsPerRun
// that produces a readable failure message pointing at the caller.
//
// Use this in regular tests (not benchmarks) to prevent allocation regressions
// in hot paths. Typical allocation budgets for mcpkit hot paths:
//
//   - handler.TextResult:      2-3 allocs
//   - handler.GetStringParam:  0-1 allocs
//   - registry.GetTool:        0 allocs
//   - registry.SearchTools:   proportional to matching tools, not baseline
//
// Example:
//
//	func TestTextResult_Allocs(t *testing.T) {
//	    mcptest.AssertMaxAllocs(t, 3, 1000, func() {
//	        _ = handler.TextResult("hello")
//	    })
//	}
//
// `runs` should be high enough (>=100) to amortize the GC noise floor. The
// reported value is a float because testing.AllocsPerRun averages across runs.
//
// The tb parameter accepts *testing.T or *testing.B so the helper can be used
// from regular tests, benchmarks, or custom harnesses.
func AssertMaxAllocs(tb testing.TB, maxAllocs float64, runs int, fn func()) {
	tb.Helper()
	if runs < 1 {
		tb.Errorf("AssertMaxAllocs: runs must be >= 1, got %d", runs)
		return
	}
	got := testing.AllocsPerRun(runs, fn)
	if got > maxAllocs {
		tb.Errorf("allocs-per-run = %.2f, want <= %.2f (%d runs)", got, maxAllocs, runs)
	}
}

// ReportAllocDelta measures the number of heap allocations made during fn and
// returns them. It can be called multiple times to build up an allocation
// profile for a multi-step operation. Unlike testing.AllocsPerRun, it measures
// a single invocation and does not average.
//
// Callers are responsible for running fn in a warm state if that matters
// (e.g., calling fn() once before measuring).
func ReportAllocDelta(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.Mallocs - before.Mallocs
}

// BenchmarkAllocLimit runs a benchmark for fn with b.ReportAllocs and then
// verifies that the mean allocs-per-op is within maxAllocs. It is intended
// for CI regression gates — pair it with go test -bench and a threshold.
//
// Unlike AssertMaxAllocs, this runs inside a benchmark, so b.N is set by the
// caller (typically the go test runner).
//
// Example:
//
//	func BenchmarkTextResult_WithLimit(b *testing.B) {
//	    mcptest.BenchmarkAllocLimit(b, 3, func() {
//	        _ = handler.TextResult("hello")
//	    })
//	}
func BenchmarkAllocLimit(b *testing.B, maxAllocs float64, fn func()) {
	b.Helper()
	b.ReportAllocs()

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn()
	}
	b.StopTimer()

	runtime.ReadMemStats(&after)
	if b.N == 0 {
		return
	}
	got := float64(after.Mallocs-before.Mallocs) / float64(b.N)
	if got > maxAllocs {
		b.Errorf("allocs/op = %.2f, want <= %.2f (N=%d)", got, maxAllocs, b.N)
	}
}
