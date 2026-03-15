package skills

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestMiddlewareInjectsLoader(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	sr := NewSkillRegistry(reg)
	loader := NewContextLoader(sr)

	mw := Middleware(loader)

	var capturedLoader *ContextLoader
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		capturedLoader = LoaderFromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	td := registry.ToolDefinition{
		Tool: registry.Tool{Name: "test_tool"},
	}

	wrapped := mw("test_tool", td, handler)
	_, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if capturedLoader == nil {
		t.Fatal("LoaderFromContext returned nil; expected the injected loader")
	}
	if capturedLoader != loader {
		t.Error("LoaderFromContext returned a different loader than injected")
	}
}

func TestLoaderFromContextReturnsNilWhenNotSet(t *testing.T) {
	ctx := context.Background()
	l := LoaderFromContext(ctx)
	if l != nil {
		t.Errorf("LoaderFromContext on bare context should return nil, got %v", l)
	}
}
