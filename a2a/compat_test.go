package a2a

import (
	"testing"
)

func TestToSDKAgentCard(t *testing.T) {
	t.Parallel()
	card := AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		URL:         "https://example.com",
		Version:     "1.0.0",
		Capabilities: &Capabilities{
			Streaming:         true,
			PushNotifications: false,
		},
		Skills: []Skill{
			{ID: "s1", Name: "skill-1", Description: "First skill", Tags: []string{"tag1"}},
		},
	}

	sdkCard := ToSDKAgentCard(card)

	if sdkCard.Name != "test-agent" {
		t.Errorf("Name = %q", sdkCard.Name)
	}
	if sdkCard.Version != "1.0.0" {
		t.Errorf("Version = %q", sdkCard.Version)
	}
	if !sdkCard.Capabilities.Streaming {
		t.Error("Streaming should be true")
	}
	if len(sdkCard.Skills) != 1 {
		t.Errorf("Skills = %d", len(sdkCard.Skills))
	}
	if len(sdkCard.SupportedInterfaces) != 1 || sdkCard.SupportedInterfaces[0].URL != "https://example.com" {
		t.Error("URL not in SupportedInterfaces")
	}
}

func TestFromSDKAgentCard(t *testing.T) {
	t.Parallel()
	sdkCard := ToSDKAgentCard(AgentCard{
		Name:    "roundtrip",
		URL:     "https://rt.example.com",
		Version: "2.0.0",
		Capabilities: &Capabilities{
			Streaming: true,
		},
		Skills: []Skill{
			{ID: "a", Name: "alpha"},
			{ID: "b", Name: "beta"},
		},
	})

	card := FromSDKAgentCard(sdkCard)

	if card.Name != "roundtrip" {
		t.Errorf("Name = %q", card.Name)
	}
	if card.URL != "https://rt.example.com" {
		t.Errorf("URL = %q", card.URL)
	}
	if card.Version != "2.0.0" {
		t.Errorf("Version = %q", card.Version)
	}
	if len(card.Skills) != 2 {
		t.Errorf("Skills = %d", len(card.Skills))
	}
}

func TestSDKVersion(t *testing.T) {
	t.Parallel()
	if v := SDKVersion(); v != "v2.1.0" {
		t.Errorf("SDKVersion() = %q", v)
	}
}
