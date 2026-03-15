//go:build !official_sdk

package registry

import (
	"context"
	"sync"
	"testing"
)

func makeDynamicTool(name, category string) ToolDefinition {
	return ToolDefinition{
		Tool:     Tool{Name: name, Description: "test tool " + name},
		Handler:  func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
		Category: category,
	}
}

func TestDynamicRegistry_AddTool(t *testing.T) {
	d := NewDynamicRegistry()

	d.AddTool(makeDynamicTool("my_tool", "cat"))

	td, ok := d.GetTool("my_tool")
	if !ok {
		t.Fatal("expected my_tool to be registered after AddTool")
	}
	if td.Category != "cat" {
		t.Errorf("category = %q, want cat", td.Category)
	}
}

func TestDynamicRegistry_AddTool_InfersIsWrite(t *testing.T) {
	d := NewDynamicRegistry()

	d.AddTool(makeDynamicTool("resource_create", "cat"))
	d.AddTool(makeDynamicTool("resource_list", "cat"))

	td, _ := d.GetTool("resource_create")
	if !td.IsWrite {
		t.Error("resource_create should be inferred as a write tool")
	}

	td, _ = d.GetTool("resource_list")
	if td.IsWrite {
		t.Error("resource_list should not be inferred as a write tool")
	}
}

func TestDynamicRegistry_RemoveTool(t *testing.T) {
	d := NewDynamicRegistry()

	d.AddTool(makeDynamicTool("my_tool", "cat"))
	existed := d.RemoveTool("my_tool")

	if !existed {
		t.Error("RemoveTool should return true when tool existed")
	}

	_, ok := d.GetTool("my_tool")
	if ok {
		t.Error("my_tool should not exist after RemoveTool")
	}
}

func TestDynamicRegistry_RemoveTool_NotFound(t *testing.T) {
	d := NewDynamicRegistry()

	existed := d.RemoveTool("nonexistent")
	if existed {
		t.Error("RemoveTool should return false for nonexistent tool")
	}
}

func TestDynamicRegistry_OnChange_FiredOnAdd(t *testing.T) {
	d := NewDynamicRegistry()

	called := 0
	d.OnChange(func() { called++ })

	d.AddTool(makeDynamicTool("tool_a", "cat"))
	if called != 1 {
		t.Errorf("OnChange should fire once on AddTool, fired %d times", called)
	}

	d.AddTool(makeDynamicTool("tool_b", "cat"))
	if called != 2 {
		t.Errorf("OnChange should fire twice after two AddTool calls, fired %d times", called)
	}
}

func TestDynamicRegistry_OnChange_FiredOnRemove(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTool(makeDynamicTool("my_tool", "cat"))

	called := 0
	d.OnChange(func() { called++ })

	d.RemoveTool("my_tool")
	if called != 1 {
		t.Errorf("OnChange should fire once on RemoveTool, fired %d times", called)
	}
}

func TestDynamicRegistry_OnChange_NotFiredWhenRemoveMisses(t *testing.T) {
	d := NewDynamicRegistry()

	called := 0
	d.OnChange(func() { called++ })

	d.RemoveTool("nonexistent")
	if called != 0 {
		t.Errorf("OnChange should not fire when RemoveTool misses, fired %d times", called)
	}
}

func TestDynamicRegistry_OnChange_MultipleNotifiers(t *testing.T) {
	d := NewDynamicRegistry()

	var a, b int
	d.OnChange(func() { a++ })
	d.OnChange(func() { b++ })

	d.AddTool(makeDynamicTool("tool_a", "cat"))

	if a != 1 {
		t.Errorf("first notifier: got %d calls, want 1", a)
	}
	if b != 1 {
		t.Errorf("second notifier: got %d calls, want 1", b)
	}
}

