package a2a

import (
	"github.com/hairglasses-studio/mcpkit/registry"
)

// AgentCardFromRegistry generates an A2A AgentCard from an MCP tool registry.
// Each registered tool becomes a Skill in the agent card.
func AgentCardFromRegistry(reg *registry.ToolRegistry, opts ...CardOption) AgentCard {
	cfg := cardConfig{
		name:    "mcpkit-agent",
		version: "1.0.0",
	}
	for _, o := range opts {
		o(&cfg)
	}

	card := AgentCard{
		Name:        cfg.name,
		Description: cfg.description,
		URL:         cfg.url,
		Version:     cfg.version,
		Capabilities: &Capabilities{
			Streaming:         cfg.streaming,
			PushNotifications: cfg.pushNotifications,
		},
	}

	if cfg.organization != "" {
		card.Provider = &Provider{
			Organization: cfg.organization,
			URL:          cfg.providerURL,
		}
	}

	if cfg.authSchemes != nil {
		card.Auth = &AuthConfig{Schemes: cfg.authSchemes}
	}

	// Convert MCP tools to A2A skills
	catalog := reg.GetToolCatalog()
	for category, groups := range catalog {
		for _, tools := range groups {
			for _, td := range tools {
				skill := Skill{
					ID:          td.Tool.Name,
					Name:        td.Tool.Name,
					Description: td.Tool.Description,
				}
				if category != "" {
					skill.Tags = append(skill.Tags, category)
				}
				skill.Tags = append(skill.Tags, td.Tags...)
				card.Skills = append(card.Skills, skill)
			}
		}
	}

	return card
}

// CardOption configures AgentCard generation.
type CardOption func(*cardConfig)

type cardConfig struct {
	name              string
	description       string
	url               string
	version           string
	organization      string
	providerURL       string
	streaming         bool
	pushNotifications bool
	authSchemes       []string
}

// WithName sets the agent name.
func WithName(name string) CardOption {
	return func(c *cardConfig) { c.name = name }
}

// WithDescription sets the agent description.
func WithDescription(desc string) CardOption {
	return func(c *cardConfig) { c.description = desc }
}

// WithURL sets the agent URL.
func WithURL(url string) CardOption {
	return func(c *cardConfig) { c.url = url }
}

// WithVersion sets the agent version.
func WithVersion(version string) CardOption {
	return func(c *cardConfig) { c.version = version }
}

// WithOrganization sets the provider organization.
func WithOrganization(org, url string) CardOption {
	return func(c *cardConfig) {
		c.organization = org
		c.providerURL = url
	}
}

// WithStreaming enables streaming capability.
func WithStreaming() CardOption {
	return func(c *cardConfig) { c.streaming = true }
}

// WithPushNotifications enables push notification capability.
func WithPushNotifications() CardOption {
	return func(c *cardConfig) { c.pushNotifications = true }
}

// WithAuth sets the authentication schemes.
func WithAuth(schemes ...string) CardOption {
	return func(c *cardConfig) { c.authSchemes = schemes }
}
