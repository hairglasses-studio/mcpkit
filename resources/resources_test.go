package resources

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// testModule implements ResourceModule for testing.
type testModule struct {
	name      string
	resources []ResourceDefinition
	templates []TemplateDefinition
}

func (m *testModule) Name() string                     { return m.name }
func (m *testModule) Description() string              { return "test module" }
func (m *testModule) Resources() []ResourceDefinition  { return m.resources }
func (m *testModule) Templates() []TemplateDefinition  { return m.templates }

func textHandler(text string) ResourceHandlerFunc {
	return func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{URI: "test://", Text: text},
		}, nil
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewResourceRegistry()

	mod := &testModule{
		name: "testmod",
		resources: []ResourceDefinition{
			{
				Resource: mcp.NewResource("file:///config.json", "Config", mcp.WithMIMEType("application/json")),
				Handler:  textHandler(`{"key":"value"}`),
				Category: "config",
			},
			{
				Resource: mcp.NewResource("file:///readme.md", "README"),
				Handler:  textHandler("# README"),
				Category: "docs",
			},
		},
		templates: []TemplateDefinition{
			{
				Template: mcp.NewResourceTemplate("user://{id}/profile", "User Profile"),
				Handler:  textHandler(`{"id":"1"}`),
				Category: "users",
			},
		},
	}

	r.RegisterModule(mod)

	if r.ResourceCount() != 2 {
		t.Fatalf("expected 2 resources, got %d", r.ResourceCount())
	}
	if r.TemplateCount() != 1 {
		t.Fatalf("expected 1 template, got %d", r.TemplateCount())
	}
	if r.ModuleCount() != 1 {
		t.Fatalf("expected 1 module, got %d", r.ModuleCount())
	}

	rd, ok := r.GetResource("file:///config.json")
	if !ok {
		t.Fatal("config.json not found")
	}
	if rd.Category != "config" {
		t.Errorf("category = %q, want config", rd.Category)
	}
	if rd.Resource.MIMEType != "application/json" {
		t.Errorf("mime = %q, want application/json", rd.Resource.MIMEType)
	}

	td, ok := r.GetTemplate("user://{id}/profile")
	if !ok {
		t.Fatal("user profile template not found")
	}
	if td.Category != "users" {
		t.Errorf("template category = %q, want users", td.Category)
	}

	m, ok := r.GetModule("testmod")
	if !ok {
		t.Fatal("testmod not found")
	}
	if m.Name() != "testmod" {
		t.Errorf("module name = %q, want testmod", m.Name())
	}
}

func TestRegistryListResources(t *testing.T) {
	r := NewResourceRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{Resource: mcp.NewResource("file:///b.txt", "B"), Handler: textHandler("b")},
			{Resource: mcp.NewResource("file:///a.txt", "A"), Handler: textHandler("a")},
		},
	})

	uris := r.ListResources()
	if len(uris) != 2 {
		t.Fatalf("expected 2, got %d", len(uris))
	}
	if uris[0] != "file:///a.txt" || uris[1] != "file:///b.txt" {
		t.Errorf("not sorted: %v", uris)
	}
}

func TestRegistryListByCategory(t *testing.T) {
	r := NewResourceRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{Resource: mcp.NewResource("file:///config.json", "Config"), Handler: textHandler(""), Category: "config"},
			{Resource: mcp.NewResource("file:///readme.md", "Readme"), Handler: textHandler(""), Category: "docs"},
			{Resource: mcp.NewResource("file:///schema.json", "Schema"), Handler: textHandler(""), Category: "config"},
		},
	})

	uris := r.ListResourcesByCategory("config")
	if len(uris) != 2 {
		t.Fatalf("expected 2 config resources, got %d", len(uris))
	}
}

