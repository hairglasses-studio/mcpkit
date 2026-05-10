package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPassesWhenWithinThresholds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	baseline := writeBenchFile(t, dir, "baseline.txt", `
BenchmarkFast-16 1000 100.0 ns/op 64 B/op 2 allocs/op
BenchmarkAlloc-16 1000 200.0 ns/op 100 B/op 4 allocs/op
`)
	current := writeBenchFile(t, dir, "current.txt", `
BenchmarkFast-16 1000 105.0 ns/op 70 B/op 2 allocs/op
BenchmarkAlloc-16 1000 190.0 ns/op 110 B/op 4 allocs/op
`)

	var stdout, stderr strings.Builder
	code := run([]string{"-baseline", baseline, "-current", current, "-min-overlap", "2"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "2 benchmarks compared") {
		t.Fatalf("stdout missing comparison summary: %s", stdout.String())
	}
}

func TestRunFailsOnRegression(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	baseline := writeBenchFile(t, dir, "baseline.txt", `
BenchmarkSlow-16 1000 100.0 ns/op 64 B/op 2 allocs/op
`)
	current := writeBenchFile(t, dir, "current.txt", `
BenchmarkSlow-16 1000 125.0 ns/op 64 B/op 2 allocs/op
`)

	var stdout, stderr strings.Builder
	code := run([]string{
		"-baseline", baseline,
		"-current", current,
		"-latency-threshold", "10",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run code = %d, want 1; stdout = %s stderr = %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "BenchmarkSlow") {
		t.Fatalf("stderr missing benchmark name: %s", stderr.String())
	}
}

func TestRunFailsWhenOverlapTooSmall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	baseline := writeBenchFile(t, dir, "baseline.txt", `
BenchmarkOnlyBaseline-16 1000 100.0 ns/op 64 B/op 2 allocs/op
`)
	current := writeBenchFile(t, dir, "current.txt", `
BenchmarkOnlyCurrent-16 1000 100.0 ns/op 64 B/op 2 allocs/op
`)

	var stdout, stderr strings.Builder
	code := run([]string{"-baseline", baseline, "-current", current, "-min-overlap", "1"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run code = %d, want 2; stdout = %s stderr = %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "shared benchmarks") {
		t.Fatalf("stderr missing overlap error: %s", stderr.String())
	}
}

func writeBenchFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write bench file: %v", err)
	}
	return path
}
