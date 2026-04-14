//go:build !official_sdk

package main

import "testing"

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
