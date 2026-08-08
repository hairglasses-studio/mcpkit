// Package conformance provides MCP conformance test servers for mcpkit.
//
// Two server constructors exist, deliberately split along a portability
// line drawn in the P52.6/P52.7 dual-SDK round:
//
//   - NewEverythingServer (everything_server.go, !official_sdk only) is the
//     full "everything-server" implementing every testable MCP capability —
//     tools, resources, prompts, logging, completions, sampling, and
//     elicitation — for the official MCP conformance suite
//     (https://github.com/modelcontextprotocol/conformance). Sampling,
//     elicitation, logging notifications, progress notifications, and
//     completions are driven through mcp-go's *server.MCPServer session
//     methods (RequestSampling, RequestElicitation, SendLogMessageToClient,
//     server.WithCompletions/WithPromptCompletionProvider/
//     WithResourceCompletionProvider) and mcp-go functional-option
//     capability negotiation, none of which the official go-sdk v1.7.0
//     exposes in a structurally equivalent shape (session-scoped APIs
//     differ; completions are a single ServerOptions.CompletionHandler
//     rather than mcp-go's separate prompt/resource completion-provider
//     interfaces). Porting that is out of scope for this round and stays
//     honestly !official_sdk-only rather than a stub reporting green with
//     no coverage.
//
//   - NewPortableEverythingServer (portable_server.go, both tags) covers the
//     tools/resources/prompts lifecycle subset that has zero MCPServer
//     session dependency and therefore ports cleanly: echo/add tools
//     (portable_tools.go), static/dynamic resources (portable_resources.go
//     / portable_resources_official.go), and all 8 conformance prompts
//     (portable_prompts.go / portable_prompts_official.go). Built on the
//     SDK-neutral registry.NewMCPServerWithOptions / AddToolToServer /
//     resources.ResourceRegistry / prompts.PromptRegistry surface. See
//     portable_server_test.go for the real initialize -> tools/resources/
//     prompts round trip verified under both build tags.
package conformance
