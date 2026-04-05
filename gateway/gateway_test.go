//go:build !official_sdk

package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/mcptest"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// newTestUpstream creates a test upstream with a simple echo tool.
func newTestUpstream(t *testing.T, name string, tools ...registry.ToolDefinition) (*mcptest.HTTPServer, UpstreamConfig) {
	t.Helper()
	reg := registry.NewToolRegistry()
	for _, td := range tools {
		reg.RegisterModule(&singleToolModule{td: td})
	}
	httpServer := mcptest.NewHTTPServer(t, reg)
	t.Cleanup(httpServer.Close)
	return httpServer, UpstreamConfig{
		Name:           name,
		URL:            httpServer.Endpoint(),
		HealthInterval: 24 * time.Hour, // effectively disable health polling in tests
	}
}

// singleToolModule wraps a single ToolDefinition as a ToolModule.
type singleToolModule struct {
	td registry.ToolDefinition
}

func (m *singleToolModule) Name() string        { return m.td.Tool.Name }
func (m *singleToolModule) Description() string { return m.td.Tool.Description }
func (m *singleToolModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{m.td}
}

func echoTool(name string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: mcp.Tool{
			Name:        name,
			Description: "Echo tool: " + name,
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"message": map[string]any{"type": "string"},
				},
			},
		},
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := registry.ExtractArguments(req)
			msg, _ := args["message"].(string)
			return registry.MakeTextResult("echo:" + name + ":" + msg), nil
		},
	}
}

func TestAddUpstream(t *testing.T) {
	_, cfg := newTestUpstream(t, "svc1", echoTool("greet"), echoTool("farewell"))
	gw, reg := NewGateway()
	defer gw.Close()

	count, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 tools, got %d", count)
	}

	tools := reg.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 registered tools, got %d: %v", len(tools), tools)
	}

	expected := map[string]bool{"svc1.greet": true, "svc1.farewell": true}
	for _, name := range tools {
		if !expected[name] {
			t.Errorf("unexpected tool %q", name)
		}
	}
}

func TestAddUpstream_AllowedToolsFilter(t *testing.T) {
	_, cfg := newTestUpstream(t, "svc1", echoTool("greet"), echoTool("farewell"))
	cfg.AllowedTools = []string{"farewell"}

	gw, reg := NewGateway()
	defer gw.Close()

	count, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 allowed tool, got %d", count)
	}

	tools := reg.ListTools()
	if len(tools) != 1 || tools[0] != "svc1.farewell" {
		t.Fatalf("expected only svc1.farewell, got %v", tools)
	}
}

func TestProxyToolCall(t *testing.T) {
	_, cfg := newTestUpstream(t, "echo", echoTool("say"))
	gw, reg := NewGateway()
	defer gw.Close()

	_, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	td, ok := reg.GetTool("echo.say")
	if !ok {
		t.Fatal("tool echo.say not found in registry")
	}

	result, err := td.Handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "echo.say",
			Arguments: map[string]any{"message": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatalf("expected success, got error result")
	}

	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if text != "echo:say:hello" {
		t.Fatalf("expected 'echo:say:hello', got %q", text)
	}
}

func TestRemoveUpstream(t *testing.T) {
	_, cfg := newTestUpstream(t, "removeme", echoTool("tool1"))
	gw, reg := NewGateway()
	defer gw.Close()

	_, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	if len(reg.ListTools()) != 1 {
		t.Fatalf("expected 1 tool before removal")
	}

	ok := gw.RemoveUpstream("removeme")
	if !ok {
		t.Fatal("expected RemoveUpstream to return true")
	}

	if len(reg.ListTools()) != 0 {
		t.Fatalf("expected 0 tools after removal, got %v", reg.ListTools())
	}

	// Removing again should return false
	if gw.RemoveUpstream("removeme") {
		t.Fatal("expected RemoveUpstream to return false for removed upstream")
	}
}

func TestDuplicateUpstreamName(t *testing.T) {
	_, cfg := newTestUpstream(t, "dup", echoTool("tool1"))
	gw, _ := NewGateway()
	defer gw.Close()

	_, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first AddUpstream: %v", err)
	}

	_, err = gw.AddUpstream(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for duplicate upstream name")
	}
}

func TestMultipleUpstreams(t *testing.T) {
	_, cfg1 := newTestUpstream(t, "svc1", echoTool("action"))
	_, cfg2 := newTestUpstream(t, "svc2", echoTool("action"))
	gw, reg := NewGateway()
	defer gw.Close()

	_, err := gw.AddUpstream(context.Background(), cfg1)
	if err != nil {
		t.Fatalf("AddUpstream svc1: %v", err)
	}
	_, err = gw.AddUpstream(context.Background(), cfg2)
	if err != nil {
		t.Fatalf("AddUpstream svc2: %v", err)
	}

	tools := reg.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(tools), tools)
	}

	expected := map[string]bool{"svc1.action": true, "svc2.action": true}
	for _, name := range tools {
		if !expected[name] {
			t.Errorf("unexpected tool %q", name)
		}
	}
}

func TestCloseGateway(t *testing.T) {
	_, cfg := newTestUpstream(t, "closeme", echoTool("tool1"))
	gw, _ := NewGateway()

	_, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	if err := gw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Operations after close should fail
	_, err = gw.AddUpstream(context.Background(), UpstreamConfig{Name: "new", URL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error after close")
	}

	// Double close should return error
	if err := gw.Close(); err == nil {
		t.Fatal("expected error on double close")
	}
}

func TestUpstreamStatus(t *testing.T) {
	_, cfg := newTestUpstream(t, "status", echoTool("tool1"), echoTool("tool2"))
	gw, _ := NewGateway()
	defer gw.Close()

	_, err := gw.AddUpstream(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	info, err := gw.UpstreamStatus("status")
	if err != nil {
		t.Fatalf("UpstreamStatus: %v", err)
	}
	if !info.Healthy {
		t.Error("expected healthy")
	}
	if info.ToolCount != 2 {
		t.Errorf("expected 2 tools, got %d", info.ToolCount)
	}

	// Unknown upstream
	_, err = gw.UpstreamStatus("unknown")
	if err == nil {
		t.Fatal("expected error for unknown upstream")
	}
}
