//go:build !official_sdk

package toolindex

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func testRegistry() *registry.ToolRegistry {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&stubModule{
		name: "net",
		desc: "Networking tools",
		tools: []registry.ToolDefinition{
			{
				Tool:     registry.Tool{Name: "net_ping", Description: "Ping a host"},
				Category: "network",
				Handler:  nopHandler,
			},
			{
				Tool:     registry.Tool{Name: "net_traceroute", Description: "Trace network route to a host"},
				Category: "network",
				Handler:  nopHandler,
			},
			{
				Tool:     registry.Tool{Name: "net_dns_lookup", Description: "Look up DNS records"},
				Category: "network",
				Handler:  nopHandler,
			},
		},
	})
	reg.RegisterModule(&stubModule{
		name: "fs",
		desc: "Filesystem tools",
		tools: []registry.ToolDefinition{
			{
				Tool:     registry.Tool{Name: "fs_read_file", Description: "Read a file from disk"},
				Category: "filesystem",
				Handler:  nopHandler,
			},
			{
				Tool:     registry.Tool{Name: "fs_write_file", Description: "Write content to a file"},
				Category: "filesystem",
				IsWrite:  true,
				Handler:  nopHandler,
			},
		},
	})
	return reg
}

type stubModule struct {
	name  string
	desc  string
	tools []registry.ToolDefinition
}

func (m *stubModule) Name() string                       { return m.name }
func (m *stubModule) Description() string                { return m.desc }
func (m *stubModule) Tools() []registry.ToolDefinition   { return m.tools }

var nopHandler = func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("ok"), nil
}

func TestCatalogReturnsAllTools(t *testing.T) {
	reg := testRegistry()
	mod := NewToolIndexModule("test", reg)

	// Register the module itself so catalog/search tools are also present.
	reg.RegisterModule(mod)

	tools := mod.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	// Call the catalog tool with no filter.
	catalogTD := tools[0]
	result, err := catalogTD.Handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var out catalogOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal catalog output: %v", err)
	}

	// 5 stub tools + 2 discovery tools = 7.
	if out.TotalTools != 7 {
		t.Errorf("expected 7 total tools, got %d", out.TotalTools)
	}
	// Categories: discovery, filesystem, network.
	if len(out.Groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(out.Groups))
	}
}

func TestCatalogWithCategoryFilter(t *testing.T) {
	reg := testRegistry()
	mod := NewToolIndexModule("test", reg)
	reg.RegisterModule(mod)

	catalogTD := mod.Tools()[0]
	result, err := catalogTD.Handler(context.Background(), makeRequest(map[string]any{
		"category": "network",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var out catalogOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(out.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(out.Groups))
	}
	if out.Groups[0].Category != "network" {
		t.Errorf("expected category 'network', got %q", out.Groups[0].Category)
	}
	if out.Groups[0].ToolCount != 3 {
		t.Errorf("expected 3 tools in network, got %d", out.Groups[0].ToolCount)
	}
}

func TestSearchByNameSubstring(t *testing.T) {
	reg := testRegistry()
	mod := NewToolIndexModule("test", reg)
	reg.RegisterModule(mod)

	searchTD := mod.Tools()[1]
	result, err := searchTD.Handler(context.Background(), makeRequest(map[string]any{
		"query": "ping",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var out searchOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if out.Total != 1 {
		t.Fatalf("expected 1 result, got %d", out.Total)
	}
	if out.Results[0].Name != "net_ping" {
		t.Errorf("expected net_ping, got %q", out.Results[0].Name)
	}
}

func TestSearchByDescriptionSubstring(t *testing.T) {
	reg := testRegistry()
	mod := NewToolIndexModule("test", reg)
	reg.RegisterModule(mod)

	searchTD := mod.Tools()[1]
	result, err := searchTD.Handler(context.Background(), makeRequest(map[string]any{
		"query": "dns records",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var out searchOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if out.Total != 1 {
		t.Fatalf("expected 1 result, got %d", out.Total)
	}
	if out.Results[0].Name != "net_dns_lookup" {
		t.Errorf("expected net_dns_lookup, got %q", out.Results[0].Name)
	}
}

// --- helpers ---

func makeRequest(args map[string]any) registry.CallToolRequest {
	req := registry.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func extractText(t *testing.T, result *registry.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("empty content")
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("content is not text")
	}
	return text
}
