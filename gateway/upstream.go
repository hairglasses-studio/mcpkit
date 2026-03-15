//go:build !official_sdk

package gateway

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// UpstreamConfig configures an upstream MCP server connection.
type UpstreamConfig struct {
	// Name is the namespace prefix for tools from this upstream (e.g. "github").
	Name string

	// URL is the streamable HTTP endpoint of the upstream server.
	URL string

	// HealthInterval is how often to ping the upstream. Default: 30s.
	HealthInterval time.Duration

	// UnhealthyThreshold is how many consecutive ping failures before marking unhealthy. Default: 3.
	UnhealthyThreshold int
}

func (c *UpstreamConfig) applyDefaults() {
	if c.HealthInterval == 0 {
		c.HealthInterval = 30 * time.Second
	}
	if c.UnhealthyThreshold == 0 {
		c.UnhealthyThreshold = 3
	}
}

// UpstreamInfo provides status information about an upstream.
type UpstreamInfo struct {
	Name      string
	URL       string
	Healthy   bool
	ToolCount int
}

// upstream manages a connection to a single upstream MCP server.
type upstream struct {
	config UpstreamConfig
	client *client.Client

	mu    sync.RWMutex
	tools []mcp.Tool

	healthy      atomic.Bool
	failCount    atomic.Int32
	cancelHealth context.CancelFunc
}

// connect establishes a client connection to the upstream server.
func (u *upstream) connect(ctx context.Context) error {
	tp, err := transport.NewStreamableHTTP(u.config.URL)
	if err != nil {
		return err
	}
	c := client.NewClient(tp)
	if err := c.Start(ctx); err != nil {
		return err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "mcpkit-gateway",
		Version: "1.0.0",
	}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		c.Close()
		return err
	}

	u.client = c
	u.healthy.Store(true)
	return nil
}

// syncTools fetches the current tool list from the upstream.
func (u *upstream) syncTools(ctx context.Context) ([]mcp.Tool, error) {
	result, err := u.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	u.mu.Lock()
	u.tools = result.Tools
	u.mu.Unlock()
	return result.Tools, nil
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
				err := u.client.Ping(pingCtx)
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
		return u.client.Close()
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
