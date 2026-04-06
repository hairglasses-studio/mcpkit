//go:build !official_sdk

package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/mcptest"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// newTestPeerGateway creates a test MCP server that can act as a peer gateway.
// Returns the HTTP server and its endpoint URL.
func newTestPeerGateway(t *testing.T, tools ...registry.ToolDefinition) *mcptest.HTTPServer {
	t.Helper()
	reg := registry.NewToolRegistry()
	for _, td := range tools {
		reg.RegisterModule(&singleToolModule{td: td})
	}
	httpServer := mcptest.NewHTTPServer(t, reg)
	t.Cleanup(httpServer.Close)
	return httpServer
}

func TestFederation_PeerDiscovery(t *testing.T) {
	// Set up a peer gateway with two tools.
	peer := newTestPeerGateway(t, echoTool("alpha"), echoTool("beta"))

	// Create a local registry and federation.
	localReg := registry.NewDynamicRegistry()
	fed := NewFederation(FederationConfig{
		Peers:             []string{peer.Endpoint()},
		DiscoveryInterval: 24 * time.Hour, // effectively disable periodic refresh
		Namespace:         false,
	}, localReg)

	ctx := context.Background()
	if err := fed.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fed.Stop()

	// After Start, the federation should have discovered the peer's tools.
	allTools := fed.AllTools()
	if len(allTools) != 2 {
		t.Fatalf("expected 2 tools from peer, got %d", len(allTools))
	}

	// Tools should be registered in the local registry.
	regTools := localReg.ListTools()
	if len(regTools) != 2 {
		t.Fatalf("expected 2 tools in local registry, got %d: %v", len(regTools), regTools)
	}

	expected := map[string]bool{"alpha": true, "beta": true}
	for _, name := range regTools {
		if !expected[name] {
			t.Errorf("unexpected tool %q in registry", name)
		}
	}

	// Verify peer health.
	peers := fed.Peers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if !peers[0].Healthy {
		t.Error("expected peer to be healthy")
	}
	if peers[0].LastSeen.IsZero() {
		t.Error("expected LastSeen to be set")
	}
}

func TestFederation_ToolNamespacing(t *testing.T) {
	// Set up two peer gateways with overlapping tool names.
	peer1 := newTestPeerGateway(t, echoTool("action"))
	peer2 := newTestPeerGateway(t, echoTool("action"))

	localReg := registry.NewDynamicRegistry()
	fed := NewFederation(FederationConfig{
		Peers:             []string{peer1.Endpoint(), peer2.Endpoint()},
		DiscoveryInterval: 24 * time.Hour,
		Namespace:         true,
	}, localReg)

	ctx := context.Background()
	if err := fed.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fed.Stop()

	// With namespacing enabled, tools from different peers should have different names.
	regTools := localReg.ListTools()
	if len(regTools) != 2 {
		t.Fatalf("expected 2 namespaced tools, got %d: %v", len(regTools), regTools)
	}

	// Each tool should contain a "/" separator from namespacing.
	for _, name := range regTools {
		if !contains(name, "/") {
			t.Errorf("expected namespaced tool name with '/', got %q", name)
		}
		if !contains(name, "action") {
			t.Errorf("expected tool name to contain 'action', got %q", name)
		}
	}

	// Tools should be distinct.
	if regTools[0] == regTools[1] {
		t.Errorf("expected distinct namespaced tool names, both are %q", regTools[0])
	}
}

func TestFederation_RouteToRemote(t *testing.T) {
	peer := newTestPeerGateway(t, echoTool("greet"))

	localReg := registry.NewDynamicRegistry()
	fed := NewFederation(FederationConfig{
		Peers:             []string{peer.Endpoint()},
		DiscoveryInterval: 24 * time.Hour,
		Namespace:         false,
	}, localReg)

	ctx := context.Background()
	if err := fed.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fed.Stop()

	// Route should find the peer.
	pg, err := fed.RouteCall("greet")
	if err != nil {
		t.Fatalf("RouteCall: %v", err)
	}
	if pg.Endpoint != peer.Endpoint() {
		t.Errorf("expected endpoint %q, got %q", peer.Endpoint(), pg.Endpoint)
	}
	if !pg.Healthy {
		t.Error("expected routed peer to be healthy")
	}

	// Route to unknown tool should fail.
	_, err = fed.RouteCall("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}

	// Actually call the tool through the proxy handler registered in the registry.
	td, ok := localReg.GetTool("greet")
	if !ok {
		t.Fatal("tool 'greet' not found in local registry")
	}

	result, err := td.Handler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "greet",
			Arguments: map[string]any{"message": "world"},
		},
	})
	if err != nil {
		t.Fatalf("proxy call: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatalf("expected success, got error result")
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if text != "echo:greet:world" {
		t.Fatalf("expected 'echo:greet:world', got %q", text)
	}
}

func TestFederation_PeerHealthCheck(t *testing.T) {
	peer := newTestPeerGateway(t, echoTool("tool1"))

	localReg := registry.NewDynamicRegistry()
	fed := NewFederation(FederationConfig{
		Peers:             []string{peer.Endpoint()},
		DiscoveryInterval: 24 * time.Hour,
		Namespace:         false,
	}, localReg)

	ctx := context.Background()
	if err := fed.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fed.Stop()

	// Peer should be healthy after discovery.
	peers := fed.Peers()
	if len(peers) != 1 || !peers[0].Healthy {
		t.Fatal("expected 1 healthy peer")
	}

	// Now test with an unreachable peer.
	localReg2 := registry.NewDynamicRegistry()
	fed2 := NewFederation(FederationConfig{
		Peers:             []string{"http://127.0.0.1:1/mcp"}, // unreachable
		DiscoveryInterval: 24 * time.Hour,
		Namespace:         false,
	}, localReg2)

	if err := fed2.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fed2.Stop()

	// Unreachable peer should not be healthy.
	peers2 := fed2.Peers()
	if len(peers2) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers2))
	}
	if peers2[0].Healthy {
		t.Error("expected unreachable peer to be unhealthy")
	}

	// No tools should be discovered.
	if len(fed2.AllTools()) != 0 {
		t.Errorf("expected 0 tools from unreachable peer, got %d", len(fed2.AllTools()))
	}
}

func TestFederation_Concurrent(t *testing.T) {
	peer := newTestPeerGateway(t, echoTool("concurrent_tool"))

	localReg := registry.NewDynamicRegistry()
	fed := NewFederation(FederationConfig{
		Peers:             []string{peer.Endpoint()},
		DiscoveryInterval: 24 * time.Hour,
		Namespace:         false,
	}, localReg)

	ctx := context.Background()
	if err := fed.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fed.Stop()

	// Run concurrent operations: AllTools, RouteCall, Peers.
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			switch n % 3 {
			case 0:
				tools := fed.AllTools()
				if len(tools) != 1 {
					t.Errorf("goroutine %d: expected 1 tool, got %d", n, len(tools))
				}
			case 1:
				pg, err := fed.RouteCall("concurrent_tool")
				if err != nil {
					t.Errorf("goroutine %d: RouteCall: %v", n, err)
				} else if !pg.Healthy {
					t.Errorf("goroutine %d: expected healthy peer", n)
				}
			case 2:
				peers := fed.Peers()
				if len(peers) != 1 {
					t.Errorf("goroutine %d: expected 1 peer, got %d", n, len(peers))
				}
			}
		}(i)
	}

	wg.Wait()
}

// contains checks if substr is in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
