package a2a

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestAgentCardFromRegistry(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()

	card := AgentCardFromRegistry(reg,
		WithName("test-server"),
		WithDescription("A test MCP server"),
		WithURL("https://example.com"),
		WithVersion("2.0.0"),
		WithOrganization("TestOrg", "https://testorg.com"),
		WithStreaming(),
		WithAuth("bearer"),
	)

	if card.Name != "test-server" {
		t.Errorf("Name = %q, want test-server", card.Name)
	}
	if card.Description != "A test MCP server" {
		t.Errorf("Description = %q", card.Description)
	}
	if card.URL != "https://example.com" {
		t.Errorf("URL = %q", card.URL)
	}
	if card.Version != "2.0.0" {
		t.Errorf("Version = %q", card.Version)
	}
	if card.Provider == nil || card.Provider.Organization != "TestOrg" {
		t.Error("Provider not set correctly")
	}
	if !card.Capabilities.Streaming {
		t.Error("Streaming should be true")
	}
	if card.Auth == nil || len(card.Auth.Schemes) != 1 || card.Auth.Schemes[0] != "bearer" {
		t.Error("Auth not set correctly")
	}
}

func TestAgentCardFromRegistry_Defaults(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg)

	if card.Name != "mcpkit-agent" {
		t.Errorf("default Name = %q, want mcpkit-agent", card.Name)
	}
	if card.Version != "1.0.0" {
		t.Errorf("default Version = %q, want 1.0.0", card.Version)
	}
	if card.Provider != nil {
		t.Error("Provider should be nil by default")
	}
	if card.Auth != nil {
		t.Error("Auth should be nil by default")
	}
}
