package ralph

import (
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
)

const systemPrompt = `You are an autonomous task executor. You receive a task specification and must work through it iteratively.

Each iteration, you MUST respond with a single JSON object (no other text):

{
  "complete": false,
  "task_id": "task-1",
  "tool_name": "tool_to_call",
  "arguments": {"param": "value"},
  "reasoning": "brief explanation of why this step",
  "mark_done": false
}

When a task is finished, set "mark_done": true.
When ALL tasks are done, set "complete": true (no tool call needed).

Rules:
- One tool call per iteration
- Use only the tools listed below
- If a tool fails, try a different approach
- Always include reasoning`

// buildIterationPrompt constructs the user-role prompt for a single iteration.
func buildIterationPrompt(spec Spec, progress Progress, tools []registry.ToolDefinition) string {
	var b strings.Builder

	// Spec header
	fmt.Fprintf(&b, "# Task: %s\n\n%s\n\n", spec.Name, spec.Description)
	fmt.Fprintf(&b, "**Completion criteria:** %s\n\n", spec.Completion)

	// Task checklist
	b.WriteString("## Tasks\n\n")
	completed := make(map[string]bool)
	for _, id := range progress.CompletedIDs {
		completed[id] = true
	}
	for _, task := range spec.Tasks {
		done := task.Done || completed[task.ID]
		check := " "
		if done {
			check = "x"
		}
		fmt.Fprintf(&b, "- [%s] `%s`: %s\n", check, task.ID, task.Description)
	}

	// Recent log (last 5)
	if len(progress.Log) > 0 {
		b.WriteString("\n## Recent Activity\n\n")
		start := len(progress.Log) - 5
		if start < 0 {
			start = 0
		}
		for _, entry := range progress.Log[start:] {
			fmt.Fprintf(&b, "- Iteration %d", entry.Iteration)
			if entry.TaskID != "" {
				fmt.Fprintf(&b, " [%s]", entry.TaskID)
			}
			if len(entry.ToolCalls) > 0 {
				fmt.Fprintf(&b, " called %s", strings.Join(entry.ToolCalls, ", "))
			}
			fmt.Fprintf(&b, ": %s\n", entry.Result)
		}
	}

	// Available tools
	b.WriteString("\n## Available Tools\n\n")
	for _, td := range tools {
		fmt.Fprintf(&b, "### %s\n%s\n\n", td.Tool.Name, td.Tool.Description)
	}

	b.WriteString("\nRespond with a JSON decision.")
	return b.String()
}
