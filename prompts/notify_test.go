package prompts

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestWirePromptListChanged_Add(t *testing.T) {
	d := NewDynamicRegistry()
	s := registry.NewMCPServer("test", "0.0.0")

	d.AddPrompt(PromptDefinition{
		Prompt:  mcp.Prompt{Name: "initial", Description: "initial"},
		Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) { return nil, nil },
	})

	d.RegisterWithServer(s)

	d.AddPrompt(PromptDefinition{
		Prompt:  mcp.Prompt{Name: "added", Description: "added"},
		Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) { return nil, nil },
	})

	names := d.ListPrompts()
	if len(names) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(names))
	}
}

func TestWirePromptListChanged_Remove(t *testing.T) {
	d := NewDynamicRegistry()
	s := registry.NewMCPServer("test", "0.0.0")

	d.AddPrompt(PromptDefinition{
		Prompt:  mcp.Prompt{Name: "keep", Description: "keep"},
		Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) { return nil, nil },
	})
	d.AddPrompt(PromptDefinition{
		Prompt:  mcp.Prompt{Name: "remove_me", Description: "remove"},
		Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) { return nil, nil },
	})

	d.RegisterWithServer(s)

	ok := d.RemovePrompt("remove_me")
	if !ok {
		t.Fatal("expected RemovePrompt to return true")
	}

	names := d.ListPrompts()
	if len(names) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(names))
	}
	if names[0] != "keep" {
		t.Fatalf("expected 'keep', got %q", names[0])
	}
}

func TestWirePromptListChanged_AddAndRemove(t *testing.T) {
	d := NewDynamicRegistry()
	s := registry.NewMCPServer("test", "0.0.0")

	d.AddPrompt(PromptDefinition{
		Prompt:  mcp.Prompt{Name: "alpha", Description: "alpha"},
		Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) { return nil, nil },
	})
	d.AddPrompt(PromptDefinition{
		Prompt:  mcp.Prompt{Name: "beta", Description: "beta"},
		Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) { return nil, nil },
	})

	d.RegisterWithServer(s)

	d.RemovePrompt("beta")
	d.AddPrompt(PromptDefinition{
		Prompt:  mcp.Prompt{Name: "gamma", Description: "gamma"},
		Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) { return nil, nil },
	})

	names := d.ListPrompts()
	if len(names) != 2 {
		t.Fatalf("expected 2 prompts, got %d: %v", len(names), names)
	}
	expected := map[string]bool{"alpha": true, "gamma": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected prompt %q", name)
		}
	}
}
