//go:build !official_sdk

package resources_test

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/resources"
)

// configModule is a minimal ResourceModule used in examples.
type configModule struct{}

func (m *configModule) Name() string        { return "config" }
func (m *configModule) Description() string { return "Config resource module" }
func (m *configModule) Resources() []resources.ResourceDefinition {
	return []resources.ResourceDefinition{
		{
			Resource: mcp.NewResource("file:///config.json", "Config"),
			Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{URI: "file:///config.json", Text: `{"version":"1.0"}`},
				}, nil
			},
			Category: "config",
		},
	}
}

func (m *configModule) Templates() []resources.TemplateDefinition {
	return nil
}

func ExampleNewResourceRegistry() {
	reg := resources.NewResourceRegistry()
	reg.RegisterModule(&configModule{})

	fmt.Println(reg.ResourceCount())
	fmt.Println(reg.ModuleCount())
	// Output:
	// 1
	// 1
}

func ExampleResourceRegistry_ListResources() {
	reg := resources.NewResourceRegistry()
	reg.RegisterModule(&configModule{})

	uris := reg.ListResources()
	for _, uri := range uris {
		fmt.Println(uri)
	}
	// Output:
	// file:///config.json
}

func ExampleResourceRegistry_GetResource() {
	reg := resources.NewResourceRegistry()
	reg.RegisterModule(&configModule{})

	rd, ok := reg.GetResource("file:///config.json")
	fmt.Println(ok)
	fmt.Println(rd.Category)
	// Output:
	// true
	// config
}
