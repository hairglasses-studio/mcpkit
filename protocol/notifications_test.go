package protocol

import (
	"testing"
)

func TestNotificationConstants(t *testing.T) {
	t.Parallel()

	// Verify notification method strings match the MCP specification.
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Initialized", NotificationInitialized, "notifications/initialized"},
		{"Cancelled", NotificationCancelled, "notifications/cancelled"},
		{"Progress", NotificationProgress, "notifications/progress"},
		{"ToolsListChanged", NotificationToolsListChanged, "notifications/tools/list_changed"},
		{"ResourcesListChanged", NotificationResourcesListChanged, "notifications/resources/list_changed"},
		{"ResourceUpdated", NotificationResourceUpdated, "notifications/resources/updated"},
		{"PromptsListChanged", NotificationPromptsListChanged, "notifications/prompts/list_changed"},
		{"RootsListChanged", NotificationRootsListChanged, "notifications/roots/list_changed"},
		{"Message", NotificationMessage, "notifications/message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("Notification%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestMethodConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Initialize", MethodInitialize, "initialize"},
		{"Ping", MethodPing, "ping"},
		{"ToolsList", MethodToolsList, "tools/list"},
		{"ToolsCall", MethodToolsCall, "tools/call"},
		{"ResourcesList", MethodResourcesList, "resources/list"},
		{"ResourcesRead", MethodResourcesRead, "resources/read"},
		{"PromptsList", MethodPromptsList, "prompts/list"},
		{"PromptsGet", MethodPromptsGet, "prompts/get"},
		{"LoggingSetLevel", MethodLoggingSetLevel, "logging/setLevel"},
		{"CompletionComplete", MethodCompletionComplete, "completion/complete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("Method%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestCancelledParams(t *testing.T) {
	t.Parallel()

	p := CancelledParams{
		RequestID: "req-42",
		Reason:    "user navigated away",
	}

	if p.RequestID != "req-42" {
		t.Errorf("RequestID = %v, want req-42", p.RequestID)
	}
	if p.Reason != "user navigated away" {
		t.Errorf("Reason = %q, want 'user navigated away'", p.Reason)
	}
}
