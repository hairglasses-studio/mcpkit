package mcptest

import (
	"context"
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
	result := c.CallTool("test_echo", map[string]any{"message": "hello"})
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
	result := c.CallTool("test_echo", map[string]any{"message": "hello world"})
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

	c.CallTool("test_echo", map[string]any{"message": "hi"})
	c.CallTool("test_echo", map[string]any{"message": "bye"})

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

	c.CallTool("test_echo", map[string]any{"message": "hi"})
	rec.Reset()
	rec.AssertCallCount(t, 0)
}

func TestReadResource(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result := c.ReadResource("test://greeting")
	AssertResourceText(t, result, "Hello, world!")
}

func TestReadResourceE_NotFound(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	_, err := c.ReadResourceE("test://nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent resource")
	}
}

func TestGetPrompt(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result := c.GetPrompt("greeting", nil)
	AssertPromptMessages(t, result, 2)
}

func TestGetPrompt_WithArgs(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result := c.GetPrompt("personalized", map[string]string{"name": "Alice"})
	AssertPromptMessages(t, result, 1)
	AssertPromptContains(t, result, "Hello, Alice!")
}

func TestAssertResourceText(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result := c.ReadResource("test://greeting")
	AssertResourceText(t, result, "Hello, world!")
}

func TestAssertResourceContains(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result := c.ReadResource("test://greeting")
	AssertResourceContains(t, result, "world")
}

func TestAssertPromptMessages(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result := c.GetPrompt("greeting", nil)
	AssertPromptMessages(t, result, 2)
}

func TestAssertPromptContains(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result := c.GetPrompt("greeting", nil)
	AssertPromptContains(t, result, "hello")
	AssertPromptContains(t, result, "Hello!")
}

func TestClient_CallToolWithContext(t *testing.T) {
	_, c := setupTestServer(t)
	ctx := context.Background()
	result := c.CallToolWithContext(ctx, "test_echo", map[string]any{"message": "ctx-test"})
	AssertToolResult(t, result, "ctx-test")
}

func TestBuildCallToolRequest(t *testing.T) {
	args := map[string]any{"x": 1}
	req := buildCallToolRequest("my-tool", args)
	if req.Params.Name != "my-tool" {
		t.Errorf("Params.Name = %q, want %q", req.Params.Name, "my-tool")
	}
}

func TestBuildCallToolRequest_NilArgs(t *testing.T) {
	req := buildCallToolRequest("tool", nil)
	_ = req // ensures the function was called; nil args sets no Arguments
}

func TestBenchmarkToolHelpers(t *testing.T) {
	// Exercise BenchmarkTool, BenchmarkToolParallel, BenchmarkSuite through
	// a real *testing.B run via t.Run so coverage is collected.
	result := testing.Benchmark(func(b *testing.B) {
		reg := newEchoRegistry()
		BenchmarkTool(b, reg, "echo", map[string]any{"message": "unit"})
	})
	if result.N == 0 {
		t.Error("BenchmarkTool should have run at least once")
	}
}

func TestBenchmarkToolParallelHelper(t *testing.T) {
	result := testing.Benchmark(func(b *testing.B) {
		reg := newEchoRegistry()
		BenchmarkToolParallel(b, reg, "echo", map[string]any{"message": "parallel"})
	})
	if result.N == 0 {
		t.Error("BenchmarkToolParallel should have run at least once")
	}
}

func TestBenchmarkSuiteHelper(t *testing.T) {
	result := testing.Benchmark(func(b *testing.B) {
		reg := newEchoRegistry()
		BenchmarkSuite(b, reg, func(name string) map[string]any {
			return map[string]any{"message": "suite"}
		})
	})
	if result.N == 0 {
		t.Error("BenchmarkSuite should have run at least once")
	}
}

func TestNewServer_WithResourceRegistry(t *testing.T) {
	// Exercises the non-nil resource/prompt registry path in NewServer.
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{})
	s := NewServer(t, reg)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}
