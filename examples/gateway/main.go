//go:build !official_sdk

// Command gateway demonstrates a production-grade mcpkit gateway server that
// aggregates tools from multiple upstream MCP servers into a single namespaced
// registry. It shows:
//   - NewGateway with a custom middleware stack
//   - DynamicUpstreamRegistry with a default resilience policy and lifecycle hooks
//   - Per-upstream circuit breaking, rate limiting, and call timeouts via UpstreamPolicy
//   - Adding upstreams from environment variables (MCP_UPSTREAM_1, MCP_UPSTREAM_2)
//   - Serving the aggregated tool surface via stdio
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/gateway"
	"github.com/hairglasses-studio/mcpkit/resilience"
)

func main() {
	ctx := context.Background()

	// --- Gateway + DynamicRegistry ---
	// NewGateway returns both the Gateway (upstream manager) and a
	// DynamicRegistry that is automatically populated as upstreams are added.
	gw, dynReg := gateway.NewGateway()

	// --- Default resilience policy ---
	// All upstreams added through the DynamicUpstreamRegistry will inherit
	// this policy unless they specify their own.
	defaultPolicy := gateway.UpstreamPolicy{
		CircuitBreaker: &resilience.CircuitBreakerConfig{
			// Open the circuit after 5 consecutive failures.
			FailureThreshold: 5,
			// Allow 1 probe call while half-open.
			HalfOpenMaxCalls: 1,
			// Keep the circuit open for 15 seconds before allowing a probe.
			Timeout: 15 * time.Second,
		},
		RateLimit: &resilience.RateLimitConfig{
			// Allow 50 requests per second with a burst of 10.
			Rate:  50,
			Burst: 10,
		},
		// Cut any individual proxied call after 10 seconds regardless of the
		// caller's own deadline.
		CallTimeout: 10 * time.Second,
	}

	// --- DynamicUpstreamRegistry ---
	// Wraps the gateway with default-policy injection and lifecycle callbacks.
	dynUp := gateway.NewDynamicUpstreamRegistry(gw,
		gateway.WithDefaultPolicy(defaultPolicy),
		gateway.WithDynamicHooks(gateway.DynamicHooks{
			OnAdd: func(name string, toolCount int) {
				log.Printf("upstream added: %s (%d tools)", name, toolCount)
			},
			OnRemove: func(name string) {
				log.Printf("upstream removed: %s", name)
			},
		}),
	)

	// --- Add upstreams from environment ---
	// Each upstream is optional; the gateway starts with zero upstreams if
	// none are configured and tools can be added at runtime.
	type upstreamEnv struct {
		name string
		envVar string
	}
	candidates := []upstreamEnv{
		{name: "upstream1", envVar: "MCP_UPSTREAM_1"},
		{name: "upstream2", envVar: "MCP_UPSTREAM_2"},
	}
	for _, c := range candidates {
		url := os.Getenv(c.envVar)
		if url == "" {
			log.Printf("skipping %s: %s not set", c.name, c.envVar)
			continue
		}
		count, err := dynUp.Add(ctx, gateway.UpstreamConfig{
			Name: c.name,
			URL:  url,
			// HealthInterval and UnhealthyThreshold get sensible defaults
			// (30s and 3 respectively) when left at zero.
		})
		if err != nil {
			log.Printf("failed to add upstream %s (%s): %v", c.name, url, err)
			continue
		}
		log.Printf("registered upstream %s with %d tools", c.name, count)
	}

	// --- Wire registry to MCP server ---
	// The DynamicRegistry returned by NewGateway exposes all proxied tools.
	// RegisterWithServer attaches it to the MCP server so that tools are
	// available under their namespaced names (e.g. "upstream1.list_files").
	s := server.NewMCPServer("gateway-example", "1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	dynReg.RegisterWithServer(s)

	log.Printf("gateway-example server starting on stdio (upstreams: %v)", dynUp.List())
	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
