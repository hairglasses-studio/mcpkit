package a2a

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// BridgeToolInput is the input for the a2a_send_task MCP tool.
type BridgeToolInput struct {
	AgentURL string `json:"agent_url" jsonschema:"required,description=URL of the A2A agent to send the task to"`
	Message  string `json:"message"   jsonschema:"required,description=Task message to send to the agent"`
	TaskID   string `json:"task_id,omitempty" jsonschema:"description=Optional task ID (auto-generated if empty)"`
	Wait     bool   `json:"wait,omitempty"    jsonschema:"description=Wait for task completion (polls until terminal state)"`
}

// BridgeToolOutput is the output of the a2a_send_task MCP tool.
type BridgeToolOutput struct {
	TaskID    string    `json:"task_id"`
	State     TaskState `json:"state"`
	AgentURL  string    `json:"agent_url"`
	Response  string    `json:"response,omitempty"`
	Artifacts []string  `json:"artifacts,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// NewBridgeTool creates an MCP tool that sends tasks to A2A agents.
// This bridges MCP→A2A: an MCP client can call this tool to delegate
// work to any A2A-compatible agent.
func NewBridgeTool() registry.ToolDefinition {
	return handler.TypedHandler[BridgeToolInput, BridgeToolOutput](
		"a2a_send_task",
		"Send a task to a remote A2A agent. The agent URL must serve an A2A-compatible endpoint. Use wait=true to block until the task completes.",
		func(ctx context.Context, input BridgeToolInput) (BridgeToolOutput, error) {
			client := NewClient(input.AgentURL)

			taskID := input.TaskID
			if taskID == "" {
				taskID = uuid.New().String()
			}

			params := TaskSendParams{
				ID: taskID,
				Messages: []Message{
					{Role: "user", Parts: []Part{TextPart(input.Message)}},
				},
			}

			task, err := client.SendTask(ctx, params)
			if err != nil {
				return BridgeToolOutput{
					TaskID:   taskID,
					State:    TaskFailed,
					AgentURL: input.AgentURL,
					Error:    err.Error(),
				}, nil
			}

			// If not waiting, return immediately
			if !input.Wait {
				return taskToOutput(task, input.AgentURL), nil
			}

			// Poll until terminal state
			for !task.State.IsTerminal() {
				select {
				case <-ctx.Done():
					return BridgeToolOutput{
						TaskID:   taskID,
						State:    TaskCanceled,
						AgentURL: input.AgentURL,
						Error:    "context canceled while waiting",
					}, nil
				case <-time.After(2 * time.Second):
				}

				task, err = client.GetTask(ctx, taskID)
				if err != nil {
					return BridgeToolOutput{
						TaskID:   taskID,
						State:    TaskFailed,
						AgentURL: input.AgentURL,
						Error:    fmt.Sprintf("poll failed: %v", err),
					}, nil
				}
			}

			return taskToOutput(task, input.AgentURL), nil
		},
	)
}

func taskToOutput(task *Task, agentURL string) BridgeToolOutput {
	out := BridgeToolOutput{
		TaskID:   task.ID,
		State:    task.State,
		AgentURL: agentURL,
	}

	// Extract text from last agent message
	for i := len(task.Messages) - 1; i >= 0; i-- {
		if task.Messages[i].Role == "agent" {
			for _, part := range task.Messages[i].Parts {
				if part.Type == "text" && part.Text != "" {
					out.Response = part.Text
					break
				}
			}
			break
		}
	}

	// Extract artifact names
	for _, a := range task.Artifacts {
		out.Artifacts = append(out.Artifacts, a.Name)
	}

	return out
}
