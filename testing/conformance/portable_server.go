// portable_server.go — NewPortableEverythingServer builds the reduced
// conformance server covering only the tools/resources/prompts lifecycle:
// the subset that ports cleanly to both mcp-go and the official SDK (see
// doc.go for the full split rationale — sampling, elicitation, logging
// notifications, progress notifications, and completions stay mcp-go-only,
// served instead by NewEverythingServer in everything_server.go). Built
// entirely on the SDK-neutral registry.NewMCPServerWithOptions /
// registry.NewToolRegistry / resources.NewResourceRegistry /
// prompts.NewPromptRegistry surface (registry/server_compat*.go,
// items 3-4 of the P52.6/P52.7 canary-unblocker round), so this
// constructor's own source is identical on both build tags and carries no
// build tag itself — only PortableToolsModule / PortableResourcesModule /
// PortablePromptsModule underneath it are build-tagged per SDK.
package conformance

import (
	mcpprompts "github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

// NewPortableEverythingServer creates an MCP server implementing the
// portable subset of conformance capabilities: tools (echo, add),
// resources (static text/binary, dynamic templates), and prompts (simple,
// complex, embedded resource, image).
func NewPortableEverythingServer(cfg ServerConfig) *registry.MCPServer {
	s := registry.NewMCPServerWithOptions(cfg.Name, cfg.Version, registry.ServerOptions{
		Instructions:         "mcpkit portable conformance server (tools/resources/prompts lifecycle only)",
		ToolCapabilities:     true,
		ResourceCapabilities: true,
		PromptCapabilities:   true,
		Recovery:             true,
	})

	toolReg := registry.NewToolRegistry()
	toolReg.RegisterModule(&PortableToolsModule{})
	toolReg.RegisterWithServer(s)

	resReg := resources.NewResourceRegistry()
	resReg.RegisterModule(&PortableResourcesModule{})
	resReg.RegisterWithServer(s)

	promptReg := mcpprompts.NewPromptRegistry()
	promptReg.RegisterModule(&PortablePromptsModule{})
	promptReg.RegisterWithServer(s)

	return s
}
