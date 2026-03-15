package ralph

import (
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
)

const systemPrompt = `You are an autonomous task executor. You receive a task specification and must work through it iteratively.

Each iteration, you MUST respond with a single JSON object (no other text).

You may call one or more tools per iteration using the tool_calls array:
{
  "complete": false,
  "task_id": "task-1",
  "tool_calls": [
    {"name": "tool_a", "arguments": {"param": "value"}},
    {"name": "tool_b", "arguments": {"other": "value"}}
  ],
  "reasoning": "brief explanation of why these steps",
  "mark_done": false
}

For a single tool, the shorthand "tool_name"/"arguments" also works:
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
- Use only the tools listed below
- If a tool fails, try a different approach
- Always include reasoning`

// buildIterationPrompt constructs the user-role prompt for a single iteration.
func buildIterationPrompt(spec Spec, progress Progress, tools []registry.ToolDefinition) string {
	var b strings.Builder

	// Spec header
	fmt.Fprintf(&b, "# Task: %s\n\n%s\n\n", spec.Name, spec.Description)
	fmt.Fprintf(&b, "**Completion criteria:** %s\n\n", spec.Completion)

	// Task checklist with dependency awareness
	b.WriteString("## Tasks\n\n")
	completed := make(map[string]bool)
	for _, id := range progress.CompletedIDs {
		completed[id] = true
	}

	// Classify tasks
	readySet := make(map[string]bool)
	for _, id := range ReadyTasks(spec.Tasks, completed) {
		readySet[id] = true
	}

	// Build dependency label helper
	depLabel := func(task Task) string {
		if len(task.DependsOn) == 0 {
			return ""
		}
		var parts []string
		for _, dep := range task.DependsOn {
			if completed[dep] {
				parts = append(parts, dep+" [done]")
			} else {
				parts = append(parts, dep)
			}
		}
		return " (depends on: " + strings.Join(parts, ", ") + ")"
	}

	// Ready tasks
	hasReady := false
	for _, task := range spec.Tasks {
		if readySet[task.ID] {
			if !hasReady {
				b.WriteString("### Ready (work on these)\n")
				hasReady = true
			}
			fmt.Fprintf(&b, "- [ ] `%s`: %s%s\n", task.ID, task.Description, depLabel(task))
		}
	}

	// Blocked tasks
	hasBlocked := false
	for _, task := range spec.Tasks {
		done := task.Done || completed[task.ID]
		if !done && !readySet[task.ID] {
			if !hasBlocked {
				if hasReady {
					b.WriteString("\n")
				}
				b.WriteString("### Blocked (dependencies not met)\n")
				hasBlocked = true
			}
			// Show which deps are blocking
			var blockers []string
			for _, dep := range task.DependsOn {
				if !completed[dep] {
					blockers = append(blockers, dep)
				}
			}
			fmt.Fprintf(&b, "- [ ] `%s`: %s (blocked by: %s)\n", task.ID, task.Description, strings.Join(blockers, ", "))
		}
	}

	// Completed tasks
	hasCompleted := false
	for _, task := range spec.Tasks {
		done := task.Done || completed[task.ID]
		if done {
			if !hasCompleted {
				if hasReady || hasBlocked {
					b.WriteString("\n")
				}
				b.WriteString("### Completed\n")
				hasCompleted = true
			}
			fmt.Fprintf(&b, "- [x] `%s`: %s\n", task.ID, task.Description)
		}
	}
	b.WriteString("\n")

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
