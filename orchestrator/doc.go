// Package orchestrator provides multi-agent execution patterns for composing
// MCP tool calls. It includes fan-out (parallel broadcast), pipeline
// (sequential chaining), and select (first-success) patterns. A
// StageMiddleware interface and WrapStage/WrapStages helpers allow
// cross-cutting concerns to be applied uniformly across all stages.
package orchestrator
