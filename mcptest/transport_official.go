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

func (tr *transport) readResource(ctx context.Context, t testing.TB, uri string) (*registry.ReadResourceResult, error) {
	t.Helper()

	result, err := tr.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		return nil, fmt.Errorf("ReadResource(%s): %w", uri, err)
	}

	return result, nil
}

func (tr *transport) listToolNames(ctx context.Context, t testing.TB) ([]string, error) {
	t.Helper()

	result, err := tr.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("ListTools: %w", err)
	}
	names := make([]string, 0, len(result.Tools))
	for _, tl := range result.Tools {
		names = append(names, tl.Name)
	}
	return names, nil
}

func (tr *transport) listResourceURIs(ctx context.Context, t testing.TB) ([]string, error) {
	t.Helper()

	result, err := tr.session.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		return nil, fmt.Errorf("ListResources: %w", err)
	}
	uris := make([]string, 0, len(result.Resources))
	for _, r := range result.Resources {
		uris = append(uris, r.URI)
	}
	return uris, nil
}

func (tr *transport) listPromptNames(ctx context.Context, t testing.TB) ([]string, error) {
	t.Helper()

	result, err := tr.session.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		return nil, fmt.Errorf("ListPrompts: %w", err)
	}
	names := make([]string, 0, len(result.Prompts))
	for _, p := range result.Prompts {
		names = append(names, p.Name)
	}
	return names, nil
}

func (tr *transport) getPrompt(ctx context.Context, t testing.TB, name string, args map[string]string) (*registry.GetPromptResult, error) {
	t.Helper()

	result, err := tr.session.GetPrompt(ctx, &mcp.GetPromptParams{Name: name, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("GetPrompt(%s): %w", name, err)
	}

	return result, nil
}
