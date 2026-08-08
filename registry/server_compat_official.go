//go:build official_sdk

// server_compat_official.go — MCP server integration (official go-sdk variant).
package registry

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer is the server type used for tool/resource/prompt registration.
type MCPServer = mcp.Server

// ResourceHandlerFunc is the server-level resource handler signature.
type ResourceHandlerFunc = func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)

// PromptHandlerFunc is the server-level prompt handler signature.
type PromptHandlerFunc = func(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// NewMCPServer creates a new MCP server instance.
func NewMCPServer(name, version string, opts ...any) *MCPServer {
	return mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Version: version,
	}, nil)
}

// AddToolToServer registers a tool with the MCP server.
// Adapts from mcpkit's value-receiver handler to official SDK's pointer-receiver handler.
func AddToolToServer(s *MCPServer, tool mcp.Tool, handler ToolHandlerFunc) {
	s.AddTool(&tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handler(ctx, *req)
	})
}

// AddResourceToServer registers a resource with the MCP server.
func AddResourceToServer(s *MCPServer, resource mcp.Resource, handler ResourceHandlerFunc) {
	s.AddResource(&resource, mcp.ResourceHandler(handler))
}

// AddResourceTemplateToServer registers a resource template with the MCP server.
func AddResourceTemplateToServer(s *MCPServer, tmpl mcp.ResourceTemplate, handler ResourceHandlerFunc) {
	s.AddResourceTemplate(&tmpl, mcp.ResourceHandler(handler))
}

// AddPromptToServer registers a prompt with the MCP server.
func AddPromptToServer(s *MCPServer, prompt mcp.Prompt, handler PromptHandlerFunc) {
	s.AddPrompt(&prompt, mcp.PromptHandler(handler))
}

// ServeStdio starts the MCP server on stdin/stdout.
func ServeStdio(s *MCPServer) error {
	ctx := context.Background()
	_, err := s.Connect(ctx, &mcp.StdioTransport{}, nil)
	if err != nil {
		return err
	}
	// Block until stdin closes
	<-ctx.Done()
	return ctx.Err()
}

// ServeAuto starts the MCP server on stdin/stdout. Unlike the mcp-go build,
// this does not support MCP_SOCKET_PATH-based Unix-socket pooling:
// transport.NewUnixSocketServer (transport/unixsocket.go) is itself
// unconditionally mcp-go-specific — it imports mark3labs/mcp-go's
// *server.MCPServer directly with no build tag and no official_sdk variant.
// Porting its per-connection JSON-RPC dispatch loop is a DynamicRegistry-
// class effort, out of scope here. If MCP_SOCKET_PATH is set, this logs a
// warning (rather than silently ignoring it) and falls back to stdio.
func ServeAuto(s *MCPServer) error {
	if socketPath := os.Getenv("MCP_SOCKET_PATH"); socketPath != "" {
		slog.Warn("MCP_SOCKET_PATH set but Unix-socket pooling is not supported on the official_sdk build; falling back to stdio",
			"socket_path", socketPath)
	}
	return ServeStdio(s)
}

// RemoveToolsFromServer removes tools from the MCP server by name.
func RemoveToolsFromServer(s *MCPServer, names ...string) {
	s.RemoveTools(names...)
}

// RemoveResourcesFromServer removes resources from the MCP server by URI.
func RemoveResourcesFromServer(s *MCPServer, uris ...string) {
	s.RemoveResources(uris...)
}

// RemovePromptsFromServer removes prompts from the MCP server by name.
func RemovePromptsFromServer(s *MCPServer, names ...string) {
	s.RemovePrompts(names...)
}

// NewResource constructs a Resource with the given URI and name. mcp-go's
// aliased NewResource (compat.go) also accepts variadic
// ...mcp.ResourceOption functional options (mcp.WithResourceDescription,
// mcp.WithMIMEType, ...) — an mcp-go-specific type with no official-SDK
// equivalent in this compat layer. No known consumer passes options today
// (every real call site is 2-arg, then sets .Description/.MIMEType directly
// on the returned value, which this signature supports identically since
// Resource's fields are exported on both builds); a variadic option
// parameter can be added later if a consumer needs one.
func NewResource(uri, name string) Resource {
	return mcp.Resource{URI: uri, Name: name}
}

// NewResourceTemplate constructs a ResourceTemplate with the given URI
// template and name. Same 2-arg-only scope as NewResource above — mcp-go's
// aliased NewResourceTemplate also accepts variadic
// ...mcp.ResourceTemplateOption functional options, which this compat layer
// does not mirror because no known consumer uses them.
func NewResourceTemplate(uriTemplate, name string) ResourceTemplate {
	return mcp.ResourceTemplate{URITemplate: uriTemplate, Name: name}
}

// NewMCPServerWithOptions creates an MCPServer from SDK-neutral ServerOptions
// (see server_options.go). Instructions and the Tool/Resource/Prompt
// capability flags are honored via mcp.ServerOptions.Capabilities (an
// explicit override of go-sdk's own default auto-inference from
// added tools/resources/prompts). StrictInputSchemas and Recovery have no
// go-sdk v1.7.0 equivalent and are documented no-ops here — see
// server_options.go's field-level doc comments.
func NewMCPServerWithOptions(name, version string, opts ServerOptions) *MCPServer {
	caps := &mcp.ServerCapabilities{
		Tools:   &mcp.ToolCapabilities{ListChanged: opts.ToolCapabilities},
		Prompts: &mcp.PromptCapabilities{ListChanged: opts.PromptCapabilities},
	}
	if opts.ResourceCapabilities {
		caps.Resources = &mcp.ResourceCapabilities{
			Subscribe:   opts.ResourceSubscribe,
			ListChanged: opts.ResourceListChanged,
		}
	}
	return mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: opts.Instructions,
		Capabilities: caps,
	})
}

// NewStreamableHTTPHandler wraps a single MCPServer in a streamable-HTTP
// http.Handler from SDK-neutral HTTPServerOptions (see server_options.go).
// go-sdk's mcp.NewStreamableHTTPHandler takes a per-request server-provider
// function rather than a fixed server; this always returns s, matching the
// single-server usage this compat pair targets (secretstudios-mcp's
// buildHTTPHandler). HTTPServerOptions.EndpointPath is a documented no-op
// here — go-sdk's handler has no internal path concept; mount the returned
// handler at that path via your own mux, same as the caller already does for
// the mcp-go build.
func NewStreamableHTTPHandler(s *MCPServer, opts HTTPServerOptions) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *MCPServer {
		return s
	}, &mcp.StreamableHTTPOptions{
		Stateless: opts.Stateless,
	})
}
