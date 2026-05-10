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

CI regression thresholds are not enabled by default yet. Capture benchmark output in CI with `go test -bench=. -benchmem`, then compare with either `benchstat` or `mcptest.CompareBenchmarkResults`.

## Interpreting Results

```
BenchmarkToolDirect-16    1234567    987.6 ns/op    256 B/op    3 allocs/op
```

- **ns/op**: Nanoseconds per operation — lower is better
- **B/op**: Bytes allocated per operation — lower is better
- **allocs/op**: Heap allocations per operation — lower is better

When comparing with `benchstat`, look for statistically significant changes (p < 0.05). A regression in `allocs/op` often matters more than `ns/op` for GC pressure.
