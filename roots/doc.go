// Package roots provides client workspace root discovery for MCP servers.
// It exposes a ServerRootsClient for listing roots declared by the connected
// MCP client, a CachedClient that memoizes root lookups, and context helpers
// for propagating root information through the request chain. A middleware
// variant is available for automatic root injection.
package roots
