// Package ralph implements the Ralph Loop pattern — an autonomous iterative
// task runner for MCP-based agents. A Config defines the task spec, sampling
// client, tool registry, and optional hooks. The Loop drives repeated
// sampling iterations with DAG-enforced task ordering, YAML spec support,
// per-iteration model selection, and a WorkflowLoop bridge for integration
// with the workflow engine.
package ralph
