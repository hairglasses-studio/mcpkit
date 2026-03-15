// Package gateway aggregates tools from multiple upstream MCP servers into a
// single namespaced registry.
//
// [Gateway] discovers tools from each upstream via its Streamable HTTP
// endpoint, prefixes their names with the upstream's configured namespace
// (e.g. "github__search_repos"), and exposes them through a
// [registry.DynamicRegistry] that can be mounted on any MCP server.
// Per-upstream resilience — circuit breaker, rate limiter, and call timeout —
// is configured via [UpstreamPolicy]. Runtime additions and removals are
// handled by [DynamicUpstreamRegistry] with optional lifecycle hooks.
//
// Example:
//
//	gw, reg := gateway.NewGateway()
//	gw.AddUpstream(ctx, gateway.UpstreamConfig{
//	    Name: "github",
//	    URL:  "https://github-mcp.example.com/mcp",
//	    Policy: gateway.UpstreamPolicy{
//	        CircuitBreaker: &resilience.CircuitBreakerConfig{FailureThreshold: 3},
//	    },
//	})
//	reg.RegisterWithServer(srv)
package gateway
