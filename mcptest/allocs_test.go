//go:build !official_sdk

package mcptest

import (
	"strings"
	"testing"
)

func TestAssertMaxAllocs_UnderLimit(t *testing.T) {
	// An empty closure allocates nothing.
	AssertMaxAllocs(t, 0, 100, func() {})
}

// allocSink prevents the compiler from optimizing away allocations in tests.
var allocSink []byte

func TestAssertMaxAllocs_OverLimitFails(t *testing.T) {
	// Use a sub-test whose failure is captured, not propagated.
	captured := &captureT{T: t}
	AssertMaxAllocs(captured, 0, 100, func() {
		allocSink = make([]byte, 64) // forces one alloc; package-level var sink
	})
	if !captured.failed {
		t.Error("expected failure when allocs exceed limit, got none")
	}
	if !strings.Contains(captured.logs, "allocs-per-run") {
		t.Errorf("error message missing expected text, got %q", captured.logs)
	}
}

func TestAssertMaxAllocs_InvalidRuns(t *testing.T) {
	captured := &captureT{T: t}
	AssertMaxAllocs(captured, 0, 0, func() {})
	if !captured.failed {
		t.Error("expected Errorf when runs < 1")
	}
}

func TestReportAllocDelta_EmptyFn(t *testing.T) {
	// Warm up — the first call after GC can have one internal alloc from runtime.
	_ = ReportAllocDelta(func() {})
	got := ReportAllocDelta(func() {})
	if got > 2 {
		t.Errorf("empty fn reported %d allocs, want <= 2", got)
	}
}

func TestReportAllocDelta_NonZero(t *testing.T) {
	got := ReportAllocDelta(func() {
		allocSink = make([]byte, 4096)
	})
	if got < 1 {
		t.Errorf("expected at least one alloc for make([]byte, 4096), got %d", got)
	}
}

func TestBenchmarkAllocLimit_UnderLimit(t *testing.T) {
	// Run as a fake benchmark context.
	result := testing.Benchmark(func(b *testing.B) {
		BenchmarkAllocLimit(b, 1, func() {
			_ = make([]byte, 8)
		})
	})
	if result.N == 0 {
		t.Error("benchmark did not run")
	}
}

// captureT is a thin testing.TB harness that records Errorf/Fatalf calls
// without propagating them to the parent test, so we can assert on failure
// behavior.
type captureT struct {
	*testing.T
	failed bool
	fatal  bool
	logs   string
}

func (c *captureT) Helper() {}

func (c *captureT) Errorf(format string, args ...any) {
	c.failed = true
	c.logs += sprintf(format, args...)
}

func (c *captureT) Fatalf(format string, args ...any) {
	c.fatal = true
	c.logs += sprintf(format, args...)
}

func (c *captureT) Fatal(args ...any) {
	c.fatal = true
}

// sprintf avoids the fmt import where we only need simple string building.
func sprintf(format string, args ...any) string {
	// Use a very cheap fallback — fmt.Sprintf is fine but adds the import.
	// The message content we care about is in the format string itself.
	_ = args
	return format
}
