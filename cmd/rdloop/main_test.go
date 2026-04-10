//go:build !official_sdk

package main

import "testing"

func TestResolveRDLoopAPIKeyPrefersOllamaForLocalTargets(t *testing.T) {
	t.Setenv("RDLOOP_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	t.Setenv("OLLAMA_API_KEY", "ollama-key")

	got := resolveRDLoopAPIKey("ollama", "http://127.0.0.1:11434")
	if got != "ollama-key" {
		t.Fatalf("resolveRDLoopAPIKey() = %q, want ollama-key", got)
	}
}

func TestDefaultModelTierForOllamaUsesLocalModelFamilies(t *testing.T) {
	t.Setenv("OLLAMA_FAST_MODEL", "code-fast")
	t.Setenv("OLLAMA_CODE_MODEL", "code-primary")
	t.Setenv("OLLAMA_HIGH_CONTEXT_CODE_MODEL", "code-long")

	tier := defaultModelTier("ollama", "http://127.0.0.1:11434", "code-primary")

	if tier.TaskOverrides["scan"] != "code-long" {
		t.Fatalf("scan model = %q, want code-long", tier.TaskOverrides["scan"])
	}
	if tier.TaskOverrides["plan"] != "code-primary" {
		t.Fatalf("plan model = %q, want code-primary", tier.TaskOverrides["plan"])
	}
	if tier.TaskOverrides["implement"] != "code-primary" {
		t.Fatalf("implement model = %q, want code-primary", tier.TaskOverrides["implement"])
	}
	if tier.TaskOverrides["verify"] != "code-fast" {
		t.Fatalf("verify model = %q, want code-fast", tier.TaskOverrides["verify"])
	}
}

func TestMarathonProfileForOllamaUsesZeroDollarAccounting(t *testing.T) {
	profile := marathonProfile("ollama", "http://127.0.0.1:11434")
	if len(profile.ModelPricing) != 0 {
		t.Fatalf("model pricing len = %d, want 0", len(profile.ModelPricing))
	}
}
