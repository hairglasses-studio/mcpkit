package a2a

import (
	"context"
	"testing"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestAgentCardGenerator_Generate_BasicFields(t *testing.T) {
	reg := registry.NewToolRegistry()
	gen := NewAgentCardGenerator(reg, nil, CardConfig{
		Name:        "test-agent",
		Description: "A test agent",
		Version:     "2.0.0",
		URL:         "http://localhost:8080",
		Provider: &a2atypes.AgentProvider{
			Org: "test-org",
			URL: "https://test-org.example.com",
		},
	})

	card := gen.Generate()

	if card.Name != "test-agent" {
		t.Errorf("expected name %q, got %q", "test-agent", card.Name)
	}
	if card.Description != "A test agent" {
		t.Errorf("expected description %q, got %q", "A test agent", card.Description)
	}
	if card.Version != "2.0.0" {
		t.Errorf("expected version %q, got %q", "2.0.0", card.Version)
	}
	if card.Provider == nil {
		t.Fatal("expected provider to be set")
	}
	if card.Provider.Org != "test-org" {
		t.Errorf("expected provider org %q, got %q", "test-org", card.Provider.Org)
	}
	if card.Capabilities.Streaming {
		t.Error("expected streaming to be false")
	}
	if card.Capabilities.PushNotifications {
		t.Error("expected push notifications to be false")
	}
	if len(card.SupportedInterfaces) != 1 {
		t.Fatalf("expected 1 supported interface, got %d", len(card.SupportedInterfaces))
	}
	if card.SupportedInterfaces[0].URL != "http://localhost:8080" {
		t.Errorf("expected URL %q, got %q", "http://localhost:8080", card.SupportedInterfaces[0].URL)
	}
	if card.SupportedInterfaces[0].ProtocolBinding != a2atypes.TransportProtocolHTTPJSON {
		t.Errorf("expected protocol %q, got %q", a2atypes.TransportProtocolHTTPJSON, card.SupportedInterfaces[0].ProtocolBinding)
	}
}

func TestAgentCardGenerator_Generate_DefaultVersion(t *testing.T) {
	reg := registry.NewToolRegistry()
	gen := NewAgentCardGenerator(reg, nil, CardConfig{
		Name: "test-agent",
	})

	card := gen.Generate()

	if card.Version != "1.0.0" {
		t.Errorf("expected default version %q, got %q", "1.0.0", card.Version)
	}
}

func TestAgentCardGenerator_Generate_SkillsMatchRegisteredTools(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name:        "test-module",
		description: "Test module",
		tools: []registry.ToolDefinition{
			{
				Tool: registry.Tool{
					Name:        "tool_alpha",
					Description: "First tool",
				},
				Category: "system",
				Tags:     []string{"status"},
				Handler:  noopHandler,
			},
			{
				Tool: registry.Tool{
					Name:        "tool_beta",
					Description: "Second tool",
				},
				Category: "network",
				Tags:     []string{"dns"},
				IsWrite:  true,
				Handler:  noopHandler,
			},
		},
	})

	gen := NewAgentCardGenerator(reg, nil, CardConfig{
		Name: "test-agent",
	})

	card := gen.Generate()

	if len(card.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(card.Skills))
	}

	// Skills should be sorted by ID.
	if card.Skills[0].ID != "tool_alpha" {
		t.Errorf("expected first skill ID %q, got %q", "tool_alpha", card.Skills[0].ID)
	}
	if card.Skills[1].ID != "tool_beta" {
		t.Errorf("expected second skill ID %q, got %q", "tool_beta", card.Skills[1].ID)
	}

	// Check descriptions.
	if card.Skills[0].Description != "First tool" {
		t.Errorf("expected description %q, got %q", "First tool", card.Skills[0].Description)
	}
	if card.Skills[1].Description != "Second tool" {
		t.Errorf("expected description %q, got %q", "Second tool", card.Skills[1].Description)
	}

	// Check tags.
	assertContainsTag(t, card.Skills[0].Tags, "system")
	assertContainsTag(t, card.Skills[0].Tags, "status")
	assertContainsTag(t, card.Skills[0].Tags, "read")

	assertContainsTag(t, card.Skills[1].Tags, "network")
	assertContainsTag(t, card.Skills[1].Tags, "dns")
	assertContainsTag(t, card.Skills[1].Tags, "write")

	// Check input/output modes.
	if len(card.Skills[0].InputModes) != 1 || card.Skills[0].InputModes[0] != "application/json" {
		t.Errorf("expected input mode application/json, got %v", card.Skills[0].InputModes)
	}
}

