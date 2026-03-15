package registry_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// echoModule is a minimal ToolModule used in examples.
type echoModule struct{}

func (m *echoModule) Name() string        { return "echo" }
func (m *echoModule) Description() string { return "Echo tool module" }
func (m *echoModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool:    registry.Tool{Name: "echo_text", Description: "Echo text back"},
			Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) { return registry.MakeTextResult("ok"), nil },
			Category: "io",
		},
		{
			Tool:    registry.Tool{Name: "echo_json", Description: "Echo JSON back"},
			Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) { return registry.MakeTextResult("{}"), nil },
			Category: "io",
		},
	}
}

func ExampleNewToolRegistry() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&echoModule{})

	fmt.Println(reg.ToolCount())
	fmt.Println(reg.ModuleCount())
	// Output:
	// 2
	// 1
}

func ExampleToolRegistry_ListTools() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&echoModule{})

	tools := reg.ListTools()
	for _, name := range tools {
		fmt.Println(name)
	}
	// Output:
	// echo_json
	// echo_text
}

func ExampleToolRegistry_SearchTools() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&echoModule{})

	results := reg.SearchTools("echo")
	fmt.Println(len(results) > 0)
	fmt.Println(results[0].MatchType)
	// Output:
	// true
	// name
}

func ExampleToolRegistry_GetTool() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&echoModule{})

	td, ok := reg.GetTool("echo_text")
	fmt.Println(ok)
	fmt.Println(td.Category)
	// Output:
	// true
	// io
}
