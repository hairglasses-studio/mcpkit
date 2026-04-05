package adapter

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/a2a"
)

// A2AAdapter connects to an A2A agent and exposes its skills as MCP tools.
// It wraps the mcpkit/a2a Client for protocol translation.
type A2AAdapter struct {
	cfg    Config
	client *a2a.Client
	card   *a2a.AgentCard
}

// NewA2AAdapter creates an A2A adapter from config.
func NewA2AAdapter(ctx context.Context, cfg Config) (ProtocolAdapter, error) {
	adapter := &A2AAdapter{cfg: cfg}
	if err := adapter.Connect(ctx); err != nil {
		return nil, err
	}
	return adapter, nil
}

func (a *A2AAdapter) Protocol() Protocol { return ProtocolA2A }

func (a *A2AAdapter) Connect(ctx context.Context) error {
	opts := []a2a.ClientOption{}
	if a.cfg.Auth != nil && a.cfg.Auth.Token != "" {
		opts = append(opts, a2a.WithAuthToken(a.cfg.Auth.Token))
	}
	a.client = a2a.NewClient(a.cfg.URL, opts...)

	card, err := a.client.GetAgentCard(ctx)
	if err != nil {
		return fmt.Errorf("a2a connect: fetch agent card: %w", err)
	}
	a.card = card
	return nil
}

func (a *A2AAdapter) DiscoverTools(ctx context.Context) ([]mcp.Tool, error) {
	if a.card == nil {
		return nil, fmt.Errorf("a2a: not connected (no agent card)")
	}

	tools := make([]mcp.Tool, 0, len(a.card.Skills))
	for _, skill := range a.card.Skills {
		tool := mcp.NewTool(skill.ID,
			mcp.WithDescription(skill.Description),
			mcp.WithString("message", mcp.Required(),
				mcp.Description("Message to send to the A2A agent for this skill")),
		)
		tools = append(tools, tool)
	}
	return tools, nil
}

func (a *A2AAdapter) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	msg := ""
	if m, ok := arguments["message"].(string); ok {
		msg = m
	}
	if msg == "" {
		msg = fmt.Sprintf("Execute skill: %s", toolName)
	}

	task, err := a.client.SendTask(ctx, a2a.TaskSendParams{
		ID: fmt.Sprintf("gw-%s-%d", toolName, hashArgs(arguments)),
		Messages: []a2a.Message{
			{Role: "user", Parts: []a2a.Part{a2a.TextPart(msg)}},
		},
	})
	if err != nil {
		return makeErrorResult(fmt.Sprintf("a2a call failed: %v", err)), nil
	}

	// Extract response text from agent messages
	responseText := fmt.Sprintf("Task %s: state=%s", task.ID, task.State)
	for i := len(task.Messages) - 1; i >= 0; i-- {
		if task.Messages[i].Role == "agent" {
			for _, part := range task.Messages[i].Parts {
				if part.Type == "text" && part.Text != "" {
					responseText = part.Text
					break
				}
			}
			break
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: responseText},
		},
	}, nil
}

func (a *A2AAdapter) Healthy(ctx context.Context) error {
	_, err := a.client.GetAgentCard(ctx)
	return err
}

func (a *A2AAdapter) Close() error {
	return nil // HTTP client doesn't need explicit close
}

func makeErrorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: msg},
		},
		IsError: true,
	}
}

func hashArgs(args map[string]interface{}) uint64 {
	// Simple hash for task ID uniqueness
	var h uint64 = 14695981039346656037
	for k, v := range args {
		for _, c := range k {
			h ^= uint64(c)
			h *= 1099511628211
		}
		for _, c := range fmt.Sprint(v) {
			h ^= uint64(c)
			h *= 1099511628211
		}
	}
	return h
}
