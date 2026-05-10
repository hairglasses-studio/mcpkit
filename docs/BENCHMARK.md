# Benchmarks

mcpkit includes benchmark suites for measuring tool dispatch, middleware overhead, and gateway proxy performance.

## Running Benchmarks

```bash
# All benchmarks
go test -bench=. -benchmem ./...

# Specific package
go test -bench=. -benchmem ./mcptest/

# With count for statistical significance
go test -bench=. -benchmem -count=5 ./... | tee bench.txt

# Compare two runs with benchstat
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

mcpkit also includes a lightweight parser for automation that only needs
per-benchmark percentage deltas:

```go
before, _ := mcptest.ParseBenchmarkOutput(oldReader)
after, _ := mcptest.ParseBenchmarkOutput(newReader)
deltas := mcptest.CompareBenchmarkResults(before, after)
for _, delta := range deltas {
    if delta.Regressed(15) {
        t.Fatalf("%s regressed: %.1f%% ns/op", delta.Name, delta.NsPerOpPct)
    }
}
```

For CI or shell automation, use `tools/benchguard`:

```bash
go test -bench=. -benchmem -run '^$' ./mcptest ./testing/benchmark | tee bench-current.txt
go run ./tools/benchguard \
  -baseline bench-baseline.txt \
  -current bench-current.txt \
  -latency-threshold 15 \
  -memory-threshold 20 \
  -alloc-threshold 10
```

## Benchmark Suites

### mcptest — Tool Benchmarks

```bash
go test -bench=. -benchmem ./mcptest/
```

| Benchmark | Measures |
|-----------|----------|
| `BenchmarkToolDirect` | Raw handler latency (no middleware, no gateway) |
| `BenchmarkMiddlewareOverhead` | Cost of a single middleware layer |
| `BenchmarkGatewayProxy` | Gateway namespace lookup + dispatch overhead |
| `BenchmarkToolSequential` | Sequential tool invocations |
| `BenchmarkToolParallel` | Concurrent tool invocations |

### Gateway Adapter Benchmarks

```bash
go test -bench=. -benchmem ./gateway/adapter/
```

Tests adapter creation, tool discovery, and call proxying for each protocol.

## Writing Benchmarks

Use the helpers in `mcptest/benchmark.go` and `mcptest/benchmark_gateway.go`:

```go
func BenchmarkMyTool(b *testing.B) {
    handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        return &mcp.CallToolResult{
            Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}},
        }, nil
    }
    mcptest.BenchmarkToolDirect(b, handler, map[string]any{"key": "value"})
}
```

## CI Integration

The PR workflow runs benchmark regression checks against the pull request base commit for `./mcptest` and `./testing/benchmark`. It fails when a shared benchmark exceeds the configured guardrail:

- `ns/op`: 15% regression
- `B/op`: 20% regression
- `allocs/op`: 10% regression

Local equivalent:

```bash
make bench BENCH_CURRENT=bench-current.txt
make bench-guard \
  BENCH_BASELINE=bench-baseline.txt \
  BENCH_CURRENT=bench-current.txt \
  BENCH_MIN_OVERLAP=10
```

Use `benchstat` when you need statistical significance across repeated runs. `benchguard` is intentionally stricter and simpler: it compares the parsed percentage deltas from two benchmark output files and exits non-zero when any threshold is exceeded.

## Interpreting Results

```
BenchmarkToolDirect-16    1234567    987.6 ns/op    256 B/op    3 allocs/op
```

- **ns/op**: Nanoseconds per operation — lower is better
- **B/op**: Bytes allocated per operation — lower is better
- **allocs/op**: Heap allocations per operation — lower is better

When comparing with `benchstat`, look for statistically significant changes (p < 0.05). A regression in `allocs/op` often matters more than `ns/op` for GC pressure.
