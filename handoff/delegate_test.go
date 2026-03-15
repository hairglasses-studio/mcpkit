//go:build !official_sdk

package handoff

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestAgentAsToolName(t *testing.T) {
	m := NewHandoffManager(Config{
		Delegate: mockDelegate("completed"),
	})
	_ = m.Register(AgentRef{Name: "summarizer", Description: "Summarises long documents"})

	td := AgentAsTool(m, "summarizer")
	if td.Tool.Name != "delegate_summarizer" {
		t.Errorf("expected tool name delegate_summarizer, got %s", td.Tool.Name)
	}
	if td.Tool.Description != "Summarises long documents" {
		t.Errorf("expected agent description, got %s", td.Tool.Description)
	}
}

func TestAgentAsToolNameFallbackDescription(t *testing.T) {
	m := NewHandoffManager(Config{
		Delegate: mockDelegate("completed"),
	})
	// Agent without description — should fall back to default.
	_ = m.Register(AgentRef{Name: "plain"})

	td := AgentAsTool(m, "plain")
	if !strings.Contains(td.Tool.Description, "plain") {
		t.Errorf("expected fallback description to mention agent name, got %s", td.Tool.Description)
	}
}

func TestAgentAsToolSchema(t *testing.T) {
	m := NewHandoffManager(Config{Delegate: mockDelegate("completed")})
	_ = m.Register(AgentRef{Name: "worker"})

	td := AgentAsTool(m, "worker")
	if td.Tool.InputSchema.Type != "object" {
		t.Errorf("expected schema type object, got %s", td.Tool.InputSchema.Type)
	}
	if _, ok := td.Tool.InputSchema.Properties["task"]; !ok {
		t.Error("expected 'task' property in schema")
	}
	if len(td.Tool.InputSchema.Required) == 0 || td.Tool.InputSchema.Required[0] != "task" {
		t.Error("expected 'task' to be required")
	}
}

func TestAgentAsToolHandlerSuccess(t *testing.T) {
	m := NewHandoffManager(Config{
		Delegate: mockDelegate("completed"),
	})
	_ = m.Register(AgentRef{Name: "worker"})

	td := AgentAsTool(m, "worker")

	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"task": "write a poem"}

	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatalf("expected success result, got error")
	}

	// Unmarshal the JSON result.
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content in result")
	}
	var hr HandoffResult
	if err := json.Unmarshal([]byte(text), &hr); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if hr.Status != "completed" {
		t.Errorf("expected status completed, got %s", hr.Status)
	}
}

func TestAgentAsToolHandlerMissingTask(t *testing.T) {
	m := NewHandoffManager(Config{
		Delegate: mockDelegate("completed"),
	})
	_ = m.Register(AgentRef{Name: "worker"})

	td := AgentAsTool(m, "worker")

	// No arguments — task is missing.
	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error (should return error result, not Go error): %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result when task is missing")
	}
}

func TestAgentAsToolHandlerDelegationError(t *testing.T) {
	m := NewHandoffManager(Config{
		Delegate: mockDelegate("completed"),
	})
	// Register manager without the agent we're wrapping — will get ErrAgentNotFound.
	td := AgentAsTool(m, "ghost")

	req := registry.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"task": "impossible task"}

	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result when agent is not found")
	}
}

func TestAgentAsToolCategories(t *testing.T) {
	m := NewHandoffManager(Config{Delegate: mockDelegate("completed")})
	_ = m.Register(AgentRef{Name: "agent1"})

	td := AgentAsTool(m, "agent1")
	if td.Category != "handoff" {
		t.Errorf("expected category handoff, got %s", td.Category)
	}
	found := false
	for _, tag := range td.Tags {
		if tag == "delegation" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'delegation' tag, got %v", td.Tags)
	}
}
