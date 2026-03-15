//go:build !official_sdk

package registry

import (
	"context"
	"sort"
	"testing"
)

func makeDeferredTool(name, category string) ToolDefinition {
	return ToolDefinition{
		Tool:     Tool{Name: name, Description: "deferred test tool " + name},
		Handler:  func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
		Category: category,
	}
}

func TestRegisterDeferredModule_AllToolsStored(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeDeferredTool("tool_eager", "cat"),
			makeDeferredTool("tool_lazy", "cat"),
		},
	}

	r.RegisterDeferredModule(mod, map[string]bool{"tool_lazy": true})

	if r.ToolCount() != 2 {
		t.Fatalf("expected 2 tools stored, got %d", r.ToolCount())
	}

	_, okEager := r.GetTool("tool_eager")
	if !okEager {
		t.Error("expected tool_eager to be stored")
	}

	_, okLazy := r.GetTool("tool_lazy")
	if !okLazy {
		t.Error("expected tool_lazy to be stored even though it is deferred")
	}
}

func TestListEagerTools(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeDeferredTool("tool_eager_a", "cat"),
			makeDeferredTool("tool_eager_b", "cat"),
			makeDeferredTool("tool_lazy", "cat"),
		},
	}

	r.RegisterDeferredModule(mod, map[string]bool{"tool_lazy": true})

	eager := r.ListEagerTools()
	sort.Strings(eager)

	if len(eager) != 2 {
		t.Fatalf("expected 2 eager tools, got %d: %v", len(eager), eager)
	}
	if eager[0] != "tool_eager_a" {
		t.Errorf("eager[0] = %q, want tool_eager_a", eager[0])
	}
	if eager[1] != "tool_eager_b" {
		t.Errorf("eager[1] = %q, want tool_eager_b", eager[1])
	}
}

func TestListDeferredTools(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeDeferredTool("tool_eager", "cat"),
			makeDeferredTool("tool_lazy_a", "cat"),
			makeDeferredTool("tool_lazy_b", "cat"),
		},
	}

	r.RegisterDeferredModule(mod, map[string]bool{
		"tool_lazy_a": true,
		"tool_lazy_b": true,
	})

	deferred := r.ListDeferredTools()
	sort.Strings(deferred)

	if len(deferred) != 2 {
		t.Fatalf("expected 2 deferred tools, got %d: %v", len(deferred), deferred)
	}
	if deferred[0] != "tool_lazy_a" {
		t.Errorf("deferred[0] = %q, want tool_lazy_a", deferred[0])
	}
	if deferred[1] != "tool_lazy_b" {
		t.Errorf("deferred[1] = %q, want tool_lazy_b", deferred[1])
	}
}

func TestListEagerTools_NoDeferredTools(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeDeferredTool("tool_a", "cat"),
			makeDeferredTool("tool_b", "cat"),
		},
	}

	// No tools marked as deferred
	r.RegisterDeferredModule(mod, map[string]bool{})

	eager := r.ListEagerTools()
	if len(eager) != 2 {
		t.Fatalf("all tools should be eager when none deferred, got %d", len(eager))
	}
}

func TestListDeferredTools_NoneDeferred(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeDeferredTool("tool_a", "cat"),
		},
	}

	r.RegisterDeferredModule(mod, map[string]bool{})

	deferred := r.ListDeferredTools()
	if len(deferred) != 0 {
		t.Fatalf("expected 0 deferred tools, got %d: %v", len(deferred), deferred)
	}
}

func TestSetDeferred_MarksExistingTool(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name:  "mod",
		tools: []ToolDefinition{makeDeferredTool("my_tool", "cat")},
	})

	if r.IsDeferred("my_tool") {
		t.Error("my_tool should not be deferred initially")
	}

	r.SetDeferred("my_tool", true)

	if !r.IsDeferred("my_tool") {
		t.Error("my_tool should be deferred after SetDeferred(true)")
	}
}

func TestSetDeferred_UnmarksExistingTool(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name:  "mod",
		tools: []ToolDefinition{makeDeferredTool("my_tool", "cat")},
	}
	r.RegisterDeferredModule(mod, map[string]bool{"my_tool": true})

	if !r.IsDeferred("my_tool") {
		t.Fatal("my_tool should start deferred")
	}

	r.SetDeferred("my_tool", false)

	if r.IsDeferred("my_tool") {
		t.Error("my_tool should not be deferred after SetDeferred(false)")
	}
}

func TestSetDeferred_NonexistentToolIsNoop(t *testing.T) {
	r := NewToolRegistry()

	// Should not panic
	r.SetDeferred("nonexistent", true)

	if r.IsDeferred("nonexistent") {
		t.Error("nonexistent tool should not be considered deferred")
	}
}

func TestIsDeferred_ReturnsFalseForUnknownTool(t *testing.T) {
	r := NewToolRegistry()

	if r.IsDeferred("unknown") {
		t.Error("IsDeferred should return false for unknown tool")
	}
}

func TestRegisterDeferredModule_InfersIsWrite(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeDeferredTool("item_create", "cat"),
			makeDeferredTool("item_list", "cat"),
		},
	}
	r.RegisterDeferredModule(mod, map[string]bool{"item_create": true})

	tdCreate, _ := r.GetTool("item_create")
	if !tdCreate.IsWrite {
		t.Error("item_create should be inferred as a write tool")
	}

	tdList, _ := r.GetTool("item_list")
	if tdList.IsWrite {
		t.Error("item_list should not be inferred as a write tool")
	}
}

func TestRegisterDeferredModule_RuntimeGroupMapper(t *testing.T) {
	r := NewToolRegistry(Config{
		RuntimeGroupMapper: func(category string) string {
			if category == "storage" {
				return "data-tier"
			}
			return ""
		},
	})

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			{
				Tool:     Tool{Name: "storage_list", Description: "list storage"},
				Handler:  func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
				Category: "storage",
			},
		},
	}
	r.RegisterDeferredModule(mod, map[string]bool{"storage_list": true})

	td, ok := r.GetTool("storage_list")
	if !ok {
		t.Fatal("expected storage_list to be registered")
	}
	if td.RuntimeGroup != "data-tier" {
		t.Errorf("RuntimeGroup = %q, want data-tier", td.RuntimeGroup)
	}
}

func TestDeferredTools_NotInListTools_WhenUsingFilteredTools(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "mod",
		tools: []ToolDefinition{
			makeDeferredTool("tool_eager", "cat"),
			makeDeferredTool("tool_lazy", "cat"),
		},
	}
	r.RegisterDeferredModule(mod, map[string]bool{"tool_lazy": true})

	// FilteredTools with NotDeferred should exclude deferred tools
	results := r.FilteredTools(NotDeferred(map[string]bool{"tool_lazy": true}))
	for _, td := range results {
		if td.Tool.Name == "tool_lazy" {
			t.Error("tool_lazy should be excluded by NotDeferred filter")
		}
	}
}

func TestSetDeferred_ToggleMultipleTimes(t *testing.T) {
	r := NewToolRegistry()
	r.RegisterModule(&testModule{
		name:  "mod",
		tools: []ToolDefinition{makeDeferredTool("my_tool", "cat")},
	})

	r.SetDeferred("my_tool", true)
	if !r.IsDeferred("my_tool") {
		t.Error("expected deferred after first SetDeferred(true)")
	}

	r.SetDeferred("my_tool", false)
	if r.IsDeferred("my_tool") {
		t.Error("expected not deferred after SetDeferred(false)")
	}

	r.SetDeferred("my_tool", true)
	if !r.IsDeferred("my_tool") {
		t.Error("expected deferred after second SetDeferred(true)")
	}
}
