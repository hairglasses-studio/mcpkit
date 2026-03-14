package registry

import (
	"context"
	"errors"
	"testing"
)

// testModule implements ToolModule for testing.
type testModule struct {
	name  string
	tools []ToolDefinition
}

func (m *testModule) Name() string            { return m.name }
func (m *testModule) Description() string      { return "test module" }
func (m *testModule) Tools() []ToolDefinition { return m.tools }

func newTestTool(name, category string, handler ToolHandlerFunc) ToolDefinition {
	if handler == nil {
		handler = func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
			return MakeTextResult("ok"), nil
		}
	}
	return ToolDefinition{
		Tool:     Tool{Name: name, Description: "test tool " + name},
		Handler:  handler,
		Category: category,
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewToolRegistry()

	mod := &testModule{
		name: "testmod",
		tools: []ToolDefinition{
			newTestTool("test_list", "test", nil),
			newTestTool("test_create", "test", nil),
		},
	}

	r.RegisterModule(mod)

	if r.ToolCount() != 2 {
		t.Fatalf("expected 2 tools, got %d", r.ToolCount())
	}
	if r.ModuleCount() != 1 {
		t.Fatalf("expected 1 module, got %d", r.ModuleCount())
	}

	td, ok := r.GetTool("test_list")
	if !ok {
		t.Fatal("test_list not found")
	}
	if td.Category != "test" {
		t.Errorf("category = %q, want test", td.Category)
	}

	mod2, ok := r.GetModule("testmod")
	if !ok {
		t.Fatal("testmod not found")
	}
	if mod2.Name() != "testmod" {
		t.Errorf("module name = %q, want testmod", mod2.Name())
	}
}

func TestRegistryInferIsWrite(t *testing.T) {
	r := NewToolRegistry()

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("my_list", "test", nil),
			newTestTool("my_create", "test", nil),
			newTestTool("my_delete", "test", nil),
		},
	})

	td, _ := r.GetTool("my_list")
	if td.IsWrite {
		t.Error("my_list should not be a write tool")
	}

	td, _ = r.GetTool("my_create")
	if !td.IsWrite {
		t.Error("my_create should be a write tool")
	}

	td, _ = r.GetTool("my_delete")
	if !td.IsWrite {
		t.Error("my_delete should be a write tool")
	}
}

func TestRegistryRuntimeGroupMapper(t *testing.T) {
	mapper := func(category string) string {
		if category == "spotify" {
			return "music"
		}
		return ""
	}

	r := NewToolRegistry(Config{
		RuntimeGroupMapper: mapper,
	})

	r.RegisterModule(&testModule{
		name: "spotify",
		tools: []ToolDefinition{
			newTestTool("spotify_list", "spotify", nil),
			newTestTool("other_list", "other", nil),
		},
	})

	td, _ := r.GetTool("spotify_list")
	if td.RuntimeGroup != "music" {
		t.Errorf("expected runtime group music, got %q", td.RuntimeGroup)
	}

	td, _ = r.GetTool("other_list")
	if td.RuntimeGroup != "" {
		t.Errorf("expected empty runtime group, got %q", td.RuntimeGroup)
	}
}

func TestRegistryListMethods(t *testing.T) {
	r := NewToolRegistry()

	r.RegisterModule(&testModule{
		name: "mod_a",
		tools: []ToolDefinition{
			newTestTool("a_list", "catA", nil),
			newTestTool("a_create", "catA", nil),
		},
	})
	r.RegisterModule(&testModule{
		name: "mod_b",
		tools: []ToolDefinition{
			newTestTool("b_list", "catB", nil),
		},
	})

	modules := r.ListModules()
	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	tools := r.ListTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	catA := r.ListToolsByCategory("catA")
	if len(catA) != 2 {
		t.Fatalf("expected 2 catA tools, got %d", len(catA))
	}

	catB := r.ListToolsByCategory("catB")
	if len(catB) != 1 {
		t.Fatalf("expected 1 catB tools, got %d", len(catB))
	}
}

func TestRegistryGetToolStats(t *testing.T) {
	r := NewToolRegistry()

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("read_list", "test", nil),
			newTestTool("write_create", "test", nil),
			{
				Tool:       Tool{Name: "deprecated_tool"},
				Handler:    func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return nil, nil },
				Category:   "test",
				Deprecated: true,
			},
		},
	})

	stats := r.GetToolStats()
	if stats.TotalTools != 3 {
		t.Errorf("total = %d, want 3", stats.TotalTools)
	}
	if stats.WriteToolsCount != 1 {
		t.Errorf("write count = %d, want 1", stats.WriteToolsCount)
	}
	if stats.DeprecatedCount != 1 {
		t.Errorf("deprecated count = %d, want 1", stats.DeprecatedCount)
	}
}

