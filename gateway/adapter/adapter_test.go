package adapter

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// mockAdapter is a test adapter that returns canned responses.
type mockAdapter struct {
	protocol Protocol
	tools    []mcp.Tool
	healthy  bool
	closed   bool
}

func (m *mockAdapter) Protocol() Protocol { return m.protocol }
func (m *mockAdapter) Connect(ctx context.Context) error { return nil }
func (m *mockAdapter) DiscoverTools(ctx context.Context) ([]mcp.Tool, error) {
	return m.tools, nil
}
func (m *mockAdapter) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "mock response"}},
	}, nil
}
func (m *mockAdapter) Healthy(ctx context.Context) error {
	if !m.healthy {
		return &UnsupportedProtocolError{Protocol: "unhealthy"}
	}
	return nil
}
func (m *mockAdapter) Close() error { m.closed = true; return nil }

func TestRegistry_RegisterAndCreate(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()

	factory := func(ctx context.Context, cfg Config) (ProtocolAdapter, error) {
		return &mockAdapter{protocol: cfg.Protocol, healthy: true}, nil
	}
	reg.Register(ProtocolMCP, factory)

	adapter, err := reg.Create(context.Background(), Config{
		Protocol: ProtocolMCP,
		Name:     "test",
		URL:      "http://localhost:8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Protocol() != ProtocolMCP {
		t.Errorf("Protocol = %q, want mcp", adapter.Protocol())
	}
}

func TestRegistry_UnsupportedProtocol(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()

	_, err := reg.Create(context.Background(), Config{
		Protocol: "unknown",
		Name:     "test",
	})
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
	if _, ok := err.(*UnsupportedProtocolError); !ok {
		t.Errorf("error type = %T, want *UnsupportedProtocolError", err)
	}
}

func TestRegistry_Has(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	reg.Register(ProtocolA2A, func(ctx context.Context, cfg Config) (ProtocolAdapter, error) {
		return &mockAdapter{protocol: ProtocolA2A}, nil
	})

	if !reg.Has(ProtocolA2A) {
		t.Error("should have a2a")
	}
	if reg.Has(ProtocolGRPC) {
		t.Error("should not have grpc")
	}
}

func TestRegistry_Protocols(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	reg.Register(ProtocolMCP, nil)
	reg.Register(ProtocolA2A, nil)

	protocols := reg.Protocols()
	if len(protocols) != 2 {
		t.Errorf("Protocols = %d, want 2", len(protocols))
	}
}

func TestMockAdapter_Lifecycle(t *testing.T) {
	t.Parallel()
	adapter := &mockAdapter{
		protocol: ProtocolMCP,
		tools:    []mcp.Tool{{Name: "test_tool"}},
		healthy:  true,
	}

	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	tools, err := adapter.DiscoverTools(context.Background())
	if err != nil || len(tools) != 1 {
		t.Errorf("tools = %d, want 1", len(tools))
	}

	result, err := adapter.CallTool(context.Background(), "test_tool", nil)
	if err != nil || result == nil {
		t.Fatal("expected result")
	}

	if err := adapter.Healthy(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if !adapter.closed {
		t.Error("adapter should be closed")
	}
}
