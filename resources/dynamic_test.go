//go:build !official_sdk

package resources

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func resourceDef(uri, name string) ResourceDefinition {
	return ResourceDefinition{
		Resource: mcp.NewResource(uri, name),
		Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{URI: uri, Text: "hello"},
			}, nil
		},
	}
}

func templateDef(uriTemplate, name string) TemplateDefinition {
	return TemplateDefinition{
		Template: mcp.NewResourceTemplate(uriTemplate, name),
		Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{URI: uriTemplate, Text: "template-result"},
			}, nil
		},
	}
}

func TestDynamicRegistry_RegisterWithServer(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddResource(resourceDef("test://data", "test-data"))

	s := registry.NewMCPServer("test", "1.0")
	// Should not panic
	d.RegisterWithServer(s)

	if d.ResourceCount() != 1 {
		t.Errorf("expected 1 resource, got %d", d.ResourceCount())
	}
}

func TestDynamicRegistry_RegisterWithServer_ChangeFires(t *testing.T) {
	d := NewDynamicRegistry()

	s := registry.NewMCPServer("test", "1.0")
	d.RegisterWithServer(s)

	// Adding after RegisterWithServer should trigger the change notifier
	// which re-registers resources with the server (no panic expected).
	d.AddResource(resourceDef("test://new", "new-resource"))

	if d.ResourceCount() != 1 {
		t.Errorf("expected 1 resource after add, got %d", d.ResourceCount())
	}
}

func TestDynamicRegistry_NotifyOnAdd(t *testing.T) {
	d := NewDynamicRegistry()

	var count int32
	d.OnChange(func() {
		atomic.AddInt32(&count, 1)
	})

	d.AddResource(resourceDef("test://a", "A"))
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected 1 notification after add, got %d", atomic.LoadInt32(&count))
	}

	d.AddResource(resourceDef("test://b", "B"))
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("expected 2 notifications after second add, got %d", atomic.LoadInt32(&count))
	}
}

func TestDynamicRegistry_NotifyOnRemove(t *testing.T) {
	d := NewDynamicRegistry()

	var count int32
	d.OnChange(func() {
		atomic.AddInt32(&count, 1)
	})

	d.AddResource(resourceDef("test://a", "A"))
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected 1 notification after add, got %d", atomic.LoadInt32(&count))
	}

	ok := d.RemoveResource("test://a")
	if !ok {
		t.Error("expected RemoveResource to return true")
	}
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("expected 2 notifications after remove, got %d", atomic.LoadInt32(&count))
	}

	// Removing nonexistent should not notify
	ok = d.RemoveResource("test://nonexistent")
	if ok {
		t.Error("expected RemoveResource to return false for nonexistent")
	}
	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("should not notify on no-op remove, got %d", atomic.LoadInt32(&count))
	}
}

func TestDynamicRegistry_RegisterWithServer_Template(t *testing.T) {
	d := NewDynamicRegistry()
	d.AddTemplate(templateDef("test://{id}/data", "Test Template"))

	s := registry.NewMCPServer("test", "1.0")
	// Should not panic
	d.RegisterWithServer(s)

	if d.TemplateCount() != 1 {
		t.Errorf("expected 1 template, got %d", d.TemplateCount())
	}
}

func TestDynamicRegistry_RegisterWithServer_TemplateChangeFires(t *testing.T) {
	d := NewDynamicRegistry()

	s := registry.NewMCPServer("test", "1.0")
	d.RegisterWithServer(s)

	// Adding a template after RegisterWithServer triggers re-registration (no panic).
	d.AddTemplate(templateDef("test://{id}/profile", "Profile Template"))

	if d.TemplateCount() != 1 {
		t.Errorf("expected 1 template after add, got %d", d.TemplateCount())
	}
}

func TestDynamicRegistry_MultipleNotifiers(t *testing.T) {
	d := NewDynamicRegistry()

	var c1, c2 int32
	d.OnChange(func() { atomic.AddInt32(&c1, 1) })
	d.OnChange(func() { atomic.AddInt32(&c2, 1) })

	d.AddResource(resourceDef("test://x", "X"))

	if atomic.LoadInt32(&c1) != 1 || atomic.LoadInt32(&c2) != 1 {
		t.Errorf("expected both notifiers to fire: c1=%d c2=%d",
			atomic.LoadInt32(&c1), atomic.LoadInt32(&c2))
	}
}
