//go:build !official_sdk

// Command truncate-demo demonstrates the truncation middleware, which limits
// response size to prevent oversized payloads from consuming model context.
//
// The server registers a single tool that returns a large response (20KB of
// repeated text). The truncation middleware caps output at 4KB and appends a
// guidance message directing the model to refine its query.
//
// Usage:
//
//	go run ./examples/truncate-demo
//	npx @modelcontextprotocol/inspector go run ./examples/truncate-demo
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/middleware/truncate"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- Input/Output types ---

type DumpInput struct {
	Lines int `json:"lines,omitempty" jsonschema:"description=Number of lines to generate (default 500)"`
}

type DumpOutput struct {
	Data string `json:"data"`
}

// --- Module ---

type DemoModule struct{}

func (m *DemoModule) Name() string        { return "demo" }
func (m *DemoModule) Description() string { return "Truncation demo tools" }
func (m *DemoModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		handler.TypedHandler[DumpInput, DumpOutput](
			"dump_logs",
			"Return a large block of log-like text. Without truncation middleware this would flood the context window.",
			func(_ context.Context, input DumpInput) (DumpOutput, error) {
				lines := input.Lines
				if lines <= 0 {
					lines = 500
				}
				var sb strings.Builder
				for i := 1; i <= lines; i++ {
					fmt.Fprintf(&sb, "[%04d] INFO  service=demo msg=\"processing request\" latency=12ms status=200\n", i)
				}
				return DumpOutput{Data: sb.String()}, nil
			},
		),
	}
}

func main() {
	// Create a tool registry with truncation middleware.
	// MaxBytes=4096 caps text content at 4KB per response.
	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{
			truncate.New(truncate.WithMaxBytes(4096)),
		},
	})
	reg.RegisterModule(&DemoModule{})

	s := registry.NewMCPServer("truncate-demo", "1.0.0")
	reg.RegisterWithServer(s)

	log.Println("truncate-demo: serving on stdio (4KB response limit)")
	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
