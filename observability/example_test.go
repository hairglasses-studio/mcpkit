//go:build !official_sdk

package observability_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/observability"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func ExampleInit() {
	ctx := context.Background()
	p, shutdown, err := observability.Init(ctx, observability.Config{
		ServiceName: "my-mcp-server",
		// Tracing and metrics disabled — no external dependencies needed.
	})
	if err != nil {
		fmt.Println("init error:", err)
		return
	}
	defer shutdown(ctx)

	fmt.Println(p != nil)
	// Output:
	// true
}

func ExampleProvider_Middleware() {
	ctx := context.Background()
	p, shutdown, _ := observability.Init(ctx, observability.Config{
		ServiceName: "example-server",
	})
	defer shutdown(ctx)

	mw := p.Middleware()

	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "greet", Description: "Greeting tool"},
		Category: "utility",
	}

	wrapped := mw("greet", td, func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("hello"), nil
	})

	result, err := wrapped(ctx, registry.CallToolRequest{})
	fmt.Println(err)
	_ = result
	// Output:
	// <nil>
}
