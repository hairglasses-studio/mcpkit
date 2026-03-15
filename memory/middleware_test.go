package memory

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestMiddleware_InjectsStore(t *testing.T) {
	t.Parallel()
	store := NewInMemoryStore()
	mw := Middleware(store)
	td := registry.ToolDefinition{}

	var capturedStore Store
	handler := mw("tool", td, func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		capturedStore = FromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	})

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedStore == nil {
		t.Fatal("expected store to be injected into context, got nil")
	}
	if capturedStore != store {
		t.Error("injected store does not match the one passed to Middleware")
	}
}

func TestFromContext_Nil(t *testing.T) {
	t.Parallel()
	got := FromContext(context.Background())
	if got != nil {
		t.Errorf("expected nil from empty context, got %v", got)
	}
}

func TestMiddleware_NextIsCalledWithInjectedStore(t *testing.T) {
	t.Parallel()
	store := NewInMemoryStore()

	// Pre-populate the store so the handler can verify retrieval
	ctx := context.Background()
	_ = store.Set(ctx, MemoryEntry{Key: "hello", Value: "world", Tier: TierSemantic})

	mw := Middleware(store)
	td := registry.ToolDefinition{}

	handler := mw("tool", td, func(handlerCtx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		s := FromContext(handlerCtx)
		if s == nil {
			t.Error("store not found in handler context")
			return registry.MakeTextResult("no store"), nil
		}
		entry, ok, err := s.Get(handlerCtx, "hello")
		if err != nil || !ok || entry.Value != "world" {
			t.Errorf("unexpected Get result: entry=%v ok=%v err=%v", entry, ok, err)
		}
		return registry.MakeTextResult("ok"), nil
	})

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
