//go:build !official_sdk

package ralph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestNewModule_NameDescription(t *testing.T) {
	reg := registry.NewToolRegistry()
	m := NewModule(reg, &mockSampler{})

	if m.Name() != "ralph" {
		t.Errorf("Name() = %q, want %q", m.Name(), "ralph")
	}
	if m.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestNewModule_Tools(t *testing.T) {
	reg := registry.NewToolRegistry()
	m := NewModule(reg, &mockSampler{})

	tools := m.Tools()
	if len(tools) != 3 {
		t.Fatalf("Tools() len = %d, want 3", len(tools))
	}

	names := make(map[string]bool)
	for _, td := range tools {
		names[td.Tool.Name] = true
	}
	for _, want := range []string{"ralph_start", "ralph_stop", "ralph_status"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestModule_Status_NoLoop(t *testing.T) {
	reg := registry.NewToolRegistry()
	m := NewModule(reg, &mockSampler{})

	tools := m.Tools()
	// Find ralph_status tool
	var statusTool *registry.ToolDefinition
	for i := range tools {
		if tools[i].Tool.Name == "ralph_status" {
			statusTool = &tools[i]
			break
		}
	}
	if statusTool == nil {
		t.Fatal("ralph_status tool not found")
	}

	req := registry.CallToolRequest{}
	result, err := statusTool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("ralph_status handler error: %v", err)
	}
	if result == nil {
		t.Fatal("ralph_status handler returned nil result")
	}

	// Extract text and verify idle status
	text := extractResultText(result)
	if text == "" {
		t.Fatal("ralph_status returned empty text")
	}

	var out StatusOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to parse StatusOutput: %v — text was: %s", err, text)
	}
	if out.Status != StatusIdle {
		t.Errorf("Status = %q, want %q", out.Status, StatusIdle)
	}
}

func TestModule_Stop_NoLoop(t *testing.T) {
	reg := registry.NewToolRegistry()
	m := NewModule(reg, &mockSampler{})

	tools := m.Tools()
	var stopTool *registry.ToolDefinition
	for i := range tools {
		if tools[i].Tool.Name == "ralph_stop" {
			stopTool = &tools[i]
			break
		}
	}
	if stopTool == nil {
		t.Fatal("ralph_stop tool not found")
	}

	req := registry.CallToolRequest{}
	result, err := stopTool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("ralph_stop handler error: %v", err)
	}
	if result == nil {
		t.Fatal("ralph_stop handler returned nil result")
	}

	text := extractResultText(result)
	var out StopOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to parse StopOutput: %v — text was: %s", err, text)
	}
	if out.Status != StatusIdle {
		t.Errorf("Stop status = %q, want %q", out.Status, StatusIdle)
	}
	if out.Message != "no loop is running" {
		t.Errorf("Stop message = %q, want %q", out.Message, "no loop is running")
	}
}

// extractResultText pulls the first text content from a CallToolResult.
func extractResultText(result *registry.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	text, _ := registry.ExtractTextContent(result.Content[0])
	return text
}
