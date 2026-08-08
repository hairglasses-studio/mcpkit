// server_options.go — SDK-neutral option bundles for NewMCPServerWithOptions
// and NewStreamableHTTPHandler (P52.6/P52.7 canary unblocker: secretstudios-mcp's
// cmd/secretstudios-mcp/main.go is a frozen mcp-go holdout solely because it
// calls server.NewMCPServer/server.NewStreamableHTTPServer with mcp-go's
// server.ServerOption/server.StreamableHTTPOption functional options
// directly). Both structs use only primitive types so this file carries no
// build tag; NewMCPServerWithOptions and NewStreamableHTTPHandler each get a
// build-tagged implementation per SDK (server_compat.go / server_compat_official.go)
// that translates these fields onto that SDK's real option mechanism. Any
// field an SDK cannot honor is documented at the implementation, never
// silently dropped.
package registry

// ServerOptions is an SDK-neutral bundle of options for constructing an
// MCPServer via NewMCPServerWithOptions.
type ServerOptions struct {
	// Instructions is server-level guidance surfaced to clients (mcp-go:
	// server.WithInstructions; official SDK: mcp.ServerOptions.Instructions).
	Instructions string

	// ToolCapabilities, when true, advertises tools list-changed notification
	// support (mcp-go: server.WithToolCapabilities(true); official SDK: sets
	// ServerOptions.Capabilities.Tools = &mcp.ToolCapabilities{ListChanged: true},
	// overriding go-sdk's own default auto-inference from added tools).
	ToolCapabilities bool

	// ResourceCapabilities, when true, advertises the resources capability at
	// all (mcp-go: calls server.WithResourceCapabilities(ResourceSubscribe,
	// ResourceListChanged); official SDK: sets
	// Capabilities.Resources = &mcp.ResourceCapabilities{...} using the same
	// two sub-fields). When false, no resources capability is declared by
	// this call (a server that separately registers resources may still end
	// up advertising them via each SDK's own inference/registration path).
	ResourceCapabilities bool
	ResourceSubscribe    bool
	ResourceListChanged  bool

	// PromptCapabilities, when true, advertises prompts list-changed
	// notification support (mcp-go: server.WithPromptCapabilities(true);
	// official SDK: sets Capabilities.Prompts =
	// &mcp.PromptCapabilities{ListChanged: true}, overriding go-sdk's own
	// default auto-inference from added prompts).
	PromptCapabilities bool

	// StrictInputSchemas rejects tool calls whose arguments don't validate
	// against the declared input schema (mcp-go:
	// server.WithStrictInputSchemaDefault()). The official SDK has no
	// equivalent opt-in flag as of go-sdk v1.7.0 — this field is a documented
	// no-op there.
	StrictInputSchemas bool

	// Recovery wraps tool handlers so a panic becomes an error result
	// instead of crashing the server (mcp-go: server.WithRecovery()). The
	// official SDK has no equivalent flag as of go-sdk v1.7.0 — this field is
	// a documented no-op there.
	Recovery bool
}

// HTTPServerOptions is an SDK-neutral bundle of options for constructing a
// streamable-HTTP handler via NewStreamableHTTPHandler.
type HTTPServerOptions struct {
	// EndpointPath is the HTTP path the MCP endpoint is served on (e.g.
	// "/mcp"). Honored by mcp-go, which bakes the path into the transport's
	// own routing (server.WithEndpointPath). The official SDK's
	// StreamableHTTPHandler has no internal path concept — it is a plain
	// http.Handler the caller mounts at whatever path via their own mux — so
	// this field is a documented no-op there; mount NewStreamableHTTPHandler's
	// returned handler at EndpointPath yourself on that build.
	EndpointPath string

	// Stateless disables server-side session state when true (mcp-go:
	// server.WithStateLess(true); official SDK:
	// mcp.StreamableHTTPOptions.Stateless).
	Stateless bool
}