func TestRegistryMiddleware(t *testing.T) {
	called := false
	middleware := func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
			called = true
			return next(ctx, req)
		}
	}

	r := NewToolRegistry(Config{
		Middleware: []Middleware{middleware},
	})

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("my_tool", "test", nil),
		},
	})

	td, _ := r.GetTool("my_tool")
	wrapped := r.wrapHandler("my_tool", td)

	result, err := wrapped(context.Background(), makeEmptyCallToolRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if !called {
		t.Error("middleware was not called")
	}
}

func TestRegistryPanicRecovery(t *testing.T) {
	r := NewToolRegistry()

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("panic_tool", "test", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
				panic("test panic")
			}),
		},
	})

	td, _ := r.GetTool("panic_tool")
	wrapped := r.wrapHandler("panic_tool", td)

	result, err := wrapped(context.Background(), makeEmptyCallToolRequest())
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result from panic")
	}
}

func TestRegistryTruncation(t *testing.T) {
	r := NewToolRegistry(Config{
		MaxResponseSize: 100,
	})

	largeText := make([]byte, 200)
	for i := range largeText {
		largeText[i] = 'x'
	}

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("large_tool", "test", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
				return MakeTextResult(string(largeText)), nil
			}),
		},
	})

	td, _ := r.GetTool("large_tool")
	wrapped := r.wrapHandler("large_tool", td)

	result, err := wrapped(context.Background(), makeEmptyCallToolRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text, ok := ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("first content is not TextContent")
	}
	if len(text) <= 100 {
		t.Error("expected truncated response to be longer than max (includes suffix)")
	}
}

func TestRegistrySearchTools(t *testing.T) {
	r := NewToolRegistry()

	r.RegisterModule(&testModule{
		name: "discord",
		tools: []ToolDefinition{
			{
				Tool:     Tool{Name: "discord_list_channels", Description: "List Discord channels"},
				Handler:  func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return nil, nil },
				Category: "discord",
				Tags:     []string{"messaging", "chat"},
			},
			{
				Tool:     Tool{Name: "discord_send_message", Description: "Send a Discord message"},
				Handler:  func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) { return nil, nil },
				Category: "discord",
				Tags:     []string{"messaging", "chat"},
			},
		},
	})

	results := r.SearchTools("discord")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'discord', got %d", len(results))
	}

	results = r.SearchTools("list channels")
	if len(results) == 0 {
		t.Fatal("expected results for 'list channels'")
	}

	results = r.SearchTools("")
	if results != nil {
		t.Error("empty query should return nil")
	}
}

func TestInferIsWrite(t *testing.T) {
	writes := []string{"my_create", "my_delete", "my_update", "my_send", "my_start", "my_stop"}
	for _, name := range writes {
		if !InferIsWrite(name) {
			t.Errorf("InferIsWrite(%q) = false, want true", name)
		}
	}

	reads := []string{"my_list", "my_get", "my_status", "my_check", "my_search"}
	for _, name := range reads {
		if InferIsWrite(name) {
			t.Errorf("InferIsWrite(%q) = true, want false", name)
		}
	}
}

func TestApplyMCPAnnotations(t *testing.T) {
	td := ToolDefinition{
		Tool:    Tool{Name: "myapp_inventory_list"},
		IsWrite: false,
	}

	annotated := ApplyMCPAnnotations(td, "myapp_")
	if annotationTitle(annotated) != "Inventory List" {
		t.Errorf("title = %q, want 'Inventory List'", annotationTitle(annotated))
	}
	assertReadOnlyHint(t, annotated, true)
	assertDestructiveHint(t, annotated, false)

	// Write tool
	tdWrite := ToolDefinition{
		Tool:    Tool{Name: "myapp_inventory_delete"},
		IsWrite: true,
	}
	annotated = ApplyMCPAnnotations(tdWrite, "myapp_")
	assertReadOnlyHint(t, annotated, false)
	assertDestructiveHint(t, annotated, true)
}

func TestRegistryHandlerError(t *testing.T) {
	r := NewToolRegistry()

	r.RegisterModule(&testModule{
		name: "test",
		tools: []ToolDefinition{
			newTestTool("error_tool", "test", func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
				return nil, errors.New("handler error")
			}),
		},
	})

	td, _ := r.GetTool("error_tool")
	wrapped := r.wrapHandler("error_tool", td)

	_, err := wrapped(context.Background(), makeEmptyCallToolRequest())
	if err == nil {
		t.Fatal("expected error")
	}
}
