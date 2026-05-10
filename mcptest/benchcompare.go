package mcptest

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// BenchmarkResult is one parsed row from `go test -bench -benchmem` output.
type BenchmarkResult struct {
	Name        string
	RawName     string
	Iterations  int
	NsPerOp     float64
	BytesPerOp  float64
	AllocsPerOp float64
}

// BenchmarkDelta compares a benchmark result before and after a change.
type BenchmarkDelta struct {
	Name           string
	Before         BenchmarkResult
	After          BenchmarkResult
	NsPerOpPct     float64
	BytesPerOpPct  float64
	AllocsPerOpPct float64
}

// ParseBenchmarkOutput parses standard Go benchmark output into named results.
func ParseBenchmarkOutput(r io.Reader) (map[string]BenchmarkResult, error) {
	results := make(map[string]BenchmarkResult)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		result, ok, err := parseBenchmarkLine(line)
		if err != nil {
			return nil, err
		}
		if ok {
			results[result.Name] = result
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// CompareBenchmarkResults returns deltas for benchmark names present in both maps.
func CompareBenchmarkResults(before, after map[string]BenchmarkResult) []BenchmarkDelta {
	names := make([]string, 0, len(before))
	for name := range before {
		if _, ok := after[name]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	deltas := make([]BenchmarkDelta, 0, len(names))
	for _, name := range names {
		b := before[name]
		a := after[name]
		deltas = append(deltas, BenchmarkDelta{
			Name:           name,
			Before:         b,
			After:          a,
			NsPerOpPct:     percentDelta(b.NsPerOp, a.NsPerOp),
			BytesPerOpPct:  percentDelta(b.BytesPerOp, a.BytesPerOp),
			AllocsPerOpPct: percentDelta(b.AllocsPerOp, a.AllocsPerOp),
		})
	}
	return deltas
}

// Regressed returns true when any tracked metric exceeds thresholdPct.
func (d BenchmarkDelta) Regressed(thresholdPct float64) bool {
	return d.NsPerOpPct > thresholdPct || d.BytesPerOpPct > thresholdPct || d.AllocsPerOpPct > thresholdPct
}

func parseBenchmarkLine(line string) (BenchmarkResult, bool, error) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return BenchmarkResult{}, false, nil
	}
	if fields[3] != "ns/op" {
		return BenchmarkResult{}, false, nil
	}

	iters, err := strconv.Atoi(fields[1])
	if err != nil {
		return BenchmarkResult{}, false, fmt.Errorf("parse benchmark iterations from %q: %w", line, err)
	}
	ns, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return BenchmarkResult{}, false, fmt.Errorf("parse ns/op from %q: %w", line, err)
	}

	result := BenchmarkResult{
		Name:       trimBenchmarkCPU(fields[0]),
		RawName:    fields[0],
		Iterations: iters,
		NsPerOp:    ns,
	}

	for i := 4; i+1 < len(fields); i++ {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			continue
		}
		switch fields[i+1] {
		case "B/op":
			result.BytesPerOp = value
		case "allocs/op":
			result.AllocsPerOp = value
		}
	}

	return result, true, nil
}

func trimBenchmarkCPU(name string) string {
	idx := strings.LastIndex(name, "-")
	if idx == -1 || idx == len(name)-1 {
		return name
	}
	if _, err := strconv.Atoi(name[idx+1:]); err != nil {
		return name
	}
	return name[:idx]
}

func percentDelta(before, after float64) float64 {
	if before == 0 {
		if after == 0 {
			return 0
		}
		return 100
	}
	return ((after - before) / before) * 100
}
