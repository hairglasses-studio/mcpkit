//go:build !official_sdk

package mcptest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hairglasses-studio/mcpkit/mcptest"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// minimalModule registers a single no-op tool for examples.
type minimalModule struct{}

func (m *minimalModule) Name() string        { return "minimal" }
func (m *minimalModule) Description() string { return "Minimal module for examples" }
func (m *minimalModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool:    registry.Tool{Name: "greet", Description: "Greet the user"},
			Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) { return registry.MakeTextResult("hello"), nil },
		},
	}
}

func ExampleNewRecorder() {
	rec := mcptest.NewRecorder()

	mw := rec.Middleware()
	fmt.Println(mw != nil)
	// Output:
	// true
}

func ExampleRecorder_Calls() {
	rec := mcptest.NewRecorder()

	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{rec.Middleware()},
	})
	reg.RegisterModule(&minimalModule{})

	fmt.Println(len(rec.Calls()))
	// Output:
	// 0
}

func ExampleNewServer() {
	// Example is shown as a test helper since NewServer requires testing.TB.
	// In production tests use: srv := mcptest.NewServer(t, reg)
	t := &testing.T{}
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&minimalModule{})

	srv := mcptest.NewServer(t, reg)
	fmt.Println(srv.HasTool("greet"))
	fmt.Println(len(srv.ToolNames()))
	// Output:
	// true
	// 1
}
