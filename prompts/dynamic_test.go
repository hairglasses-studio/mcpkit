//go:build !official_sdk

package prompts

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func promptDef(name string) PromptDefinition {
	return PromptDefinition{
		Prompt: mcp.NewPrompt(name, mcp.WithPromptDescription("Test prompt: "+name)),
		Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{Description: name}, nil
		},
	}
}

func TestDynamicRegistry_RegisterWithServer(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddPrompt(promptDef("test-prompt"))

	s := registry.NewMCPServer("test", "1.0")
	// Should not panic
	d.RegisterWithServer(s)

	if d.PromptCount() != 1 {
		t.Errorf("expected 1 prompt, got %d", d.PromptCount())
	}
}

func TestDynamicRegistry_RegisterWithServer_ChangeFires(t *testing.T) {
	d := NewDynamicRegistry()

	s := registry.NewMCPServer("test", "1.0")
	d.RegisterWithServer(s)

	// Adding after RegisterWithServer triggers change notifier (no panic expected).
	d.AddPrompt(promptDef("new-prompt"))

	if d.PromptCount() != 1 {
		t.Errorf("expected 1 prompt after add, got %d", d.PromptCount())
	}
}

func TestDynamicRegistry_NotifyOnAdd(t *testing.T) {
	d := NewDynamicRegistry()

	var count int32
	d.OnChange(func() {
		atomic.AddInt32(&count, 1)
	})

	d.AddPrompt(promptDef("prompt-a"))
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected 1 notification after add, got %d", atomic.LoadInt32(&count))
	}

	d.AddPrompt(promptDef("prompt-b"))
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("expected 2 notifications after second add, got %d", atomic.LoadInt32(&count))
	}
}

func TestDynamicRegistry_NotifyOnRemove(t *testing.T) {
	d := NewDynamicRegistry()

	var count int32
	d.OnChange(func() {
		atomic.AddInt32(&count, 1)
	})

	d.AddPrompt(promptDef("prompt-a"))
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected 1 notification after add, got %d", atomic.LoadInt32(&count))
	}

	ok := d.RemovePrompt("prompt-a")
	if !ok {
		t.Error("expected RemovePrompt to return true")
	}
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("expected 2 notifications after remove, got %d", atomic.LoadInt32(&count))
	}

	// Removing nonexistent should not notify
	ok = d.RemovePrompt("nonexistent")
	if ok {
		t.Error("expected RemovePrompt to return false for nonexistent")
	}
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("should not notify on no-op remove, got %d", atomic.LoadInt32(&count))
	}
}

func TestDynamicRegistry_MultipleNotifiers(t *testing.T) {
	d := NewDynamicRegistry()

	var c1, c2 int32
	d.OnChange(func() { atomic.AddInt32(&c1, 1) })
	d.OnChange(func() { atomic.AddInt32(&c2, 1) })

	d.AddPrompt(promptDef("x"))

	if atomic.LoadInt32(&c1) != 1 || atomic.LoadInt32(&c2) != 1 {
		t.Errorf("expected both notifiers to fire: c1=%d c2=%d",
			atomic.LoadInt32(&c1), atomic.LoadInt32(&c2))
	}
}
