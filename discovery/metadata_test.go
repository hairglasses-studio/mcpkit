//go:build !official_sdk

package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

// --- helpers for MetadataFromConfig tests ---

// buildToolRegistry returns a ToolRegistry populated with named tools.
func buildToolRegistry(toolDefs ...registry.ToolDefinition) *registry.ToolRegistry {
	type simpleModule struct {
		tools []registry.ToolDefinition
	}
	type mod struct {
		tools []registry.ToolDefinition
	}

	// Use the same testModule helper already defined in discovery_test.go.
	m := &testModule{tools: toolDefs}
	reg := registry.NewToolRegistry()
	reg.RegisterModule(m)
	return reg
}

// resourceModule is a minimal ResourceModule for testing.
type resourceModule struct {
	name      string
	resources []resources.ResourceDefinition
	templates []resources.TemplateDefinition
}

func (m *resourceModule) Name() string                          { return m.name }
func (m *resourceModule) Description() string                  { return "test resource module" }
func (m *resourceModule) Resources() []resources.ResourceDefinition { return m.resources }
func (m *resourceModule) Templates() []resources.TemplateDefinition { return m.templates }

// promptModule is a minimal PromptModule for testing.
type promptModule struct {
	name    string
	prompts []prompts.PromptDefinition
}

func (m *promptModule) Name() string                      { return m.name }
func (m *promptModule) Description() string               { return "test prompt module" }
func (m *promptModule) Prompts() []prompts.PromptDefinition { return m.prompts }

// --- MetadataFromConfig tests ---

func TestMetadataFromConfig_AllRegistries(t *testing.T) {
	t.Parallel()

	toolReg := buildToolRegistry(
		registry.ToolDefinition{Tool: registry.Tool{Name: "alpha", Description: "Alpha tool"}},
		registry.ToolDefinition{Tool: registry.Tool{Name: "beta", Description: "Beta tool"}},
	)

	resReg := resources.NewResourceRegistry()
	resReg.RegisterModule(&resourceModule{
		name: "res-mod",
		resources: []resources.ResourceDefinition{
			{Resource: mcp.NewResource("file:///config.json", "Config")},
			{Resource: mcp.NewResource("file:///data.json", "Data")},
		},
		templates: []resources.TemplateDefinition{
			{Template: mcp.NewResourceTemplate("user://{id}/profile", "User Profile")},
		},
	})

	promptReg := prompts.NewPromptRegistry()
	promptReg.RegisterModule(&promptModule{
		name: "prompt-mod",
		prompts: []prompts.PromptDefinition{
			{Prompt: mcp.NewPrompt("summarize", mcp.WithPromptDescription("Summarize text"))},
			{Prompt: mcp.NewPrompt("review", mcp.WithPromptDescription("Review code"))},
		},
	})

	meta := MetadataFromConfig(MetadataConfig{
		Name:        "full-server",
		Description: "A fully configured server",
		Version:     "2.0.0",
		Tags:        []string{"test", "full"},
		Tools:       toolReg,
		Resources:   resReg,
		Prompts:     promptReg,
		Transports:  []TransportInfo{{Type: "streamable-http", URL: "https://example.com/mcp"}},
	})

	if meta.Name != "full-server" {
		t.Errorf("Name = %q, want %q", meta.Name, "full-server")
	}
	if meta.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", meta.Version, "2.0.0")
	}
	if len(meta.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(meta.Tags))
	}

	// Tools: 2 tools, sorted by name.
	if len(meta.Tools) != 2 {
		t.Fatalf("len(Tools) = %d, want 2", len(meta.Tools))
	}
	if meta.Tools[0].Name != "alpha" {
		t.Errorf("Tools[0].Name = %q, want alpha", meta.Tools[0].Name)
	}
	if meta.Tools[1].Name != "beta" {
		t.Errorf("Tools[1].Name = %q, want beta", meta.Tools[1].Name)
	}

	// Resources: 2 static + 1 template = 3 total.
	if len(meta.Resources) != 3 {
		t.Fatalf("len(Resources) = %d, want 3", len(meta.Resources))
	}
	// Static resources sorted, then template appended after.
	// Static entries come first (sorted by URI): file:///config.json, file:///data.json.
	if meta.Resources[0].URITemplate != "file:///config.json" {
		t.Errorf("Resources[0].URITemplate = %q, want file:///config.json", meta.Resources[0].URITemplate)
	}
	if meta.Resources[1].URITemplate != "file:///data.json" {
		t.Errorf("Resources[1].URITemplate = %q, want file:///data.json", meta.Resources[1].URITemplate)
	}
	if meta.Resources[2].URITemplate != "user://{id}/profile" {
		t.Errorf("Resources[2].URITemplate = %q, want user://{id}/profile", meta.Resources[2].URITemplate)
	}

	// Prompts: 2, sorted by name.
	if len(meta.Prompts) != 2 {
		t.Fatalf("len(Prompts) = %d, want 2", len(meta.Prompts))
	}
	if meta.Prompts[0].Name != "review" {
		t.Errorf("Prompts[0].Name = %q, want review", meta.Prompts[0].Name)
	}
	if meta.Prompts[1].Name != "summarize" {
		t.Errorf("Prompts[1].Name = %q, want summarize", meta.Prompts[1].Name)
	}

	// Transports.
	if len(meta.Transports) != 1 || meta.Transports[0].Type != "streamable-http" {
		t.Errorf("unexpected transports: %v", meta.Transports)
	}
}

