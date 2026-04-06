//go:build !official_sdk

package prompts

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// testModule implements PromptModule for testing.
type testModule struct {
	name    string
	prompts []PromptDefinition
}

func (m *testModule) Name() string                   { return m.name }
func (m *testModule) Description() string            { return "test module" }
func (m *testModule) Prompts() []PromptDefinition    { return m.prompts }

func simpleHandler(text string) PromptHandlerFunc {
	return func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: text,
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
			},
		}, nil
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewPromptRegistry()

	mod := &testModule{
		name: "testmod",
		prompts: []PromptDefinition{
			{
				Prompt: mcp.NewPrompt("code-review",
					mcp.WithPromptDescription("Review code for issues"),
					mcp.WithArgument("code", mcp.RequiredArgument(), mcp.ArgumentDescription("Code to review")),
				),
				Handler:  simpleHandler("reviewing code"),
				Category: "development",
			},
			{
				Prompt: mcp.NewPrompt("summarize",
					mcp.WithPromptDescription("Summarize text"),
				),
				Handler:  simpleHandler("summarizing"),
				Category: "general",
			},
		},
	}

	r.RegisterModule(mod)

	if r.PromptCount() != 2 {
		t.Fatalf("expected 2 prompts, got %d", r.PromptCount())
	}
	if r.ModuleCount() != 1 {
		t.Fatalf("expected 1 module, got %d", r.ModuleCount())
	}

	pd, ok := r.GetPrompt("code-review")
	if !ok {
		t.Fatal("code-review not found")
	}
	if pd.Category != "development" {
		t.Errorf("category = %q, want development", pd.Category)
	}
	if pd.Prompt.Description != "Review code for issues" {
		t.Errorf("description = %q, want 'Review code for issues'", pd.Prompt.Description)
	}
	if len(pd.Prompt.Arguments) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(pd.Prompt.Arguments))
	}
	if pd.Prompt.Arguments[0].Name != "code" {
		t.Errorf("arg name = %q, want code", pd.Prompt.Arguments[0].Name)
	}
	if !pd.Prompt.Arguments[0].Required {
		t.Error("arg should be required")
	}

	m, ok := r.GetModule("testmod")
	if !ok {
		t.Fatal("testmod not found")
	}
	if m.Name() != "testmod" {
		t.Errorf("module name = %q, want testmod", m.Name())
	}
}

func TestRegistryListPrompts(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{Prompt: mcp.NewPrompt("zulu"), Handler: simpleHandler("")},
			{Prompt: mcp.NewPrompt("alpha"), Handler: simpleHandler("")},
		},
	})

	names := r.ListPrompts()
	if len(names) != 2 {
		t.Fatalf("expected 2, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "zulu" {
		t.Errorf("not sorted: %v", names)
	}
}

func TestRegistryListByCategory(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{Prompt: mcp.NewPrompt("review"), Handler: simpleHandler(""), Category: "dev"},
			{Prompt: mcp.NewPrompt("summarize"), Handler: simpleHandler(""), Category: "general"},
			{Prompt: mcp.NewPrompt("debug"), Handler: simpleHandler(""), Category: "dev"},
		},
	})

	names := r.ListPromptsByCategory("dev")
	if len(names) != 2 {
		t.Fatalf("expected 2 dev prompts, got %d", len(names))
	}
}

func TestRegistryGetAllDefinitions(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{Prompt: mcp.NewPrompt("a"), Handler: simpleHandler("")},
			{Prompt: mcp.NewPrompt("b"), Handler: simpleHandler("")},
		},
	})

	all := r.GetAllPromptDefinitions()
	if len(all) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(all))
	}
}

func TestRegistryHandlerExecution(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{
				Prompt:  mcp.NewPrompt("greet"),
				Handler: simpleHandler("hello world"),
			},
		},
	})

	pd, _ := r.GetPrompt("greet")
	result, err := pd.Handler(context.Background(), mcp.GetPromptRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Description != "hello world" {
		t.Errorf("description = %q, want hello world", result.Description)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
}

func TestRegistryMiddleware(t *testing.T) {
	var order []string

	mw1 := func(name string, pd PromptDefinition, next PromptHandlerFunc) PromptHandlerFunc {
		return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			order = append(order, "mw1-before")
			result, err := next(ctx, req)
			order = append(order, "mw1-after")
			return result, err
		}
	}

	mw2 := func(name string, pd PromptDefinition, next PromptHandlerFunc) PromptHandlerFunc {
		return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			order = append(order, "mw2-before")
			result, err := next(ctx, req)
			order = append(order, "mw2-after")
			return result, err
		}
	}

	r := NewPromptRegistry(Config{
		Middleware: []Middleware{mw1, mw2},
	})
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{
				Prompt: mcp.NewPrompt("test"),
				Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
					order = append(order, "handler")
					return &mcp.GetPromptResult{Description: "ok"}, nil
				},
			},
		},
	})

	pd := r.prompts["test"]
	wrapped := r.wrapHandler("test", pd)
	_, err := wrapped(context.Background(), mcp.GetPromptRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestRegistryPanicRecovery(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{
				Prompt: mcp.NewPrompt("panic"),
				Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
					panic("boom")
				},
			},
		},
	})

	pd := r.prompts["panic"]
	wrapped := r.wrapHandler("panic", pd)
	result, err := wrapped(context.Background(), mcp.GetPromptRequest{})
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if result != nil {
		t.Error("expected nil result from panic")
	}
}

func TestRegistryErrorHandler(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{
				Prompt: mcp.NewPrompt("fail"),
				Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
					return nil, fmt.Errorf("prompt failed")
				},
			},
		},
	})

	pd := r.prompts["fail"]
	wrapped := r.wrapHandler("fail", pd)
	_, err := wrapped(context.Background(), mcp.GetPromptRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "prompt failed" {
		t.Errorf("error = %q, want 'prompt failed'", err.Error())
	}
}

