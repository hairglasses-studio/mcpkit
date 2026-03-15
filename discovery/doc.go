// Package discovery provides MCP Registry integration for server discovery,
// publishing, metadata extraction, and server card serving.
//
// [Client] queries the MCP Registry API for [ServerMetadata], caches results
// with a configurable TTL, and maps HTTP status codes to typed sentinel
// errors. [Publisher] registers or updates a server's metadata in the
// registry using an API token. [MetadataFromConfig] introspects a local
// [registry.ToolRegistry] (and optionally resources/prompts registries) to
// build a [ServerMetadata] value without a live registry round-trip.
// [ServerCardHandler] and [StaticServerCardHandler] serve the
// .well-known/mcp.json server card over HTTP.
//
// Example:
//
//	c := discovery.NewClient("https://registry.modelcontextprotocol.io")
//	meta, err := c.Get(ctx, "my-org/my-server")
//
//	mux.Handle("/.well-known/mcp.json", discovery.ServerCardHandler(discovery.MetadataConfig{
//	    Name: "my-server", Version: "1.2.0", Registry: reg,
//	}))
package discovery