func TestMetadataFromConfig_NilRegistries(t *testing.T) {
	t.Parallel()

	// All three registries nil — should not panic and produce empty slices.
	meta := MetadataFromConfig(MetadataConfig{
		Name:        "minimal-server",
		Description: "Minimal",
		Version:     "1.0.0",
		Tools:       nil,
		Resources:   nil,
		Prompts:     nil,
	})

	if meta.Name != "minimal-server" {
		t.Errorf("Name = %q, want %q", meta.Name, "minimal-server")
	}
	if meta.Tools != nil {
		t.Errorf("Tools should be nil when no Tools registry provided, got %v", meta.Tools)
	}
	if meta.Resources != nil {
		t.Errorf("Resources should be nil when no Resources registry provided, got %v", meta.Resources)
	}
	if meta.Prompts != nil {
		t.Errorf("Prompts should be nil when no Prompts registry provided, got %v", meta.Prompts)
	}
}

func TestMetadataFromConfig_OnlyTools(t *testing.T) {
	t.Parallel()

	toolReg := buildToolRegistry(
		registry.ToolDefinition{Tool: registry.Tool{Name: "search", Description: "Search tool"}},
	)

	meta := MetadataFromConfig(MetadataConfig{
		Name:        "tools-only",
		Description: "Tools only server",
		Tools:       toolReg,
	})

	if len(meta.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(meta.Tools))
	}
	if meta.Tools[0].Name != "search" {
		t.Errorf("Tools[0].Name = %q, want search", meta.Tools[0].Name)
	}
	if meta.Tools[0].Description != "Search tool" {
		t.Errorf("Tools[0].Description = %q, want 'Search tool'", meta.Tools[0].Description)
	}

	// Resources and Prompts should be nil.
	if meta.Resources != nil {
		t.Errorf("expected nil Resources, got %v", meta.Resources)
	}
	if meta.Prompts != nil {
		t.Errorf("expected nil Prompts, got %v", meta.Prompts)
	}
}

