//go:build !official_sdk

package logging_test

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hairglasses-studio/mcpkit/logging"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func ExampleMiddleware() {
	logger := slog.Default()
	mw := logging.Middleware(logger)

	td := registry.ToolDefinition{
		Tool:     registry.Tool{Name: "echo", Description: "Echo input"},
		Category: "utility",
	}

	handler := mw("echo", td, func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("pong"), nil
	})

	result, err := handler(context.Background(), registry.CallToolRequest{})
	fmt.Println(err)
	_ = result
	// Output:
	// <nil>
}

type noopSender struct{}

func (n *noopSender) SendLog(_ context.Context, _, _ string, _ any) error { return nil }

func ExampleNewHandler() {
	h := logging.NewHandler(&noopSender{})
	fmt.Println(h.Enabled(context.Background(), slog.LevelInfo))
	// Output:
	// true
}
