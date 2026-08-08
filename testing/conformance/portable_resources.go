//go:build !official_sdk

// portable_resources.go — the conformance resources module used by both
// NewEverythingServer's ResourcesModule (unchanged, mcp-go-only,
// everything_server.go) and NewPortableEverythingServer's
// PortableResourcesModule (this file's !official_sdk half plus
// portable_resources_official.go's official_sdk half). Static-text,
// static-binary, and both dynamic/template resources have zero MCPServer
// dependency, so unlike ToolsModule's sampling/elicitation/logging/progress
// tools, the full resource set ports cleanly — the only per-tag work is
// each SDK's own Resource/ResourceTemplate/content-type construction idiom.
package conformance

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/resources"
)

// PortableResourcesModule implements the conformance resources that have no
// MCPServer dependency: static-text, static-binary, and two dynamic
// templates.
type PortableResourcesModule struct{}

// Name returns the module name.
func (m *PortableResourcesModule) Name() string { return "conformance-portable-resources" }

// Description returns the module description.
func (m *PortableResourcesModule) Description() string {
	return "MCP conformance suite resources (portable subset): static text, static binary, dynamic template"
}

// Resources returns the portable conformance resource definitions.
func (m *PortableResourcesModule) Resources() []resources.ResourceDefinition {
	return []resources.ResourceDefinition{
		{
			Resource: mcp.NewResource(
				"test://static-text",
				"Static Text Resource",
				mcp.WithResourceDescription("A static text resource for conformance testing"),
				mcp.WithMIMEType("text/plain"),
			),
			Handler: resources.TextResourceHandler(func(_ context.Context, _ string) (string, string, error) {
				return "text/plain", "This is a static text resource for conformance testing.", nil
			}),
			Category: "conformance",
		},
		{
			Resource: mcp.NewResource(
				"test://static-binary",
				"Static Binary Resource",
				mcp.WithResourceDescription("A static binary resource (base64 PNG) for conformance testing"),
				mcp.WithMIMEType("image/png"),
			),
			Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.BlobResourceContents{
						URI:      "test://static-binary",
						MIMEType: "image/png",
						Blob:     tinyImageBase64,
					},
				}, nil
			},
			Category: "conformance",
		},
	}
}

// Templates returns the portable conformance resource template definitions.
func (m *PortableResourcesModule) Templates() []resources.TemplateDefinition {
	return []resources.TemplateDefinition{
		{
			Template: mcp.NewResourceTemplate(
				"test://dynamic/{name}",
				"Dynamic Resource",
				mcp.WithTemplateDescription("A dynamic text resource that echoes the URI parameter"),
				mcp.WithTemplateMIMEType("text/plain"),
			),
			Handler: resources.TextResourceHandler(func(_ context.Context, uri string) (string, string, error) {
				return "text/plain", fmt.Sprintf("Dynamic resource content for URI: %s", uri), nil
			}),
			Category: "conformance",
		},
		{
			Template: mcp.NewResourceTemplate(
				"test://template/{id}/data",
				"Template Resource",
				mcp.WithTemplateDescription("A template resource that substitutes the id parameter"),
				mcp.WithTemplateMIMEType("application/json"),
			),
			Handler: resources.JSONResourceHandler(func(_ context.Context, uri string) (any, error) {
				id := extractTemplateID(uri)
				return map[string]any{
					"id":           id,
					"templateTest": true,
					"data":         fmt.Sprintf("Data for ID: %s", id),
				}, nil
			}),
			Category: "conformance",
		},
	}
}
