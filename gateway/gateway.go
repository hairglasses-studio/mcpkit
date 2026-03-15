//go:build !official_sdk

package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Config configures the gateway.
type Config struct {
	// Middleware to apply to all proxied tool handlers.
	Middleware []registry.Middleware
}

// Gateway aggregates tools from multiple upstream MCP servers into a single
// DynamicRegistry with namespaced tool names.
type Gateway struct {
	mu        sync.RWMutex
	upstreams map[string]*upstream
	toolMap   map[string]string // namespaced tool name → upstream name
	reg       *registry.DynamicRegistry
	config    Config
	closed    bool
}

// NewGateway creates a new gateway and its associated DynamicRegistry.
// The returned registry can be registered with an MCP server via RegisterWithServer.
func NewGateway(config ...Config) (*Gateway, *registry.DynamicRegistry) {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}

	reg := registry.NewDynamicRegistry(registry.Config{
		Middleware: cfg.Middleware,
	})

	g := &Gateway{
		upstreams: make(map[string]*upstream),
		toolMap:   make(map[string]string),
		reg:       reg,
		config:    cfg,
	}

	return g, reg
}

// AddUpstream connects to an upstream MCP server, discovers its tools,
// and registers them with namespaced names in the gateway's registry.
// Returns the number of tools discovered.
func (g *Gateway) AddUpstream(ctx context.Context, config UpstreamConfig) (int, error) {
	config.applyDefaults()

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return 0, ErrGatewayClosed
	}
	if _, exists := g.upstreams[config.Name]; exists {
		g.mu.Unlock()
		return 0, fmt.Errorf("%w: %s", ErrDuplicateUpstream, config.Name)
	}
	g.mu.Unlock()

	u := &upstream{
		config:     config,
		resilience: newUpstreamResilience(config.Name, config.Policy),
	}
	if err := u.connect(ctx); err != nil {
		return 0, fmt.Errorf("connecting to upstream %s: %w", config.Name, err)
	}

	tools, err := u.syncTools(ctx)
	if err != nil {
		u.close()
		return 0, fmt.Errorf("syncing tools from upstream %s: %w", config.Name, err)
	}

	g.mu.Lock()
	// Double-check after reconnection
	if _, exists := g.upstreams[config.Name]; exists {
		g.mu.Unlock()
		u.close()
		return 0, fmt.Errorf("%w: %s", ErrDuplicateUpstream, config.Name)
	}
	g.upstreams[config.Name] = u

	// Register tools with namespaced names
	for _, tool := range tools {
		nsName := namespacedName(config.Name, tool.Name)
		g.toolMap[nsName] = config.Name

		nsTool := tool
		nsTool.Name = nsName

		handler := g.makeProxyHandler(config.Name, tool.Name)
		if u.resilience != nil {
			handler = u.resilience.wrapHandler(config.Name, handler)
		}
		g.reg.AddTool(registry.ToolDefinition{
			Tool:    nsTool,
			Handler: handler,
		})
	}
	g.mu.Unlock()

	// Start health loop
	u.startHealthLoop(ctx, nil)

	return len(tools), nil
}

// RemoveUpstream disconnects an upstream and removes all its tools from the registry.
// Returns true if the upstream existed.
func (g *Gateway) RemoveUpstream(name string) bool {
	g.mu.Lock()
	u, exists := g.upstreams[name]
	if !exists {
		g.mu.Unlock()
		return false
	}

	// Find and remove all tools for this upstream
	var toRemove []string
	for nsName, upName := range g.toolMap {
		if upName == name {
			toRemove = append(toRemove, nsName)
		}
	}
	for _, nsName := range toRemove {
		delete(g.toolMap, nsName)
		g.reg.RemoveTool(nsName)
	}

	delete(g.upstreams, name)
	g.mu.Unlock()

	u.close()
	return true
}

// ListUpstreams returns the names of all registered upstreams.
func (g *Gateway) ListUpstreams() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	names := make([]string, 0, len(g.upstreams))
	for name := range g.upstreams {
		names = append(names, name)
	}
	return names
}

// UpstreamStatus returns status information for a named upstream.
func (g *Gateway) UpstreamStatus(name string) (UpstreamInfo, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	u, exists := g.upstreams[name]
	if !exists {
		return UpstreamInfo{}, fmt.Errorf("%w: %s", ErrUpstreamNotFound, name)
	}

	u.mu.RLock()
	toolCount := len(u.tools)
	u.mu.RUnlock()

	return UpstreamInfo{
		Name:         name,
		URL:          u.config.URL,
		Healthy:      u.healthy.Load(),
		ToolCount:    toolCount,
		CircuitState: u.resilience.circuitState(),
	}, nil
}

// Close shuts down all upstream connections.
func (g *Gateway) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return ErrGatewayClosed
	}
	g.closed = true

	var lastErr error
	for name, u := range g.upstreams {
		if err := u.close(); err != nil {
			lastErr = fmt.Errorf("closing upstream %s: %w", name, err)
		}
	}
	return lastErr
}

// makeProxyHandler creates a tool handler that forwards calls to the upstream server.
func (g *Gateway) makeProxyHandler(upstreamName, originalToolName string) registry.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		g.mu.RLock()
		u, exists := g.upstreams[upstreamName]
		g.mu.RUnlock()

		if !exists {
			return registry.MakeErrorResult(fmt.Sprintf("upstream %q not found", upstreamName)), nil
		}

		if !u.healthy.Load() {
			return registry.MakeErrorResult(fmt.Sprintf("upstream %q is unhealthy", upstreamName)), nil
		}

		// Forward the call with the original (non-namespaced) tool name
		forwardReq := mcp.CallToolRequest{}
		forwardReq.Params.Name = originalToolName
		forwardReq.Params.Arguments = request.Params.Arguments

		result, err := u.client.CallTool(ctx, forwardReq)
		if err != nil {
			return registry.MakeErrorResult(fmt.Sprintf("upstream %q call failed: %v", upstreamName, err)), nil
		}

		return result, nil
	}
}
