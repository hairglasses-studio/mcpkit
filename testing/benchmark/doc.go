// Package benchmark provides cross-protocol performance benchmarks for mcpkit.
//
// This package measures translation latency, throughput, and memory allocation
// across all protocol paths: MCP, A2A, and OpenAI function calling.
//
// # Performance Targets
//
// These targets represent the upper bounds for acceptable performance on
// modern hardware. The bridge must add negligible overhead to tool invocations.
//
//   - Translation: <500us per operation (ToolToSkill, CallResultToArtifact, etc.)
//   - Card generation: <5ms for 100 tools
//   - Gateway round-trip: <10ms (in-process, single tool call)
//   - Throughput: >10K req/s (100 concurrent callers, single bridge)
//   - Memory per translation: <4KB allocation
//
// # Running
//
//	go test ./testing/benchmark/... -bench=. -benchmem
//
// Each benchmark reports ns/op, B/op, and allocs/op via testing.B.ReportAllocs().
//
// # Regression Detection
//
// Capture a baseline with:
//
//	go test ./testing/benchmark/... -bench=. -benchmem -count=6 > baseline.txt
//
// Compare after changes using benchstat:
//
//	go test ./testing/benchmark/... -bench=. -benchmem -count=6 > new.txt
//	benchstat baseline.txt new.txt
package benchmark
