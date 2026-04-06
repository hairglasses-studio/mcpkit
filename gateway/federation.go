//go:build !official_sdk

package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ToolInfo is a lightweight descriptor for a tool exposed by a peer gateway.
// It carries the tool name, description, and originating peer — but no handler,
// since the handler is generated dynamically as a proxy.
type ToolInfo struct {
	// Name is the tool name as advertised by the peer (before namespacing).
	Name string

	// Description is the human-readable tool description.
	Description string

	// Peer is the endpoint of the gateway that owns this tool.
	Peer string
}

// FederationConfig configures gateway-to-gateway discovery and proxying.
type FederationConfig struct {
	// Peers is the list of known gateway endpoints to federate with.
	Peers []string

	// DiscoveryInterval is how often to refresh peer tool catalogs.
	// Default: 60s.
	DiscoveryInterval time.Duration

	// Namespace prefixes tool names from remote gateways to avoid conflicts.
	// When true, tools are registered as "peer-host/tool_name".
	Namespace bool

	// AuthToken for inter-gateway authentication (shared secret or JWT).
	// Sent as Bearer token in the Authorization header. Empty means no auth.
	AuthToken string
}

func (c *FederationConfig) applyDefaults() {
	if c.DiscoveryInterval == 0 {
		c.DiscoveryInterval = 60 * time.Second
	}
}

// PeerGateway represents a remote gateway with its tool catalog.
type PeerGateway struct {
	// Endpoint is the streamable HTTP URL of the peer.
	Endpoint string

	// Tools is the current catalog of tools discovered from the peer.
	Tools []ToolInfo

	// LastSeen is the last time the peer responded to a discovery request.
	LastSeen time.Time

	// Healthy indicates whether the peer is currently reachable.
	Healthy bool
}

// Federation manages cross-gateway tool discovery and proxying.
// It periodically polls peer gateways to discover their tools, registers
// proxy handlers in the local gateway's DynamicRegistry, and routes calls
// to the appropriate peer.
type Federation struct {
	config FederationConfig
	peers  map[string]*peerState
	mu     sync.RWMutex
	reg    *registry.DynamicRegistry
	cancel context.CancelFunc
	done   chan struct{}
	logger *slog.Logger
}

// peerState tracks the internal connection state for a single peer.
type peerState struct {
	endpoint string
	client   *client.Client
	tools    []ToolInfo
	lastSeen time.Time
	healthy  bool
	mu       sync.RWMutex
}

// snapshot returns a PeerGateway snapshot safe for external use.
func (ps *peerState) snapshot() PeerGateway {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	tools := make([]ToolInfo, len(ps.tools))
	copy(tools, ps.tools)
	return PeerGateway{
		Endpoint: ps.endpoint,
		Tools:    tools,
		LastSeen: ps.lastSeen,
		Healthy:  ps.healthy,
	}
}

// NewFederation creates a new Federation that will register discovered peer
// tools into the given DynamicRegistry. Call Start to begin the discovery loop.
func NewFederation(cfg FederationConfig, reg *registry.DynamicRegistry) *Federation {
	cfg.applyDefaults()
	return &Federation{
		config: cfg,
		peers:  make(map[string]*peerState),
		reg:    reg,
		done:   make(chan struct{}),
		logger: slog.Default(),
	}
}

// Start begins the background discovery loop that periodically polls all
// configured peers for their tool catalogs. It performs one immediate discovery
// pass before returning. The loop runs until the context is canceled or Stop
// is called.
func (f *Federation) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	// Perform initial discovery synchronously so callers know what tools
	// are available after Start returns.
	f.discoverAll(ctx)

	go f.loop(ctx)
	return nil
}

// Stop terminates the discovery loop and closes all peer connections.
func (f *Federation) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}

	// Wait for the loop goroutine to exit.
	<-f.done

	f.mu.Lock()
	defer f.mu.Unlock()

	var lastErr error
	for _, ps := range f.peers {
		ps.mu.Lock()
		if ps.client != nil {
			if err := ps.client.Close(); err != nil {
				lastErr = err
			}
			ps.client = nil
		}
		ps.mu.Unlock()
	}
	return lastErr
}

// AllTools returns the combined list of tools from all healthy peers.
func (f *Federation) AllTools() []ToolInfo {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var all []ToolInfo
	for _, ps := range f.peers {
		ps.mu.RLock()
		if ps.healthy {
			all = append(all, ps.tools...)
		}
		ps.mu.RUnlock()
	}
	return all
}

// RouteCall determines which peer gateway owns a given tool name and returns
// that peer. Returns an error if no peer owns the tool.
func (f *Federation) RouteCall(toolName string) (*PeerGateway, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, ps := range f.peers {
		ps.mu.RLock()
		for _, ti := range ps.tools {
			federatedName := f.federatedToolName(ps.endpoint, ti.Name)
			if federatedName == toolName {
				snap := ps.snapshot()
				ps.mu.RUnlock()
				return &snap, nil
			}
		}
		ps.mu.RUnlock()
	}
	return nil, fmt.Errorf("federation: no peer owns tool %q", toolName)
}

// Peers returns snapshots of all known peer gateways.
func (f *Federation) Peers() []PeerGateway {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]PeerGateway, 0, len(f.peers))
	for _, ps := range f.peers {
		result = append(result, ps.snapshot())
	}
	return result
}

// loop runs periodic discovery until the context is canceled.
func (f *Federation) loop(ctx context.Context) {
	defer close(f.done)

	ticker := time.NewTicker(f.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.discoverAll(ctx)
		}
	}
}

