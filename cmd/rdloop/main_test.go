//go:build !official_sdk

package main

import "testing"

func TestResolveRDLoopAPIKeyPrefersExplicitOverride(t *testing.T) {
	t.Setenv("RDLOOP_API_KEY", "rdloop-key")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")

	got := resolveRDLoopAPIKey()
	if got != "rdloop-key" {
		t.Fatalf("resolveRDLoopAPIKey() = %q, want rdloop-key", got)
	}
}

func TestDefaultModelTierUsesHostedAnthropicLanes(t *testing.T) {
	tier := defaultModelTier("claude-sonnet-4-6")

	if tier.TaskOverrides["scan"] != "claude-sonnet-4-6" {
		t.Fatalf("scan model = %q, want claude-sonnet-4-6", tier.TaskOverrides["scan"])
	}
	if tier.TaskOverrides["plan"] != "claude-opus-4-6" {
		t.Fatalf("plan model = %q, want claude-opus-4-6", tier.TaskOverrides["plan"])
	}
	if tier.TaskOverrides["implement"] != "claude-opus-4-6" {
		t.Fatalf("implement model = %q, want claude-opus-4-6", tier.TaskOverrides["implement"])
	}
	if tier.TaskOverrides["verify"] != "claude-haiku-4-5" {
		t.Fatalf("verify model = %q, want claude-haiku-4-5", tier.TaskOverrides["verify"])
	}
}

func TestMarathonProfileIncludesHostedModelPricing(t *testing.T) {
	profile := marathonProfile()
	if len(profile.ModelPricing) == 0 {
		t.Fatal("expected hosted model pricing to be configured")
	}
}