func TestRegistryGetAllDefinitions(t *testing.T) {
	r := NewResourceRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{Resource: mcp.NewResource("file:///a", "A"), Handler: textHandler("")},
		},
		templates: []TemplateDefinition{
			{Template: mcp.NewResourceTemplate("db://{table}", "Table"), Handler: textHandler("")},
		},
	})

	allRes := r.GetAllResourceDefinitions()
	if len(allRes) != 1 {
		t.Fatalf("expected 1 resource def, got %d", len(allRes))
	}

	allTmpl := r.GetAllTemplateDefinitions()
	if len(allTmpl) != 1 {
		t.Fatalf("expected 1 template def, got %d", len(allTmpl))
	}
}

func TestRegistryHandlerExecution(t *testing.T) {
	r := NewResourceRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{
				Resource: mcp.NewResource("file:///data.txt", "Data"),
				Handler:  textHandler("hello world"),
			},
		},
	})

	rd, _ := r.GetResource("file:///data.txt")
	contents, err := rd.Handler(context.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	tc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("expected TextResourceContents")
	}
	if tc.Text != "hello world" {
		t.Errorf("text = %q, want hello world", tc.Text)
	}
}

func TestRegistryMiddleware(t *testing.T) {
	var order []string

	mw1 := func(uri string, rd ResourceDefinition, next ResourceHandlerFunc) ResourceHandlerFunc {
		return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			order = append(order, "mw1-before")
			result, err := next(ctx, req)
			order = append(order, "mw1-after")
			return result, err
		}
	}

	mw2 := func(uri string, rd ResourceDefinition, next ResourceHandlerFunc) ResourceHandlerFunc {
		return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			order = append(order, "mw2-before")
			result, err := next(ctx, req)
			order = append(order, "mw2-after")
			return result, err
		}
	}

	r := NewResourceRegistry(Config{
		Middleware: []Middleware{mw1, mw2},
	})
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{
				Resource: mcp.NewResource("file:///test", "Test"),
				Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					order = append(order, "handler")
					return []mcp.ResourceContents{mcp.TextResourceContents{Text: "ok"}}, nil
				},
			},
		},
	})

	// Execute through the wrapped handler (via RegisterWithServer path)
	// We test wrapHandler directly
	rd := r.resources["file:///test"]
	wrapped := r.wrapHandler("file:///test", rd)
	_, err := wrapped(context.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestRegistryPanicRecovery(t *testing.T) {
	r := NewResourceRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{
				Resource: mcp.NewResource("file:///panic", "Panic"),
				Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					panic("boom")
				},
			},
		},
	})

	rd := r.resources["file:///panic"]
	wrapped := r.wrapHandler("file:///panic", rd)
	result, err := wrapped(context.Background(), mcp.ReadResourceRequest{})
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if result != nil {
		t.Error("expected nil result from panic")
	}
}

func TestRegistryErrorHandler(t *testing.T) {
	r := NewResourceRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{
				Resource: mcp.NewResource("file:///fail", "Fail"),
				Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					return nil, fmt.Errorf("read failed")
				},
			},
		},
	})

	rd := r.resources["file:///fail"]
	wrapped := r.wrapHandler("file:///fail", rd)
	_, err := wrapped(context.Background(), mcp.ReadResourceRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "read failed" {
		t.Errorf("error = %q, want 'read failed'", err.Error())
	}
}

func TestSearchResources(t *testing.T) {
	r := NewResourceRegistry()
	r.RegisterModule(&testModule{
		name: "test",
		resources: []ResourceDefinition{
			{
				Resource: mcp.NewResource("file:///config.json", "App Config", mcp.WithResourceDescription("Application configuration")),
				Handler:  textHandler(""),
				Category: "config",
				Tags:     []string{"settings", "json"},
			},
			{
				Resource: mcp.NewResource("file:///readme.md", "README"),
				Handler:  textHandler(""),
				Category: "docs",
			},
			{
				Resource: mcp.NewResource("db://users/schema", "User Schema"),
				Handler:  textHandler(""),
				Category: "database",
				Tags:     []string{"schema"},
			},
		},
	})

	tests := []struct {
		query string
		want  int
	}{
		{"config", 1}, // matches URI "config.json" and category "config" — same resource
		{"schema", 1}, // matches name "User Schema" — tag "schema" is same resource
		{"json", 1},   // matches tag "json"
		{"docs", 1},   // matches category
		{"readme", 1}, // matches URI
		{"user", 1},   // matches name "User Schema"
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := r.SearchResources(tt.query)
			if len(results) != tt.want {
				t.Errorf("SearchResources(%q) = %d results, want %d", tt.query, len(results), tt.want)
			}
		})
	}
}