// discoverAll polls every configured peer for its tool catalog.
func (f *Federation) discoverAll(ctx context.Context) {
	for _, endpoint := range f.config.Peers {
		if ctx.Err() != nil {
			return
		}
		f.discoverPeer(ctx, endpoint)
	}
}

// discoverPeer connects to a single peer, fetches its tools, and updates
// the local registry.
func (f *Federation) discoverPeer(ctx context.Context, endpoint string) {
	ps := f.getOrCreatePeer(endpoint)

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Lazily establish the client connection.
	if ps.client == nil {
		c, err := f.connectPeer(ctx, endpoint)
		if err != nil {
			f.logger.Warn("federation: failed to connect to peer",
				"endpoint", endpoint, "error", err)
			ps.healthy = false
			return
		}
		ps.client = c
	}

	// Fetch tools from the peer.
	discoverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := ps.client.ListTools(discoverCtx, mcp.ListToolsRequest{})
	if err != nil {
		f.logger.Warn("federation: failed to list tools from peer",
			"endpoint", endpoint, "error", err)
		ps.healthy = false
		// Close the broken client so we reconnect next cycle.
		ps.client.Close()
		ps.client = nil
		return
	}

	// Build the new tool list.
	newTools := make([]ToolInfo, 0, len(result.Tools))
	for _, t := range result.Tools {
		newTools = append(newTools, ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			Peer:        endpoint,
		})
	}

	// Diff against existing tools and update the registry.
	oldNames := make(map[string]bool, len(ps.tools))
	for _, ti := range ps.tools {
		oldNames[f.federatedToolName(endpoint, ti.Name)] = true
	}

	newNames := make(map[string]bool, len(newTools))
	for _, ti := range newTools {
		fedName := f.federatedToolName(endpoint, ti.Name)
		newNames[fedName] = true
	}

	// Remove tools that are no longer advertised.
	for name := range oldNames {
		if !newNames[name] {
			f.reg.RemoveTool(name)
		}
	}

	// Add new tools or update existing ones.
	for _, ti := range newTools {
		fedName := f.federatedToolName(endpoint, ti.Name)
		if !oldNames[fedName] {
			// Find the original mcp.Tool for this tool info.
			var tool mcp.Tool
			for _, t := range result.Tools {
				if t.Name == ti.Name {
					tool = t
					break
				}
			}
			tool.Name = fedName
			f.reg.AddTool(registry.ToolDefinition{
				Tool:    tool,
				Handler: f.makePeerProxyHandler(endpoint, ti.Name),
			})
		}
	}

	ps.tools = newTools
	ps.lastSeen = time.Now()
	ps.healthy = true
}

// getOrCreatePeer returns the peerState for an endpoint, creating it if needed.
func (f *Federation) getOrCreatePeer(endpoint string) *peerState {
	f.mu.Lock()
	defer f.mu.Unlock()

	ps, ok := f.peers[endpoint]
	if !ok {
		ps = &peerState{endpoint: endpoint}
		f.peers[endpoint] = ps
	}
	return ps
}

// connectPeer establishes a new MCP client connection to a peer gateway.
func (f *Federation) connectPeer(ctx context.Context, endpoint string) (*client.Client, error) {
	tp, err := transport.NewStreamableHTTP(endpoint)
	if err != nil {
		return nil, fmt.Errorf("creating transport for %s: %w", endpoint, err)
	}

	c := client.NewClient(tp)
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("starting client for %s: %w", endpoint, err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "mcpkit-federation",
		Version: "1.0.0",
	}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		c.Close()
		return nil, fmt.Errorf("initializing connection to %s: %w", endpoint, err)
	}

	return c, nil
}

// makePeerProxyHandler creates a handler that forwards tool calls to a peer gateway.
func (f *Federation) makePeerProxyHandler(endpoint, originalToolName string) registry.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ps := f.getOrCreatePeer(endpoint)

		ps.mu.RLock()
		c := ps.client
		healthy := ps.healthy
		ps.mu.RUnlock()

		if c == nil || !healthy {
			return registry.MakeErrorResult(
				fmt.Sprintf("federation peer %q is unavailable", endpoint),
			), nil
		}

		forwardReq := mcp.CallToolRequest{}
		forwardReq.Params.Name = originalToolName
		forwardReq.Params.Arguments = request.Params.Arguments

		result, err := c.CallTool(ctx, forwardReq)
		if err != nil {
			return registry.MakeErrorResult(
				fmt.Sprintf("federation peer %q call failed: %v", endpoint, err),
			), nil
		}

		return result, nil
	}
}

// federatedToolName returns the name under which a peer's tool is registered
// in the local registry. When Namespace is true, the tool name is prefixed
// with a sanitized version of the peer endpoint.
func (f *Federation) federatedToolName(endpoint, toolName string) string {
	if !f.config.Namespace {
		return toolName
	}
	return peerNamespace(endpoint) + "/" + toolName
}

// peerNamespace converts a peer endpoint URL into a namespace prefix.
// e.g., "http://localhost:9090/mcp" -> "localhost_9090"
func peerNamespace(endpoint string) string {
	// Strip protocol prefix.
	ns := endpoint
	for _, prefix := range []string{"https://", "http://"} {
		ns = strings.TrimPrefix(ns, prefix)
	}
	// Strip path suffix.
	if idx := strings.Index(ns, "/"); idx >= 0 {
		ns = ns[:idx]
	}
	// Replace non-alphanumeric characters with underscores.
	ns = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, ns)
	return ns
}