func TestDynamicRegistry_FilteredTools_ByCategory(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTool(makeDynamicTool("catA_tool1", "catA"))
	d.AddTool(makeDynamicTool("catA_tool2", "catA"))
	d.AddTool(makeDynamicTool("catB_tool1", "catB"))

	results := d.FilteredTools(ByCategory("catA"))
	if len(results) != 2 {
		t.Fatalf("expected 2 catA tools, got %d", len(results))
	}
	for _, td := range results {
		if td.Category != "catA" {
			t.Errorf("expected catA, got %q", td.Category)
		}
	}
}

func TestDynamicRegistry_FilteredTools_ByRuntimeGroup(t *testing.T) {
	d := NewDynamicRegistry()

	d.AddTool(ToolDefinition{
		Tool:         Tool{Name: "tool_a", Description: "desc"},
		Handler:      func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
		Category:     "cat",
		RuntimeGroup: "groupX",
	})
	d.AddTool(ToolDefinition{
		Tool:         Tool{Name: "tool_b", Description: "desc"},
		Handler:      func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
		Category:     "cat",
		RuntimeGroup: "groupY",
	})

	results := d.FilteredTools(ByRuntimeGroup("groupX"))
	if len(results) != 1 {
		t.Fatalf("expected 1 tool in groupX, got %d", len(results))
	}
	if results[0].Tool.Name != "tool_a" {
		t.Errorf("expected tool_a, got %q", results[0].Tool.Name)
	}
}

func TestDynamicRegistry_FilteredTools_ReadOnly(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTool(makeDynamicTool("resource_list", "cat"))
	d.AddTool(makeDynamicTool("resource_create", "cat"))
	d.AddTool(makeDynamicTool("resource_delete", "cat"))

	results := d.FilteredTools(ReadOnly())
	for _, td := range results {
		if td.IsWrite {
			t.Errorf("ReadOnly filter returned write tool: %q", td.Tool.Name)
		}
	}
	// resource_list should be in there
	found := false
	for _, td := range results {
		if td.Tool.Name == "resource_list" {
			found = true
		}
	}
	if !found {
		t.Error("expected resource_list in ReadOnly results")
	}
}

func TestDynamicRegistry_FilteredTools_Not(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTool(makeDynamicTool("catA_tool", "catA"))
	d.AddTool(makeDynamicTool("catB_tool", "catB"))

	results := d.FilteredTools(Not(ByCategory("catA")))
	for _, td := range results {
		if td.Category == "catA" {
			t.Errorf("Not(ByCategory(catA)) should exclude catA tools, got %q", td.Tool.Name)
		}
	}
}

func TestDynamicRegistry_FilteredTools_And(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTool(makeDynamicTool("catA_list", "catA"))   // catA, read-only
	d.AddTool(makeDynamicTool("catA_create", "catA")) // catA, write
	d.AddTool(makeDynamicTool("catB_list", "catB"))   // catB, read-only

	results := d.FilteredTools(And(ByCategory("catA"), ReadOnly()))
	if len(results) != 1 {
		t.Fatalf("And(ByCategory(catA), ReadOnly()) should return 1 tool, got %d", len(results))
	}
	if results[0].Tool.Name != "catA_list" {
		t.Errorf("expected catA_list, got %q", results[0].Tool.Name)
	}
}

func TestDynamicRegistry_FilteredTools_Exclude(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTool(makeDynamicTool("tool_a", "cat"))
	d.AddTool(makeDynamicTool("tool_b", "cat"))
	d.AddTool(makeDynamicTool("tool_c", "cat"))

	results := d.FilteredTools(Exclude("tool_a", "tool_c"))
	if len(results) != 1 {
		t.Fatalf("Exclude should remove 2 tools, leaving 1; got %d", len(results))
	}
	if results[0].Tool.Name != "tool_b" {
		t.Errorf("expected tool_b to remain, got %q", results[0].Tool.Name)
	}
}

