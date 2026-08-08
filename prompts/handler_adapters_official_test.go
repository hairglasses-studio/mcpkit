//go:build official_sdk

package prompts

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestTextPromptHandler(t *testing.T) {
	h := TextPromptHandler(func(_ context.Context, args map[string]string) (string, string, error) {
		return "desc for " + args["name"], "hello " + args["name"], nil
	})
	result, err := h(context.Background(), &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{Arguments: map[string]string{"name": "alice"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Description != "desc for alice" {
		t.Errorf("Description = %q", result.Description)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	msg := result.Messages[0]
	if msg.Role != registry.RoleUser {
		t.Errorf("Role = %q, want user", msg.Role)
	}
	tc, ok := msg.Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *TextContent, got %T", msg.Content)
	}
	if tc.Text != "hello alice" {
		t.Errorf("Text = %q", tc.Text)
	}
}

func TestTextPromptHandler_Error(t *testing.T) {
	wantErr := errors.New("boom")
	h := TextPromptHandler(func(_ context.Context, _ map[string]string) (string, string, error) {
		return "", "", wantErr
	})
	_, err := h(context.Background(), &mcp.GetPromptRequest{Params: &mcp.GetPromptParams{}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped wantErr, got %v", err)
	}
}
