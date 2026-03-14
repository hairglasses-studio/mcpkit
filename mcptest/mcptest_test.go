package mcptest

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestNewServer(t *testing.T) {
	s, _ := setupTestServer(t)
	if !s.HasTool("test_echo") {
		t.Error("server should have test_echo tool")
	}
	names := s.ToolNames()
	if len(names) != 2 {
		t.Errorf("tool count = %d, want 2", len(names))
	}
}

func TestClient_CallTool(t *testing.T) {
	_, c := setupTestServer(t)
	result := c.CallTool("test_echo", map[string]interface{}{"message": "hello"})
	AssertToolResult(t, result, "hello")
	AssertNotError(t, result)
}

func TestClient_CallToolError(t *testing.T) {
	_, c := setupTestServer(t)
	result := c.CallTool("test_error", nil)
	AssertError(t, result, "NOT_FOUND")
}

func TestAssertToolResultContains(t *testing.T) {
	_, c := setupTestServer(t)
	result := c.CallTool("test_echo", map[string]interface{}{"message": "hello world"})
	AssertToolResultContains(t, result, "world")
}

func TestRecorder(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "hi"})
	c.CallTool("test_echo", map[string]interface{}{"message": "bye"})

	rec.AssertCallCount(t, 2)
	rec.AssertCalled(t, "test_echo")

	calls := rec.CallsFor("test_echo")
	if len(calls) != 2 {
		t.Errorf("calls for test_echo = %d, want 2", len(calls))
	}
}

func TestRecorderReset(t *testing.T) {
	rec := NewRecorder()
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&testModule{})

	s := NewServer(t, reg)
	c := NewClient(t, s)

	c.CallTool("test_echo", map[string]interface{}{"message": "hi"})
	rec.Reset()
	rec.AssertCallCount(t, 0)
}