func TestDynamicRegistry_FilteredTools_Sorted(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTool(makeDynamicTool("zzz_tool", "cat"))
	d.AddTool(makeDynamicTool("aaa_tool", "cat"))
	d.AddTool(makeDynamicTool("mmm_tool", "cat"))

	results := d.FilteredTools(func(_ ToolDefinition) bool { return true })
	if len(results) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(results))
	}
	if results[0].Tool.Name != "aaa_tool" {
		t.Errorf("expected aaa_tool first, got %q", results[0].Tool.Name)
	}
	if results[1].Tool.Name != "mmm_tool" {
		t.Errorf("expected mmm_tool second, got %q", results[1].Tool.Name)
	}
	if results[2].Tool.Name != "zzz_tool" {
		t.Errorf("expected zzz_tool third, got %q", results[2].Tool.Name)
	}
}

func TestDynamicRegistry_RegisterModule(t *testing.T) {
	d := NewDynamicRegistry()

	notified := false
	d.OnChange(func() { notified = true })

	d.RegisterModule(&testModule{
		name: "mymod",
		tools: []ToolDefinition{
			makeDynamicTool("mod_tool_list", "cat"),
		},
	})

	if !notified {
		t.Error("OnChange should fire after RegisterModule")
	}

	_, ok := d.GetTool("mod_tool_list")
	if !ok {
		t.Error("expected mod_tool_list to be registered after RegisterModule")
	}
}

func TestDynamicRegistry_ConcurrentAddRemove(t *testing.T) {
	d := NewDynamicRegistry()

	const goroutines = 10
	const toolsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < toolsPerGoroutine; j++ {
				name := makeUniqueName(idx, j)
				d.AddTool(makeDynamicTool(name, "cat"))
				d.RemoveTool(name)
			}
		}(i)
	}

	wg.Wait()
	// No panics or data races means success — tool count may vary
	// but calling ToolCount should not panic
	_ = d.ToolCount()
}

// makeUniqueName returns a deterministic tool name for concurrent tests.
func makeUniqueName(goroutine, index int) string {
	// Use a simple scheme to avoid collisions between goroutines
	// but keep it deterministic. Format: tool_<goroutine>_<index>
	const base = "tool_"
	buf := make([]byte, 0, 16)
	buf = append(buf, base...)
	buf = appendInt(buf, goroutine)
	buf = append(buf, '_')
	buf = appendInt(buf, index)
	return string(buf)
}

func appendInt(buf []byte, n int) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	var tmp [10]byte
	pos := len(tmp)
	for n > 0 {
		pos--
		tmp[pos] = byte('0' + n%10)
		n /= 10
	}
	return append(buf, tmp[pos:]...)
}

func TestDynamicRegistry_RuntimeGroupMapper(t *testing.T) {
	d := NewDynamicRegistry(Config{
		RuntimeGroupMapper: func(category string) string {
			if category == "payments" {
				return "financial"
			}
			return ""
		},
	})

	d.AddTool(ToolDefinition{
		Tool:     Tool{Name: "pay_list", Description: "list payments"},
		Handler:  func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return MakeTextResult("ok"), nil },
		Category: "payments",
	})

	td, ok := d.GetTool("pay_list")
	if !ok {
		t.Fatal("expected pay_list to be registered")
	}
	if td.RuntimeGroup != "financial" {
		t.Errorf("RuntimeGroup = %q, want financial", td.RuntimeGroup)
	}
}

func TestNotDeferred_Filter(t *testing.T) {
	deferred := map[string]bool{
		"tool_lazy": true,
	}

	filter := NotDeferred(deferred)

	eager := ToolDefinition{Tool: Tool{Name: "tool_eager"}}
	lazy := ToolDefinition{Tool: Tool{Name: "tool_lazy"}}

	if !filter(eager) {
		t.Error("NotDeferred should pass through non-deferred tools")
	}
	if filter(lazy) {
		t.Error("NotDeferred should block deferred tools")
	}
}
