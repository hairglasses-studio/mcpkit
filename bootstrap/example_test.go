//go:build !official_sdk

package bootstrap_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/bootstrap"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// pingModule is a minimal ToolModule for examples.
type pingModule struct{}

func (m *pingModule) Name() string        { return "ping" }
func (m *pingModule) Description() string { return "Network ping tools" }
func (m *pingModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool:    registry.Tool{Name: "ping", Description: "Ping a host"},
			Handler: func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) { return registry.MakeTextResult("pong"), nil },
			Category: "network",
		},
	}
}

func ExampleGenerateReport() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&pingModule{})

	report := bootstrap.GenerateReport(bootstrap.Config{
		ServerName: "my-server",
		Tools:      reg,
	})

	fmt.Println(report.ServerName)
	fmt.Println(len(report.Tools))
	// Output:
	// my-server
	// 1
}

func ExampleContextReport_FormatText() {
	report := &bootstrap.ContextReport{
		ServerName: "demo",
		Tools: []bootstrap.ToolSummary{
			{Name: "greet", Description: "Greet a user"},
		},
	}

	text := report.FormatText()
	// Check that the server name appears in the output.
	fmt.Println(text[:12])
	// Output:
	// Server: demo
}
