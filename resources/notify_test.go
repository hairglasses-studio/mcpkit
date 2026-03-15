package resources

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestWireResourceListChanged_Add(t *testing.T) {
	d := NewDynamicRegistry()
	s := registry.NewMCPServer("test", "0.0.0")

	d.AddResource(ResourceDefinition{
		Resource: mcp.Resource{URI: "test://initial", Name: "initial"},
		Handler:  func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) { return nil, nil },
	})

	d.RegisterWithServer(s)

	d.AddResource(ResourceDefinition{
		Resource: mcp.Resource{URI: "test://added", Name: "added"},
		Handler:  func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) { return nil, nil },
	})

	uris := d.ListResources()
	if len(uris) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(uris))
	}
}

func TestWireResourceListChanged_Remove(t *testing.T) {
	d := NewDynamicRegistry()
	s := registry.NewMCPServer("test", "0.0.0")

	d.AddResource(ResourceDefinition{
		Resource: mcp.Resource{URI: "test://keep", Name: "keep"},
		Handler:  func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) { return nil, nil },
	})
	d.AddResource(ResourceDefinition{
		Resource: mcp.Resource{URI: "test://remove", Name: "remove"},
		Handler:  func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) { return nil, nil },
	})

	d.RegisterWithServer(s)

	ok := d.RemoveResource("test://remove")
	if !ok {
		t.Fatal("expected RemoveResource to return true")
	}

	uris := d.ListResources()
	if len(uris) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(uris))
	}
	if uris[0] != "test://keep" {
		t.Fatalf("expected 'test://keep', got %q", uris[0])
	}
}

func TestWireResourceListChanged_AddAndRemove(t *testing.T) {
	d := NewDynamicRegistry()
	s := registry.NewMCPServer("test", "0.0.0")

	d.AddResource(ResourceDefinition{
		Resource: mcp.Resource{URI: "test://a", Name: "a"},
		Handler:  func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) { return nil, nil },
	})
	d.AddResource(ResourceDefinition{
		Resource: mcp.Resource{URI: "test://b", Name: "b"},
		Handler:  func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) { return nil, nil },
	})

	d.RegisterWithServer(s)

	d.RemoveResource("test://b")
	d.AddResource(ResourceDefinition{
		Resource: mcp.Resource{URI: "test://c", Name: "c"},
		Handler:  func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) { return nil, nil },
	})

	uris := d.ListResources()
	if len(uris) != 2 {
		t.Fatalf("expected 2 resources, got %d: %v", len(uris), uris)
	}
	expected := map[string]bool{"test://a": true, "test://c": true}
	for _, uri := range uris {
		if !expected[uri] {
			t.Errorf("unexpected resource %q", uri)
		}
	}
}
