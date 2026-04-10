//go:build !official_sdk

package main

import "testing"

func TestPreflightProbeForOllamaUsesTagsEndpoint(t *testing.T) {
	label, probeURL, method := preflightProbe("http://127.0.0.1:11434/v1")
	if label != "ollama-compatible backend" {
		t.Fatalf("label = %q, want ollama-compatible backend", label)
	}
	if probeURL != "http://127.0.0.1:11434/api/tags" {
		t.Fatalf("probeURL = %q, want http://127.0.0.1:11434/api/tags", probeURL)
	}
	if method != "GET" {
		t.Fatalf("method = %q, want GET", method)
	}
}

func TestPreflightProbeForAnthropicUsesHostHead(t *testing.T) {
	label, probeURL, method := preflightProbe("https://api.anthropic.com/v1/messages")
	if label != "api.anthropic.com" {
		t.Fatalf("label = %q, want api.anthropic.com", label)
	}
	if probeURL != "https://api.anthropic.com" {
		t.Fatalf("probeURL = %q, want https://api.anthropic.com", probeURL)
	}
	if method != "HEAD" {
		t.Fatalf("method = %q, want HEAD", method)
	}
}

func TestOllamaModelInstalledExactMatchesAliasAndLatest(t *testing.T) {
	tags := ollamaTagsResponse{
		Models: []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		}{
			{Name: "code-primary", Model: "code-primary:latest"},
			{Name: "devstral-small-2", Model: "devstral-small-2:latest"},
		},
	}

	if !ollamaModelInstalledExact(tags, "code-primary") {
		t.Fatal("expected code-primary alias to be installed")
	}
	if !ollamaModelInstalledExact(tags, "devstral-small-2") {
		t.Fatal("expected backing model to be installed")
	}
	if ollamaModelInstalledExact(tags, "code-long") {
		t.Fatal("did not expect code-long to be installed")
	}
}

func TestOllamaPullHintCommandUsesBackingModelForAliases(t *testing.T) {
	if got := ollamaPullHintCommand("code-primary"); got != "ollama pull devstral-small-2" {
		t.Fatalf("ollamaPullHintCommand(code-primary) = %q, want %q", got, "ollama pull devstral-small-2")
	}
	if got := ollamaPullHintCommand("nomic-embed-text:v1.5"); got != "ollama pull nomic-embed-text:v1.5" {
		t.Fatalf("ollamaPullHintCommand(embed) = %q, want %q", got, "ollama pull nomic-embed-text:v1.5")
	}
}