func TestAgentCardGenerator_Generate_EmptyRegistry(t *testing.T) {
	reg := registry.NewToolRegistry()
	gen := NewAgentCardGenerator(reg, nil, CardConfig{
		Name:        "empty-agent",
		Description: "Agent with no tools",
	})

	card := gen.Generate()

	if card.Name != "empty-agent" {
		t.Errorf("expected name %q, got %q", "empty-agent", card.Name)
	}
	if card.Skills == nil {
		// Skills should be nil for empty registry, which is valid.
		// Just verify we don't panic.
	}
	if len(card.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(card.Skills))
	}
}

func TestAgentCardGenerator_Generate_NoURL(t *testing.T) {
	reg := registry.NewToolRegistry()
	gen := NewAgentCardGenerator(reg, nil, CardConfig{
		Name: "no-url-agent",
	})

	card := gen.Generate()

	if len(card.SupportedInterfaces) != 0 {
		t.Errorf("expected 0 supported interfaces when URL is empty, got %d", len(card.SupportedInterfaces))
	}
}

func TestAgentCardGenerator_Generate_ToolFilter(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "test",
		tools: []registry.ToolDefinition{
			{
				Tool:     registry.Tool{Name: "include_me", Description: "included"},
				Category: "system",
				Handler:  noopHandler,
			},
			{
				Tool:     registry.Tool{Name: "exclude_me", Description: "excluded"},
				Category: "system",
				IsWrite:  true,
				Handler:  noopHandler,
			},
		},
	})

	gen := NewAgentCardGenerator(reg, nil, CardConfig{
		Name: "filtered-agent",
		ToolFilter: func(name string, td registry.ToolDefinition) bool {
			return !td.IsWrite
		},
	})

	card := gen.Generate()

	if len(card.Skills) != 1 {
		t.Fatalf("expected 1 skill after filtering, got %d", len(card.Skills))
	}
	if card.Skills[0].ID != "include_me" {
		t.Errorf("expected skill ID %q, got %q", "include_me", card.Skills[0].ID)
	}
}

func TestAgentCardGenerator_Card_Caching(t *testing.T) {
	reg := registry.NewToolRegistry()
	gen := NewAgentCardGenerator(reg, nil, CardConfig{
		Name: "cached-agent",
	})

	// First call generates.
	card1 := gen.Card()
	if card1 == nil {
		t.Fatal("expected non-nil card")
	}

	// Second call returns cached.
	card2 := gen.Card()
	if card1 != card2 {
		t.Error("expected cached card to be the same pointer")
	}

	// Invalidate forces regeneration.
	gen.Invalidate()
	card3 := gen.Card()
	if card3 == card1 {
		t.Error("expected new card after invalidation")
	}
}

// --- test helpers ---

type testModule struct {
	name        string
	description string
	tools       []registry.ToolDefinition
}

func (m *testModule) Name() string                  { return m.name }
func (m *testModule) Description() string            { return m.description }
func (m *testModule) Tools() []registry.ToolDefinition { return m.tools }

func noopHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("ok"), nil
}

func assertContainsTag(t *testing.T, tags []string, expected string) {
	t.Helper()
	for _, tag := range tags {
		if tag == expected {
			return
		}
	}
	t.Errorf("expected tags %v to contain %q", tags, expected)
}
