package registry

import (
	"context"
	"testing"
)

func TestWireToolListChanged_Add(t *testing.T) {
	d := NewDynamicRegistry()
	s := NewMCPServer("test", "0.0.0")

	// Register initial tool
	d.AddTool(ToolDefinition{
		Tool:    Tool{Name: "initial", Description: "initial tool", InputSchema: ToolInputSchema{Type: "object"}},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
	})

	d.RegisterWithServer(s)

	// Add a new tool — should trigger WireToolListChanged
	d.AddTool(ToolDefinition{
		Tool:    Tool{Name: "added", Description: "added tool", InputSchema: ToolInputSchema{Type: "object"}},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
	})

	tools := d.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestWireToolListChanged_Remove(t *testing.T) {
	d := NewDynamicRegistry()
	s := NewMCPServer("test", "0.0.0")

	d.AddTool(ToolDefinition{
		Tool:    Tool{Name: "keep", Description: "keep", InputSchema: ToolInputSchema{Type: "object"}},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
	})
	d.AddTool(ToolDefinition{
		Tool:    Tool{Name: "remove_me", Description: "remove", InputSchema: ToolInputSchema{Type: "object"}},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
	})

	d.RegisterWithServer(s)

	ok := d.RemoveTool("remove_me")
	if !ok {
		t.Fatal("expected RemoveTool to return true")
	}

	tools := d.ListTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0] != "keep" {
		t.Fatalf("expected 'keep', got %q", tools[0])
	}
}

func TestWireToolListChanged_AddAndRemove(t *testing.T) {
	d := NewDynamicRegistry()
	s := NewMCPServer("test", "0.0.0")

	d.AddTool(ToolDefinition{
		Tool:    Tool{Name: "alpha", Description: "alpha", InputSchema: ToolInputSchema{Type: "object"}},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
	})
	d.AddTool(ToolDefinition{
		Tool:    Tool{Name: "beta", Description: "beta", InputSchema: ToolInputSchema{Type: "object"}},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
	})

	d.RegisterWithServer(s)

	// Remove one, add another
	d.RemoveTool("beta")
	d.AddTool(ToolDefinition{
		Tool:    Tool{Name: "gamma", Description: "gamma", InputSchema: ToolInputSchema{Type: "object"}},
		Handler: func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
	})

	tools := d.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(tools), tools)
	}

	expected := map[string]bool{"alpha": true, "gamma": true}
	for _, name := range tools {
		if !expected[name] {
			t.Errorf("unexpected tool %q", name)
		}
	}
}
