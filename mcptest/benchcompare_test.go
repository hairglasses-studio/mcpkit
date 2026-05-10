package mcptest

import (
	"strings"
	"testing"
)

func TestParseBenchmarkOutput(t *testing.T) {
	t.Parallel()

	input := `
goos: linux
goarch: amd64
pkg: github.com/hairglasses-studio/mcpkit/mcptest
BenchmarkTool_Echo-16          1234567        100.0 ns/op        64 B/op        2 allocs/op
BenchmarkSuite/foo/bar-16       500000        250.5 ns/op       128 B/op        4 allocs/op
PASS
`
	results, err := ParseBenchmarkOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseBenchmarkOutput: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}

	echo := results["BenchmarkTool_Echo"]
	if echo.RawName != "BenchmarkTool_Echo-16" {
		t.Errorf("RawName = %q", echo.RawName)
	}
	if echo.Iterations != 1234567 {
		t.Errorf("Iterations = %d", echo.Iterations)
	}
	if echo.NsPerOp != 100 || echo.BytesPerOp != 64 || echo.AllocsPerOp != 2 {
		t.Errorf("echo metrics = %+v", echo)
	}

	suite := results["BenchmarkSuite/foo/bar"]
	if suite.NsPerOp != 250.5 || suite.BytesPerOp != 128 || suite.AllocsPerOp != 4 {
		t.Errorf("suite metrics = %+v", suite)
	}
}

func TestCompareBenchmarkResults(t *testing.T) {
	t.Parallel()

	before := map[string]BenchmarkResult{
		"BenchmarkA": {Name: "BenchmarkA", NsPerOp: 100, BytesPerOp: 1000, AllocsPerOp: 10},
		"BenchmarkB": {Name: "BenchmarkB", NsPerOp: 200, BytesPerOp: 0, AllocsPerOp: 0},
		"OnlyBefore": {Name: "OnlyBefore", NsPerOp: 1},
	}
	after := map[string]BenchmarkResult{
		"BenchmarkA": {Name: "BenchmarkA", NsPerOp: 110, BytesPerOp: 900, AllocsPerOp: 12},
		"BenchmarkB": {Name: "BenchmarkB", NsPerOp: 180, BytesPerOp: 10, AllocsPerOp: 0},
		"OnlyAfter":  {Name: "OnlyAfter", NsPerOp: 1},
	}

	deltas := CompareBenchmarkResults(before, after)
	if len(deltas) != 2 {
		t.Fatalf("deltas len = %d, want 2", len(deltas))
	}
	if deltas[0].Name != "BenchmarkA" {
		t.Fatalf("first delta name = %q", deltas[0].Name)
	}
	if deltas[0].NsPerOpPct != 10 {
		t.Errorf("NsPerOpPct = %v, want 10", deltas[0].NsPerOpPct)
	}
	if deltas[0].BytesPerOpPct != -10 {
		t.Errorf("BytesPerOpPct = %v, want -10", deltas[0].BytesPerOpPct)
	}
	if deltas[0].AllocsPerOpPct != 20 {
		t.Errorf("AllocsPerOpPct = %v, want 20", deltas[0].AllocsPerOpPct)
	}
	if !deltas[0].Regressed(15) {
		t.Error("BenchmarkA should regress at 15% threshold because allocs rose 20%")
	}
	if deltas[1].Name != "BenchmarkB" {
		t.Fatalf("second delta name = %q", deltas[1].Name)
	}
	if deltas[1].BytesPerOpPct != 100 {
		t.Errorf("zero-before BytesPerOpPct = %v, want 100", deltas[1].BytesPerOpPct)
	}
}

func TestParseBenchmarkOutput_IgnoresNonBenchmarkLines(t *testing.T) {
	t.Parallel()

	results, err := ParseBenchmarkOutput(strings.NewReader("PASS\nok pkg 1s\n"))
	if err != nil {
		t.Fatalf("ParseBenchmarkOutput: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results len = %d, want 0", len(results))
	}
}
