// Package mcptest provides testing infrastructure for MCP tool handlers.
//
// [NewServer] wraps a [registry.ToolRegistry] in an in-process MCP server
// that requires no HTTP transport. Assertion helpers ([AssertToolResult],
// [AssertNoError], etc.) verify tool outputs within standard Go tests.
// [Recorder] captures invocations for session replay; [ReplaySession] runs a
// saved session against a new server and diffs the results.
// [AssertSnapshot] writes or compares golden files for structured outputs.
// [BenchmarkTool] and [BenchmarkToolParallel] integrate with [testing.B] for
// measuring tool throughput.
//
// Example:
//
//	srv := mcptest.NewServer(t, reg)
//	result := srv.Call(t, "greet", map[string]any{"name": "world"})
//	mcptest.AssertTextContains(t, result, "hello")
package mcptest
