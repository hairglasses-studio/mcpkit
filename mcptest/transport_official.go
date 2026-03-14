//go:build official_sdk

package mcptest

import (
	"context"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// transport holds the official SDK client session for communicating with the MCP server.
type transport struct {
	session *mcp.ClientSession
	srv     *Server
}

func newTransport(t testing.TB, s *Server) transport {
	t.Helper()

	ctx := context.Background()

	// Create in-memory transport pair
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// Connect the server side
	if _, err := s.MCP.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("failed to connect server transport: %v", err)
	}

	// Create and connect the client side
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mcptest-client",
		Version: "0.0.0-test",
	}, nil)

	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect client transport: %v", err)
	}

	return transport{session: cs, srv: s}
}

func (tr *transport) callTool(ctx context.Context, t testing.TB, name string, args map[string]interface{}) (*registry.CallToolResult, error) {
	t.Helper()

	params := &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	}

	result, err := tr.session.CallTool(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("CallTool(%s): %w", name, err)
	}

	return result, nil
}
