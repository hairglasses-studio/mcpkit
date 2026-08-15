package gateway

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// UpstreamConfig configures an upstream MCP server connection.
type UpstreamConfig struct {
	// Name is the namespace prefix for tools from this upstream (e.g. "github").
	Name string

	// URL is the streamable HTTP endpoint of the upstream server.
	URL string

	// AllowedTools limits the upstream surface to an explicit subset of tool
	// names. Empty means expose every discovered tool.
	AllowedTools []string

	// HealthInterval is how often to ping the upstream. Default: 30s.
	HealthInterval time.Duration

	// UnhealthyThreshold is how many consecutive ping failures before marking unhealthy. Default: 3.
	UnhealthyThreshold int

	// Policy configures per-upstream resilience (circuit breaker, rate limit, timeout).
	// Zero value means no resilience wrapping.
	Policy UpstreamPolicy
}

func (c *UpstreamConfig) applyDefaults() {
	if c.HealthInterval == 0 {
		c.HealthInterval = 30 * time.Second
	}
	if c.UnhealthyThreshold == 0 {
		c.UnhealthyThreshold = 3
	}
}

func (c UpstreamConfig) allowsTool(name string) bool {
	if len(c.AllowedTools) == 0 {
		return true
	}
	return slices.Contains(c.AllowedTools, name)
}

// UpstreamInfo provides status information about an upstream.
type UpstreamInfo struct {
	Name         string
	URL          string
	Healthy      bool
	ToolCount    int
	CircuitState string // empty when no circuit breaker is configured
}

// upstreamClient is the SDK-neutral request surface used by Gateway and
// Federation. The transport and handshake live in build-specific files.
type upstreamClient interface {
	listTools(context.Context) ([]registry.Tool, error)
	callTool(context.Context, string, map[string]any) (*registry.CallToolResult, error)
	ping(context.Context) error
	close() error
}

// upstream manages a connection to a single upstream MCP server.
type upstream struct {
	config     UpstreamConfig
	client     upstreamClient
	resilience *upstreamResilience

	mu    sync.RWMutex
	tools []registry.Tool

	healthy      atomic.Bool
	failCount    atomic.Int32
	cancelHealth context.CancelFunc
}

// connect establishes a client connection to the upstream server.
func (u *upstream) connect(ctx context.Context) error {
	c, err := newUpstreamClient(ctx, u.config.URL, "mcpkit-gateway", "")
	if err != nil {
		return err
	}
	u.client = c
	u.healthy.Store(true)
	return nil
}

// syncTools fetches the current tool list from the upstream.
func (u *upstream) syncTools(ctx context.Context) ([]registry.Tool, error) {
	tools, err := u.client.listTools(ctx)
	if err != nil {
		return nil, err
	}
	u.mu.Lock()
	filtered := make([]registry.Tool, 0, len(tools))
	for _, tool := range tools {
		if u.config.allowsTool(tool.Name) {
			filtered = append(filtered, tool)
		}
	}
	u.tools = filtered
	u.mu.Unlock()
	return filtered, nil
}

// startHealthLoop begins periodic health checking.
func (u *upstream) startHealthLoop(ctx context.Context, onHealthChange func(name string, healthy bool)) {
	ctx, cancel := context.WithCancel(ctx)
	u.cancelHealth = cancel

	go func() {
		ticker := time.NewTicker(u.config.HealthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
				err := u.client.ping(pingCtx)
				pingCancel()
				if err != nil {
					count := u.failCount.Add(1)
					if int(count) >= u.config.UnhealthyThreshold && u.healthy.Load() {
						u.healthy.Store(false)
						if onHealthChange != nil {
							onHealthChange(u.config.Name, false)
						}
					}
				} else {
					if !u.healthy.Load() {
						u.healthy.Store(true)
						u.failCount.Store(0)
						if onHealthChange != nil {
							onHealthChange(u.config.Name, true)
						}
					} else {
						u.failCount.Store(0)
					}
				}
			}
		}
	}()
}

// close shuts down the upstream connection and health loop.
func (u *upstream) close() error {
	if u.cancelHealth != nil {
		u.cancelHealth()
	}
	if u.client != nil {
		return u.client.close()
	}
	return nil
}

// namespacedName returns the namespaced tool name: "upstream.toolname"
func namespacedName(namespace, toolName string) string {
	return namespace + "." + toolName
}

// originalName strips the namespace prefix from a namespaced tool name.
func originalName(namespace, namespacedToolName string) string {
	prefix := namespace + "."
	return strings.TrimPrefix(namespacedToolName, prefix)
}
