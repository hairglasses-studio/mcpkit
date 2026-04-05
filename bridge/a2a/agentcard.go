package a2a

import (
	"sort"
	"sync"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// AgentCardGenerator produces A2A AgentCards from mcpkit registry state.
// It reads all registered tools and converts them to A2A skills via the
// configured Translator.
type AgentCardGenerator struct {
	registry   *registry.ToolRegistry
	translator *Translator
	config     CardConfig

	mu     sync.RWMutex
	cached *a2atypes.AgentCard
}

// CardConfig holds the metadata used to populate the AgentCard fields.
type CardConfig struct {
	// Name is the human-readable name for the A2A agent.
	Name string

	// Description describes the agent's purpose.
	Description string

	// Version is the semantic version string. Default: "1.0.0".
	Version string

	// URL is the base URL where the agent is served.
	URL string

	// Provider identifies the agent's creator. Optional.
	Provider *a2atypes.AgentProvider

	// ToolFilter selects which tools to expose as A2A skills.
	// If nil, all tools are exposed.
	ToolFilter func(name string, td registry.ToolDefinition) bool
}

// NewAgentCardGenerator creates a generator bound to the given registry.
// If translator is nil, a zero-value Translator is used.
func NewAgentCardGenerator(
	reg *registry.ToolRegistry,
	translator *Translator,
	cfg CardConfig,
) *AgentCardGenerator {
	if translator == nil {
		translator = &Translator{}
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	return &AgentCardGenerator{
		registry:   reg,
		translator: translator,
		config:     cfg,
	}
}

// Generate produces a fresh AgentCard by scanning all registered tools
// and converting them to skills via the translator. The result is cached
// and returned by Card() until Invalidate() is called.
func (g *AgentCardGenerator) Generate() *a2atypes.AgentCard {
	card := &a2atypes.AgentCard{
		Name:        g.config.Name,
		Description: g.config.Description,
		Version:     g.config.Version,
		Provider:    g.config.Provider,
		Capabilities: a2atypes.AgentCapabilities{
			Streaming:         false,
			PushNotifications: false,
		},
		DefaultInputModes:  []string{"application/json"},
		DefaultOutputModes: []string{"text/plain"},
	}

	// Build the supported interface if URL is configured.
	if g.config.URL != "" {
		card.SupportedInterfaces = []*a2atypes.AgentInterface{
			a2atypes.NewAgentInterface(g.config.URL, a2atypes.TransportProtocolHTTPJSON),
		}
	}

	// Convert registered tools to A2A skills.
	allTools := g.registry.GetAllToolDefinitions()
	var skills []a2atypes.AgentSkill
	for _, td := range allTools {
		name := td.Tool.Name

		// Apply filter if configured.
		if g.config.ToolFilter != nil && !g.config.ToolFilter(name, td) {
			continue
		}

		skill := g.translator.ToolToSkill(td)
		skills = append(skills, skill)
	}

	// Sort for deterministic output.
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].ID < skills[j].ID
	})
	card.Skills = skills

	// Cache the generated card.
	g.mu.Lock()
	g.cached = card
	g.mu.Unlock()

	return card
}

// Card returns the cached AgentCard, regenerating if stale.
func (g *AgentCardGenerator) Card() *a2atypes.AgentCard {
	g.mu.RLock()
	cached := g.cached
	g.mu.RUnlock()

	if cached != nil {
		return cached
	}

	return g.Generate()
}

// Invalidate forces regeneration on the next Card() call.
func (g *AgentCardGenerator) Invalidate() {
	g.mu.Lock()
	g.cached = nil
	g.mu.Unlock()
}
