package a2a

// Compatibility layer between mcpkit's A2A bridge and the official a2aproject/a2a-go SDK.

import (
	a2asdk "github.com/a2aproject/a2a-go/v2/a2a"
)

// ToSDKAgentCard converts a mcpkit AgentCard to the official SDK type.
func ToSDKAgentCard(card AgentCard) a2asdk.AgentCard {
	sdkCard := a2asdk.AgentCard{
		Name:        card.Name,
		Description: card.Description,
		Capabilities: a2asdk.AgentCapabilities{
			Streaming:         card.Capabilities != nil && card.Capabilities.Streaming,
			PushNotifications: card.Capabilities != nil && card.Capabilities.PushNotifications,
		},
	}

	if card.Version != "" {
		sdkCard.Version = card.Version
	}

	// Convert skills
	for _, s := range card.Skills {
		sdkCard.Skills = append(sdkCard.Skills, a2asdk.AgentSkill{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
		})
	}

	// Add URL as supported interface
	if card.URL != "" {
		sdkCard.SupportedInterfaces = append(sdkCard.SupportedInterfaces, &a2asdk.AgentInterface{
			URL: card.URL,
		})
	}

	return sdkCard
}

// FromSDKAgentCard converts an official SDK AgentCard to mcpkit's type.
func FromSDKAgentCard(sdkCard a2asdk.AgentCard) AgentCard {
	card := AgentCard{
		Name:        sdkCard.Name,
		Description: sdkCard.Description,
		Version:     sdkCard.Version,
		Capabilities: &Capabilities{
			Streaming:         sdkCard.Capabilities.Streaming,
			PushNotifications: sdkCard.Capabilities.PushNotifications,
		},
	}

	// Extract URL from first supported interface
	if len(sdkCard.SupportedInterfaces) > 0 {
		card.URL = sdkCard.SupportedInterfaces[0].URL
	}

	for _, s := range sdkCard.Skills {
		card.Skills = append(card.Skills, Skill{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
		})
	}

	return card
}

// SDKVersion returns the version of the official A2A Go SDK in use.
func SDKVersion() string {
	return "v2.1.0"
}
