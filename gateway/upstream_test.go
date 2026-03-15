//go:build !official_sdk

package gateway

import (
	"testing"
	"time"
)

func TestNamespacedName(t *testing.T) {
	tests := []struct {
		namespace, tool, want string
	}{
		{"github", "list_repos", "github.list_repos"},
		{"slack", "send_message", "slack.send_message"},
		{"a", "b", "a.b"},
	}
	for _, tt := range tests {
		got := namespacedName(tt.namespace, tt.tool)
		if got != tt.want {
			t.Errorf("namespacedName(%q, %q) = %q, want %q", tt.namespace, tt.tool, got, tt.want)
		}
	}
}

func TestOriginalName(t *testing.T) {
	tests := []struct {
		namespace, namespaced, want string
	}{
		{"github", "github.list_repos", "list_repos"},
		{"slack", "slack.send_message", "send_message"},
	}
	for _, tt := range tests {
		got := originalName(tt.namespace, tt.namespaced)
		if got != tt.want {
			t.Errorf("originalName(%q, %q) = %q, want %q", tt.namespace, tt.namespaced, got, tt.want)
		}
	}
}

func TestUpstreamConfigDefaults(t *testing.T) {
	c := UpstreamConfig{Name: "test", URL: "http://localhost"}
	c.applyDefaults()

	if c.HealthInterval != 30*time.Second {
		t.Errorf("expected 30s HealthInterval, got %v", c.HealthInterval)
	}
	if c.UnhealthyThreshold != 3 {
		t.Errorf("expected 3 UnhealthyThreshold, got %d", c.UnhealthyThreshold)
	}
}
