//go:build !official_sdk

// Command conformance-server runs the mcpkit everything-server on stdio for
// the official MCP conformance suite.
//
// Usage:
//
//	go build -o conformance-server ./testing/conformance/cmd/
//	./conformance-server
//
// The server implements all testable MCP capabilities (tools, resources, prompts,
// logging, completions) and speaks JSON-RPC over stdin/stdout as required by the
// conformance runner in server mode.
package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/testing/conformance"
)

func main() {
	cfg := conformance.DefaultConfig()
	s := conformance.NewEverythingServer(cfg)

	log.Printf("mcpkit conformance everything-server %s starting on stdio", cfg.Version)
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