func TestDynamicRegistryAddRemove(t *testing.T) {
	d := NewDynamicRegistry()

	var notified int32
	d.OnChange(func() {
		atomic.AddInt32(&notified, 1)
	})

	rd := ResourceDefinition{
		Resource: mcp.NewResource("file:///dynamic.txt", "Dynamic"),
		Handler:  textHandler("dynamic"),
	}

	d.AddResource(rd)
	if d.ResourceCount() != 1 {
		t.Fatalf("expected 1 resource, got %d", d.ResourceCount())
	}
	if atomic.LoadInt32(&notified) != 1 {
		t.Error("expected notification on add")
	}

	ok := d.RemoveResource("file:///dynamic.txt")
	if !ok {
		t.Error("expected RemoveResource to return true")
	}
	if d.ResourceCount() != 0 {
		t.Fatalf("expected 0 resources, got %d", d.ResourceCount())
	}
	if atomic.LoadInt32(&notified) != 2 {
		t.Error("expected notification on remove")
	}

	ok = d.RemoveResource("file:///nonexistent")
	if ok {
		t.Error("expected RemoveResource to return false for nonexistent")
	}
	if atomic.LoadInt32(&notified) != 2 {
		t.Error("should not notify on no-op remove")
	}
}

func TestDynamicRegistryTemplates(t *testing.T) {
	d := NewDynamicRegistry()

	var notified int32
	d.OnChange(func() {
		atomic.AddInt32(&notified, 1)
	})

	td := TemplateDefinition{
		Template: mcp.NewResourceTemplate("user://{id}", "User"),
		Handler:  textHandler("user"),
	}

	d.AddTemplate(td)
	if d.TemplateCount() != 1 {
		t.Fatalf("expected 1 template, got %d", d.TemplateCount())
	}

	ok := d.RemoveTemplate("user://{id}")
	if !ok {
		t.Error("expected RemoveTemplate to return true")
	}
	if d.TemplateCount() != 0 {
		t.Fatalf("expected 0 templates, got %d", d.TemplateCount())
	}

	if atomic.LoadInt32(&notified) != 2 {
		t.Errorf("expected 2 notifications, got %d", atomic.LoadInt32(&notified))
	}
}

func TestRegistryNotFoundReturns(t *testing.T) {
	r := NewResourceRegistry()

	_, ok := r.GetResource("nonexistent://")
	if ok {
		t.Error("expected false for nonexistent resource")
	}

	_, ok = r.GetTemplate("nonexistent://{id}")
	if ok {
		t.Error("expected false for nonexistent template")
	}

	_, ok = r.GetModule("nonexistent")
	if ok {
		t.Error("expected false for nonexistent module")
	}
}

func TestEmptyRegistry(t *testing.T) {
	r := NewResourceRegistry()

	if r.ResourceCount() != 0 {
		t.Error("expected 0 resources")
	}
	if r.TemplateCount() != 0 {
		t.Error("expected 0 templates")
	}
	if r.ModuleCount() != 0 {
		t.Error("expected 0 modules")
	}
	if len(r.ListResources()) != 0 {
		t.Error("expected empty resource list")
	}
	if len(r.ListTemplates()) != 0 {
		t.Error("expected empty template list")
	}
	if len(r.SearchResources("anything")) != 0 {
		t.Error("expected no search results")
	}
}
