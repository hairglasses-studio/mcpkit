//go:build !official_sdk

package handoff

import (
	"context"
	"encoding/json"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// AgentAsTool wraps a registered agent as a ToolDefinition that can be added to
// a ToolRegistry. When the tool is invoked, it delegates the provided task
// description to the named agent through the HandoffManager.
func AgentAsTool(manager *HandoffManager, agentName string) registry.ToolDefinition {
	desc := "Delegate a task to agent: " + agentName
	// Prefer the agent's own description if available at construction time.
	manager.mu.RLock()
	if a, ok := manager.agents[agentName]; ok && a.Description != "" {
		desc = a.Description
	}
	manager.mu.RUnlock()

	return registry.ToolDefinition{
		Tool: registry.Tool{
			Name:        "delegate_" + agentName,
			Description: desc,
			InputSchema: registry.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "Description of the task to delegate",
					},
					"max_iterations": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum iterations for the agent",
					},
				},
				Required: []string{"task"},
			},
		},
		Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			args := registry.ExtractArguments(req)
			task, _ := args["task"].(string)
			if task == "" {
				return registry.MakeErrorResult("task description is required"), nil
			}

			hreq := HandoffRequest{TaskDescription: task}
			if maxIter, ok := args["max_iterations"].(float64); ok {
				hreq.MaxIterations = int(maxIter)
			}

			result, err := manager.Delegate(ctx, agentName, hreq)
			if err != nil {
				return registry.MakeErrorResult("delegation failed: " + err.Error()), nil
			}

			data, _ := json.Marshal(result)
			return registry.MakeTextResult(string(data)), nil
		},
		Category: "handoff",
		Tags:     []string{"agent", "delegation"},
	}
}