func TestMetadataFromConfig_ResourceExtraction(t *testing.T) {
	t.Parallel()

	resReg := resources.NewResourceRegistry()
	resReg.RegisterModule(&resourceModule{
		name: "docs",
		resources: []resources.ResourceDefinition{
			{
				Resource: mcp.Resource{
					URI:         "docs://changelog",
					Name:        "Changelog",
					Description: "Project changelog",
				},
			},
		},
	})

	meta := MetadataFromConfig(MetadataConfig{
		Name:      "docs-server",
		Resources: resReg,
	})

	if len(meta.Resources) != 1 {
		t.Fatalf("len(Resources) = %d, want 1", len(meta.Resources))
	}
	r := meta.Resources[0]
	if r.URITemplate != "docs://changelog" {
		t.Errorf("URITemplate = %q, want docs://changelog", r.URITemplate)
	}
	if r.Name != "Changelog" {
		t.Errorf("Name = %q, want Changelog", r.Name)
	}
	if r.Description != "Project changelog" {
		t.Errorf("Description = %q, want 'Project changelog'", r.Description)
	}
}

func TestMetadataFromConfig_PromptExtraction(t *testing.T) {
	t.Parallel()

	promptReg := prompts.NewPromptRegistry()
	promptReg.RegisterModule(&promptModule{
		name: "assistants",
		prompts: []prompts.PromptDefinition{
			{
				Prompt: mcp.NewPrompt("explain",
					mcp.WithPromptDescription("Explain a concept"),
				),
			},
		},
	})

	meta := MetadataFromConfig(MetadataConfig{
		Name:    "prompt-server",
		Prompts: promptReg,
	})

	if len(meta.Prompts) != 1 {
		t.Fatalf("len(Prompts) = %d, want 1", len(meta.Prompts))
	}
	p := meta.Prompts[0]
	if p.Name != "explain" {
		t.Errorf("Name = %q, want explain", p.Name)
	}
	if p.Description != "Explain a concept" {
		t.Errorf("Description = %q, want 'Explain a concept'", p.Description)
	}
}

func TestMetadataFromConfig_AuthAndOrgFields(t *testing.T) {
	t.Parallel()

	auth := &AuthRequirement{
		Type:     "oauth2",
		TokenURL: "https://auth.example.com/token",
		Scopes:   []string{"read", "write"},
	}

	meta := MetadataFromConfig(MetadataConfig{
		Name:         "enterprise-server",
		Organization: "Acme Corp",
		Repository:   "https://github.com/acme/server",
		Auth:         auth,
	})

	if meta.Organization != "Acme Corp" {
		t.Errorf("Organization = %q, want 'Acme Corp'", meta.Organization)
	}
	if meta.Repository != "https://github.com/acme/server" {
		t.Errorf("Repository = %q, want 'https://github.com/acme/server'", meta.Repository)
	}
	if meta.Auth == nil {
		t.Fatal("Auth should not be nil")
	}
	if meta.Auth.Type != "oauth2" {
		t.Errorf("Auth.Type = %q, want oauth2", meta.Auth.Type)
	}
	if len(meta.Auth.Scopes) != 2 {
		t.Errorf("Auth.Scopes len = %d, want 2", len(meta.Auth.Scopes))
	}
}

// --- Convenience Publish / Unpublish tests ---

func TestPublish_Convenience(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/servers" {
			http.Error(w, "bad route", http.StatusBadRequest)
			return
		}
		var body ServerMetadata
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		body.ID = "pub-id"
		writeJSON(w, body)
	}))
	defer srv.Close()

	meta := ServerMetadata{Name: "convenience-server", Description: "Published via convenience"}
	result, err := Publish(context.Background(), srv.URL, "tok", meta)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.ID != "pub-id" {
		t.Errorf("ID = %q, want pub-id", result.ID)
	}
	if result.Name != "convenience-server" {
		t.Errorf("Name = %q, want convenience-server", result.Name)
	}
}

func TestPublish_MissingToken(t *testing.T) {
	t.Parallel()
	_, err := Publish(context.Background(), "http://example.com", "", ServerMetadata{})
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestUnpublish_Convenience(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := Unpublish(context.Background(), srv.URL, "tok", "srv-xyz"); err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v1/servers/srv-xyz" {
		t.Errorf("path = %q, want /v1/servers/srv-xyz", gotPath)
	}
}

func TestUnpublish_MissingToken(t *testing.T) {
	t.Parallel()
	err := Unpublish(context.Background(), "http://example.com", "", "srv-1")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}
