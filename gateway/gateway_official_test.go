//go:build official_sdk

package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	legacymcp "github.com/mark3labs/mcp-go/mcp"
	legacyserver "github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/registry"
)

type observedGatewayRequest struct {
	method        string
	name          string
	protocol      string
	authorization string
}

type requestObserver struct {
	mu       sync.Mutex
	requests []observedGatewayRequest
}

func (o *requestObserver) handler(next http.Handler, requireAuth string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		o.requests = append(o.requests, observedGatewayRequest{
			method:        r.Header.Get("Mcp-Method"),
			name:          r.Header.Get("Mcp-Name"),
			protocol:      r.Header.Get("MCP-Protocol-Version"),
			authorization: r.Header.Get("Authorization"),
		})
		o.mu.Unlock()
		if requireAuth != "" && r.Header.Get("Authorization") != requireAuth {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (o *requestObserver) saw(method, name, authorization string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, request := range o.requests {
		if request.method == method && request.name == name && request.authorization == authorization && request.protocol == "2026-07-28" {
			return true
		}
	}
	return false
}

func (o *requestObserver) count(method string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	count := 0
	for _, request := range o.requests {
		if request.method == method {
			count++
		}
	}
	return count
}

func newOfficialGatewayUpstream(t *testing.T, observer *requestObserver, requireAuth string, toolNames ...string) *httptest.Server {
	t.Helper()
	reg := registry.NewDynamicRegistry()
	for _, name := range toolNames {
		name := name
		reg.AddTool(registry.ToolDefinition{
			Tool: registry.Tool{
				Name:        name,
				Description: "official SDK test tool " + name,
				InputSchema: registry.MakeToolInputSchema(map[string]any{
					"value": map[string]any{"type": "string"},
				}, nil, nil),
			},
			Handler: func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
				return registry.MakeTextResult(fmt.Sprintf("%s:%v", name, registry.ExtractArguments(req)["value"])), nil
			},
		})
	}
	server := registry.NewMCPServer("gateway-official-test", "1.0.0")
	reg.RegisterWithServer(server)
	handler := registry.NewStreamableHTTPHandler(server, registry.HTTPServerOptions{Stateless: true})
	return httptest.NewServer(observer.handler(handler, requireAuth))
}

func newLegacyGatewayUpstream(t *testing.T, observer *requestObserver) (*httptest.Server, string) {
	t.Helper()
	server := legacyserver.NewMCPServer("gateway-legacy-test", "1.0.0")
	server.AddTool(
		legacymcp.NewTool("legacy_echo", legacymcp.WithString("value")),
		func(_ context.Context, req legacymcp.CallToolRequest) (*legacymcp.CallToolResult, error) {
			value, _ := req.GetArguments()["value"].(string)
			return legacymcp.NewToolResultText("legacy:" + value), nil
		},
	)
	handler := legacyserver.NewStreamableHTTPServer(server, legacyserver.WithStateLess(true))
	httpServer := httptest.NewServer(observer.handler(handler, ""))
	return httpServer, httpServer.URL + "/mcp"
}

func TestOfficialGatewayHeaderAwareRoutingAndDeterministicCatalog(t *testing.T) {
	observer := new(requestObserver)
	upstreamServer := newOfficialGatewayUpstream(t, observer, "", "zeta", "alpha")
	defer upstreamServer.Close()

	gw, reg := NewGateway()
	defer func() { _ = gw.Close() }()
	count, err := gw.AddUpstream(t.Context(), UpstreamConfig{
		Name: "modern",
		URL:  upstreamServer.URL,
	})
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	if count != 2 {
		t.Fatalf("AddUpstream count = %d, want 2", count)
	}

	wantCatalog := []string{"modern.alpha", "modern.zeta"}
	if got := reg.ListTools(); !slices.Equal(got, wantCatalog) {
		t.Fatalf("catalog = %v, want %v", got, wantCatalog)
	}
	if got := reg.ListTools(); !slices.Equal(got, wantCatalog) {
		t.Fatalf("repeated catalog = %v, want stable %v", got, wantCatalog)
	}

	td, ok := reg.GetTool("modern.alpha")
	if !ok {
		t.Fatal("namespaced alpha tool not registered")
	}
	req, err := registry.NewCallToolRequest("modern.alpha", map[string]any{"value": "routed"})
	if err != nil {
		t.Fatalf("NewCallToolRequest: %v", err)
	}
	result, err := td.Handler(t.Context(), req)
	if err != nil {
		t.Fatalf("proxy handler: %v", err)
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok || text != "alpha:routed" {
		t.Fatalf("proxy result = %q, %v; want alpha:routed, true", text, ok)
	}

	for _, want := range []struct {
		method string
		name   string
	}{
		{method: "server/discover"},
		{method: "tools/list"},
		{method: "tools/call", name: "alpha"},
	} {
		if !observer.saw(want.method, want.name, "") {
			t.Errorf("did not observe %s with Mcp-Name %q and modern protocol header", want.method, want.name)
		}
	}
	if observer.saw("tools/call", "modern.alpha", "") {
		t.Error("forwarded Mcp-Name retained the gateway namespace")
	}
}

func TestOfficialGatewayHealthProbeUses2026ToolsList(t *testing.T) {
	observer := new(requestObserver)
	upstreamServer := newOfficialGatewayUpstream(t, observer, "", "healthy")
	defer upstreamServer.Close()

	client, err := newUpstreamClient(t.Context(), upstreamServer.URL, "mcpkit-gateway-health-test", "")
	if err != nil {
		t.Fatalf("newUpstreamClient: %v", err)
	}
	u := &upstream{
		config: UpstreamConfig{
			Name:               "modern-health",
			HealthInterval:     5 * time.Millisecond,
			UnhealthyThreshold: 1,
		},
		client: client,
	}
	u.healthy.Store(true)
	ctx, cancel := context.WithCancel(t.Context())
	u.startHealthLoop(ctx, nil)
	t.Cleanup(func() {
		cancel()
		_ = u.close()
	})

	deadline := time.Now().Add(time.Second)
	for observer.count("tools/list") == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !observer.saw("tools/list", "", "") {
		t.Fatal("health loop did not send tools/list with MCP 2026-07-28 request metadata")
	}
	if observer.count("ping") != 0 {
		t.Fatalf("health loop sent removed MCP method ping %d time(s)", observer.count("ping"))
	}
	if !u.healthy.Load() {
		t.Fatal("successful 2026-07-28 health probe marked upstream unhealthy")
	}
}

func TestOfficialFederationPreservesBearerAuthorizationAndNamespace(t *testing.T) {
	observer := new(requestObserver)
	upstreamServer := newOfficialGatewayUpstream(t, observer, "Bearer fleet-token", "secure")
	defer upstreamServer.Close()

	reg := registry.NewDynamicRegistry()
	federation := NewFederation(FederationConfig{
		Peers:             []string{upstreamServer.URL},
		DiscoveryInterval: time.Hour,
		Namespace:         true,
		AuthToken:         "fleet-token",
	}, reg)
	if err := federation.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = federation.Stop() }()

	toolName := federation.federatedToolName(upstreamServer.URL, "secure")
	if got := reg.ListTools(); !slices.Equal(got, []string{toolName}) {
		t.Fatalf("federated catalog = %v, want [%s]", got, toolName)
	}
	td, ok := reg.GetTool(toolName)
	if !ok {
		t.Fatalf("federated tool %q not registered", toolName)
	}
	req, err := registry.NewCallToolRequest(toolName, map[string]any{"value": "authorized"})
	if err != nil {
		t.Fatalf("NewCallToolRequest: %v", err)
	}
	result, err := td.Handler(t.Context(), req)
	if err != nil {
		t.Fatalf("federated handler: %v", err)
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok || text != "secure:authorized" {
		t.Fatalf("federated result = %q, %v; want secure:authorized, true", text, ok)
	}

	for _, want := range []struct {
		method string
		name   string
	}{
		{method: "server/discover"},
		{method: "tools/list"},
		{method: "tools/call", name: "secure"},
	} {
		if !observer.saw(want.method, want.name, "Bearer fleet-token") {
			t.Errorf("did not observe authorized %s with Mcp-Name %q", want.method, want.name)
		}
	}
}

func TestOfficialGatewayFallsBackToLegacyUpstream(t *testing.T) {
	observer := new(requestObserver)
	upstreamServer, endpoint := newLegacyGatewayUpstream(t, observer)
	defer upstreamServer.Close()

	gw, reg := NewGateway()
	defer func() { _ = gw.Close() }()
	count, err := gw.AddUpstream(t.Context(), UpstreamConfig{
		Name: "legacy",
		URL:  endpoint,
	})
	if err != nil {
		t.Fatalf("AddUpstream through v1.7.0 fallback: %v", err)
	}
	if count != 1 {
		t.Fatalf("AddUpstream count = %d, want 1", count)
	}
	if !observer.saw("server/discover", "", "") {
		t.Fatal("official client did not attempt 2026-07-28 server/discover before legacy fallback")
	}

	td, ok := reg.GetTool("legacy.legacy_echo")
	if !ok {
		t.Fatal("legacy tool not registered after discovery fallback")
	}
	req, err := registry.NewCallToolRequest("legacy.legacy_echo", map[string]any{"value": "compatible"})
	if err != nil {
		t.Fatalf("NewCallToolRequest: %v", err)
	}
	result, err := td.Handler(t.Context(), req)
	if err != nil {
		t.Fatalf("legacy proxy handler: %v", err)
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok || text != "legacy:compatible" {
		t.Fatalf("legacy proxy result = %q, %v; want legacy:compatible, true", text, ok)
	}
}
