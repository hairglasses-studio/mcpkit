// Package resources provides a registry for MCP resources and resource templates.
//
// It mirrors the registry package pattern: thread-safe registration via
// [ResourceRegistry], middleware chains applied to [ResourceHandlerFunc]
// handlers, module-based organization via [ResourceModule], and server
// integration via [ResourceRegistry.RegisterWithServer]. URI validation
// middleware ([URIValidationMiddleware]) defends against path traversal and
// SSRF attacks before handlers are invoked. Dynamic resource registration at
// runtime is supported by [DynamicResourceRegistry].
//
// Example:
//
//	reg := resources.New(resources.Config{})
//	reg.RegisterResource(resources.ResourceDefinition{
//	    Resource: mcp.Resource{URI: "file:///data/readme.md", Name: "readme"},
//	    Handler: func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
//	        return []mcp.ResourceContents{{URI: req.Params.URI, Text: "# Hello"}}, nil
//	    },
//	})
//	reg.RegisterWithServer(srv)
package resources
