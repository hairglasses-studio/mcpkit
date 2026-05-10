// Command benchguard compares two Go benchmark output files and exits non-zero
// when any shared benchmark exceeds configured regression thresholds.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hairglasses-studio/mcpkit/mcptest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	defaults := mcptest.DefaultBenchmarkThresholds()

	fs := flag.NewFlagSet("benchguard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baselinePath := fs.String("baseline", "", "path to baseline go test -bench output")
	currentPath := fs.String("current", "", "path to current go test -bench output")
	latencyThreshold := fs.Float64("latency-threshold", defaults.NsPerOpPct, "allowed ns/op regression percentage")
	memoryThreshold := fs.Float64("memory-threshold", defaults.BytesPerOpPct, "allowed B/op regression percentage")
	allocThreshold := fs.Float64("alloc-threshold", defaults.AllocsPerOpPct, "allowed allocs/op regression percentage")
	minOverlap := fs.Int("min-overlap", 1, "minimum shared benchmark names required")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *baselinePath == "" || *currentPath == "" {
		fmt.Fprintln(stderr, "benchguard: -baseline and -current are required")
		fs.Usage()
		return 2
	}

	baseline, err := parseBenchmarkFile(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "benchguard: parse baseline: %v\n", err)
		return 2
	}
	current, err := parseBenchmarkFile(*currentPath)
	if err != nil {
		fmt.Fprintf(stderr, "benchguard: parse current: %v\n", err)
		return 2
	}

	deltas := mcptest.CompareBenchmarkResults(baseline, current)
	if len(deltas) < *minOverlap {
		fmt.Fprintf(stderr, "benchguard: only %d shared benchmarks found, require at least %d\n", len(deltas), *minOverlap)
		return 2
	}

	thresholds := mcptest.BenchmarkThresholds{
		NsPerOpPct:     *latencyThreshold,
		BytesPerOpPct:  *memoryThreshold,
		AllocsPerOpPct: *allocThreshold,
	}
	regressions := mcptest.BenchmarkRegressions(deltas, thresholds)
	if len(regressions) > 0 {
		fmt.Fprintf(stderr, "benchguard: %d benchmark regression(s) exceeded thresholds (ns/op %.1f%%, B/op %.1f%%, allocs/op %.1f%%)\n",
			len(regressions), thresholds.NsPerOpPct, thresholds.BytesPerOpPct, thresholds.AllocsPerOpPct)
		for _, delta := range regressions {
			fmt.Fprintf(stderr,
				"- %s: ns/op %.1f%% (%.2f -> %.2f), B/op %.1f%% (%.2f -> %.2f), allocs/op %.1f%% (%.2f -> %.2f)\n",
				delta.Name,
				delta.NsPerOpPct, delta.Before.NsPerOp, delta.After.NsPerOp,
				delta.BytesPerOpPct, delta.Before.BytesPerOp, delta.After.BytesPerOp,
				delta.AllocsPerOpPct, delta.Before.AllocsPerOp, delta.After.AllocsPerOp,
			)
		}
		return 1
	}

	fmt.Fprintf(stdout, "benchguard: %d benchmarks compared, no regressions above thresholds (ns/op %.1f%%, B/op %.1f%%, allocs/op %.1f%%)\n",
		len(deltas), thresholds.NsPerOpPct, thresholds.BytesPerOpPct, thresholds.AllocsPerOpPct)
	return 0
}

func parseBenchmarkFile(path string) (map[string]mcptest.BenchmarkResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return mcptest.ParseBenchmarkOutput(f)
}
