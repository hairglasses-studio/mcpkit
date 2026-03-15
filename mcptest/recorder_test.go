//go:build !official_sdk

package mcptest

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// buildRecorderWithCalls creates a Recorder wired into a test registry, makes n
// calls to toolName, and returns the recorder plus the number of actual calls made.
func buildRecorderWithCalls(t *testing.T, toolName string, n int) *Recorder {
	t.Helper()
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)
	c := NewClient(t, s)
	for i := 0; i < n; i++ {
		c.CallTool(toolName, map[string]interface{}{"message": "ping"})
	}
	return rec
}

func TestNewRecorder_NotNil(t *testing.T) {
	rec := NewRecorder()
	if rec == nil {
		t.Fatal("NewRecorder() returned nil")
	}
}

func TestRecorder_Middleware_NotNil(t *testing.T) {
	rec := NewRecorder()
	mw := rec.Middleware()
	if mw == nil {
		t.Fatal("Middleware() returned nil")
	}
}

func TestRecorder_Middleware_IsCallable(t *testing.T) {
	// Ensure the middleware can be invoked as part of a handler chain.
	rec := NewRecorder()
	called := false
	next := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("ok"), nil
	}
	handler := rec.Middleware()("my-tool", registry.ToolDefinition{}, next)
	req := registry.CallToolRequest{}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("middleware handler error: %v", err)
	}
	if !called {
		t.Error("next handler was not called by middleware")
	}
	if result == nil {
		t.Error("middleware handler returned nil result")
	}
}

func TestRecorder_Calls_Empty(t *testing.T) {
	rec := NewRecorder()
	calls := rec.Calls()
	if len(calls) != 0 {
		t.Errorf("Calls() on new recorder = %d, want 0", len(calls))
	}
}

func TestRecorder_Calls_WithData(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 3)
	calls := rec.Calls()
	if len(calls) != 3 {
		t.Errorf("Calls() count = %d, want 3", len(calls))
	}
	for i, c := range calls {
		if c.Name != "test_echo" {
			t.Errorf("Calls()[%d].Name = %q, want %q", i, c.Name, "test_echo")
		}
	}
}

func TestRecorder_Calls_ReturnsCopy(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 1)
	calls := rec.Calls()
	// Mutate the returned slice — should not affect the recorder's internal state.
	calls[0].Name = "mutated"
	calls2 := rec.Calls()
	if calls2[0].Name == "mutated" {
		t.Error("Calls() should return a copy, not a reference to internal state")
	}
}

func TestRecorder_CallCount_Zero(t *testing.T) {
	rec := NewRecorder()
	if n := rec.CallCount(); n != 0 {
		t.Errorf("CallCount() on empty recorder = %d, want 0", n)
	}
}

func TestRecorder_CallCount_NonZero(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 5)
	if n := rec.CallCount(); n != 5 {
		t.Errorf("CallCount() = %d, want 5", n)
	}
}

func TestRecorder_CallsFor_Matching(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "a"})
	c.CallTool("test_echo", map[string]interface{}{"message": "b"})
	c.CallTool("test_error", nil)

	echoOnly := rec.CallsFor("test_echo")
	if len(echoOnly) != 2 {
		t.Errorf("CallsFor(test_echo) = %d, want 2", len(echoOnly))
	}
	for _, call := range echoOnly {
		if call.Name != "test_echo" {
			t.Errorf("CallsFor returned call with Name = %q", call.Name)
		}
	}
}

func TestRecorder_CallsFor_NoMatch(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 2)
	none := rec.CallsFor("nonexistent_tool")
	if len(none) != 0 {
		t.Errorf("CallsFor(nonexistent) = %d, want 0", len(none))
	}
}

func TestRecorder_Reset_ClearsAll(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 3)
	if rec.CallCount() == 0 {
		t.Fatal("expected calls before reset")
	}
	rec.Reset()
	if n := rec.CallCount(); n != 0 {
		t.Errorf("CallCount() after Reset() = %d, want 0", n)
	}
	if calls := rec.Calls(); len(calls) != 0 {
		t.Errorf("Calls() after Reset() = %d, want 0", len(calls))
	}
}

func TestRecorder_Reset_ThenRecord(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "before"})
	rec.Reset()
	c.CallTool("test_echo", map[string]interface{}{"message": "after"})

	if n := rec.CallCount(); n != 1 {
		t.Errorf("CallCount() after reset + one call = %d, want 1", n)
	}
}

func TestRecorder_AssertCallCount_Pass(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 2)
	if runProbe(t, func(tb testing.TB) { rec.AssertCallCount(tb, 2) }) {
		t.Error("AssertCallCount should not fail when count matches")
	}
}

func TestRecorder_AssertCallCount_Fail(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 2)
	if !runProbe(t, func(tb testing.TB) { rec.AssertCallCount(tb, 5) }) {
		t.Error("AssertCallCount should fail when count does not match")
	}
}

func TestRecorder_AssertCalled_Pass(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 1)
	if runProbe(t, func(tb testing.TB) { rec.AssertCalled(tb, "test_echo") }) {
		t.Error("AssertCalled should not fail when tool was called")
	}
}

func TestRecorder_AssertCalled_Fail(t *testing.T) {
	rec := buildRecorderWithCalls(t, "test_echo", 2)
	if !runProbe(t, func(tb testing.TB) { rec.AssertCalled(tb, "nonexistent_tool") }) {
		t.Error("AssertCalled should fail when tool was never called")
	}
}

func TestRecorder_CapturesResult(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "captured"})

	calls := rec.Calls()
	if len(calls) == 0 {
		t.Fatal("expected recorded calls")
	}
	if calls[0].Result == nil {
		t.Fatal("expected non-nil result in recorded call")
	}
}

func TestRecorder_CapturesArgs(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "arg-value"})

	calls := rec.Calls()
	if len(calls) == 0 {
		t.Fatal("expected recorded calls")
	}
	msg, ok := calls[0].Args["message"]
	if !ok {
		t.Fatal("expected 'message' arg in recorded call")
	}
	if msg != "arg-value" {
		t.Errorf("Args[message] = %q, want %q", msg, "arg-value")
	}
}