func TestSearchPrompts(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{
				Prompt:   mcp.NewPrompt("code-review", mcp.WithPromptDescription("Review code for quality")),
				Handler:  simpleHandler(""),
				Category: "development",
				Tags:     []string{"quality", "review"},
			},
			{
				Prompt:   mcp.NewPrompt("summarize", mcp.WithPromptDescription("Summarize long text")),
				Handler:  simpleHandler(""),
				Category: "general",
			},
			{
				Prompt:   mcp.NewPrompt("debug-assist", mcp.WithPromptDescription("Help debug issues")),
				Handler:  simpleHandler(""),
				Category: "development",
				Tags:     []string{"debugging"},
			},
		},
	})

	tests := []struct {
		query string
		want  int
	}{
		{"review", 1}, // name "code-review" and tag "review" are same prompt
		{"development", 2},
		{"summarize", 1},
		{"debug", 1},
		{"quality", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := r.SearchPrompts(tt.query)
			if len(results) != tt.want {
				t.Errorf("SearchPrompts(%q) = %d results, want %d", tt.query, len(results), tt.want)
			}
		})
	}
}

func TestDynamicRegistryAddRemove(t *testing.T) {
	d := NewDynamicRegistry()

	var notified int32
	d.OnChange(func() {
		atomic.AddInt32(&notified, 1)
	})

	pd := PromptDefinition{
		Prompt:  mcp.NewPrompt("dynamic-prompt"),
		Handler: simpleHandler("dynamic"),
	}

	d.AddPrompt(pd)
	if d.PromptCount() != 1 {
		t.Fatalf("expected 1 prompt, got %d", d.PromptCount())
	}
	if atomic.LoadInt32(&notified) != 1 {
		t.Error("expected notification on add")
	}

	ok := d.RemovePrompt("dynamic-prompt")
	if !ok {
		t.Error("expected RemovePrompt to return true")
	}
	if d.PromptCount() != 0 {
		t.Fatalf("expected 0 prompts, got %d", d.PromptCount())
	}
	if atomic.LoadInt32(&notified) != 2 {
		t.Error("expected notification on remove")
	}

	ok = d.RemovePrompt("nonexistent")
	if ok {
		t.Error("expected RemovePrompt to return false for nonexistent")
	}
	if atomic.LoadInt32(&notified) != 2 {
		t.Error("should not notify on no-op remove")
	}
}

func TestRegistryNotFoundReturns(t *testing.T) {
	r := NewPromptRegistry()

	_, ok := r.GetPrompt("nonexistent")
	if ok {
		t.Error("expected false for nonexistent prompt")
	}

	_, ok = r.GetModule("nonexistent")
	if ok {
		t.Error("expected false for nonexistent module")
	}
}

func TestEmptyRegistry(t *testing.T) {
	r := NewPromptRegistry()

	if r.PromptCount() != 0 {
		t.Error("expected 0 prompts")
	}
	if r.ModuleCount() != 0 {
		t.Error("expected 0 modules")
	}
	if len(r.ListPrompts()) != 0 {
		t.Error("expected empty prompt list")
	}
	if len(r.SearchPrompts("anything")) != 0 {
		t.Error("expected no search results")
	}
}

func TestHandlerWithArguments(t *testing.T) {
	r := NewPromptRegistry()

	handler := func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		lang := req.Params.Arguments["language"]
		if lang == "" {
			lang = "Go"
		}
		return &mcp.GetPromptResult{
			Description: "Code review prompt",
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Review this "+lang+" code")),
			},
		}, nil
	}

	r.RegisterModule(&testModule{
		name: "test",
		prompts: []PromptDefinition{
			{
				Prompt: mcp.NewPrompt("review",
					mcp.WithArgument("language", mcp.ArgumentDescription("Programming language")),
					mcp.WithArgument("code", mcp.RequiredArgument()),
				),
				Handler: handler,
			},
		},
	})

	pd, _ := r.GetPrompt("review")

	// With argument
	result, err := pd.Handler(context.Background(), mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{
			Name:      "review",
			Arguments: map[string]string{"language": "Python", "code": "print('hi')"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc, ok := result.Messages[0].Content.(mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if tc.Text != "Review this Python code" {
		t.Errorf("text = %q, want 'Review this Python code'", tc.Text)
	}

	// Without optional argument (default)
	result, err = pd.Handler(context.Background(), mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{Name: "review"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc, ok = result.Messages[0].Content.(mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if tc.Text != "Review this Go code" {
		t.Errorf("text = %q, want 'Review this Go code'", tc.Text)
	}
}

func TestPromptDefinition_Version(t *testing.T) {
	r := NewPromptRegistry()
	r.RegisterModule(&testModule{
		name: "versioned",
		prompts: []PromptDefinition{
			{
				Prompt:  mcp.NewPrompt("versioned-prompt"),
				Handler: simpleHandler("v1"),
				Version: "1.2.0",
			},
			{
				Prompt:  mcp.NewPrompt("unversioned-prompt"),
				Handler: simpleHandler("no version"),
			},
		},
	})

	pd, ok := r.GetPrompt("versioned-prompt")
	if !ok {
		t.Fatal("versioned-prompt not found")
	}
	if pd.Version != "1.2.0" {
		t.Errorf("Version = %q, want %q", pd.Version, "1.2.0")
	}

	pd2, ok := r.GetPrompt("unversioned-prompt")
	if !ok {
		t.Fatal("unversioned-prompt not found")
	}
	if pd2.Version != "" {
		t.Errorf("Version = %q, want empty for unversioned", pd2.Version)
	}
}
