//go:build official_sdk

// portable_resources_official.go is the official_sdk counterpart to
// portable_resources.go — same PortableResourcesModule shape, adapted to
// go-sdk's Resource/ResourceTemplate struct-literal construction (no
// mcp-go-style functional options) and its single ResourceContents struct
// (Blob []byte, base64-encoded by json.Marshal automatically, instead of
// mcp-go's separate TextResourceContents/BlobResourceContents with a
// pre-encoded base64 string field).
package conformance

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
			Resource: mcp.Resource{
				URI:         "test://static-text",
				Name:        "Static Text Resource",
				Description: "A static text resource for conformance testing",
				MIMEType:    "text/plain",
			},
			Handler: resources.TextResourceHandler(func(_ context.Context, _ string) (string, string, error) {
				return "text/plain", "This is a static text resource for conformance testing.", nil
			}),
			Category: "conformance",
		},
		{
			Resource: mcp.Resource{
				URI:         "test://static-binary",
				Name:        "Static Binary Resource",
				Description: "A static binary resource (base64 PNG) for conformance testing",
				MIMEType:    "image/png",
			},
			Handler: func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				blob, err := base64.StdEncoding.DecodeString(tinyImageBase64)
				if err != nil {
					return nil, err
				}
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{
							URI:      "test://static-binary",
							MIMEType: "image/png",
							Blob:     blob,
						},
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
			Template: mcp.ResourceTemplate{
				URITemplate: "test://dynamic/{name}",
				Name:        "Dynamic Resource",
				Description: "A dynamic text resource that echoes the URI parameter",
				MIMEType:    "text/plain",
			},
			Handler: resources.TextResourceHandler(func(_ context.Context, uri string) (string, string, error) {
				return "text/plain", fmt.Sprintf("Dynamic resource content for URI: %s", uri), nil
			}),
			Category: "conformance",
		},
		{
			Template: mcp.ResourceTemplate{
				URITemplate: "test://template/{id}/data",
				Name:        "Template Resource",
				Description: "A template resource that substitutes the id parameter",
				MIMEType:    "application/json",
			},
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
