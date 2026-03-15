// Package registry provides the core tool registry and interfaces for MCP servers.
//
// It manages tool registration, lookup, fuzzy search, deferred (lazy) loading,
// dynamic runtime registration, and middleware-based handler wrapping. Tool
// modules implement [ToolModule] and register via [ToolRegistry.RegisterModule];
// the middleware chain — including per-tool timeout, panic recovery, and output
// truncation — is applied automatically when tools are invoked.
//
// Key types: [ToolRegistry], [ToolDefinition], [ToolModule], [Middleware],
// [DynamicRegistry], [DeferredRegistry].
//
// Basic usage:
//
//	reg := registry.New(registry.Config{})
//	reg.RegisterTool(registry.ToolDefinition{
//	    Tool:    mcp.NewTool("greet", mcp.WithDescription("Say hello")),
//	    Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
//	        return registry.MakeTextResult("hello"), nil
//	    },
//	})
//	srv := registry.NewMCPServer("my-server", "1.0.0")
//	reg.RegisterWithServer(srv)
package registry
