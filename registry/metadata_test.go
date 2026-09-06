package registry

import "testing"

func TestApplyToolMetadata_AnthropicGoogleAntigravityParity(t *testing.T) {
	td := ToolDefinition{
		Tool:           Tool{Name: "deploy_workstation"},
		IsWrite:        true,
		AlwaysLoad:     true,
		MaxResultChars: 4096,
	}

	applied := ApplyToolMetadata(td, "", false)

	boolKeys := []string{
		"anthropic/alwaysLoad",
		"google/alwaysLoad",
		"antigravity/alwaysLoad",
		"anthropic/requiresUserInteraction",
		"google/requiresUserInteraction",
		"antigravity/requiresUserInteraction",
	}

	for _, k := range boolKeys {
		v, ok := ToolMetaField(applied.Tool, k)
		if !ok {
			t.Fatalf("missing expected metadata key %s", k)
		}
		b, ok := v.(bool)
		if !ok || !b {
			t.Errorf("key %s = %v (%T), want true", k, v, v)
		}
	}

	intKeys := []string{
		"anthropic/maxResultSizeChars",
		"google/maxResultSizeChars",
		"antigravity/maxResultSizeChars",
	}

	for _, k := range intKeys {
		v, ok := ToolMetaField(applied.Tool, k)
		if !ok {
			t.Fatalf("missing expected metadata key %s", k)
		}
		n, ok := v.(int)
		if !ok || n != 4096 {
			t.Errorf("key %s = %v (%T), want 4096", k, v, v)
		}
	}
}

func TestApplyToolMetadata_ReadOnlyDefaults(t *testing.T) {
	td := ToolDefinition{
		Tool:    Tool{Name: "read_telemetry"},
		IsWrite: false,
	}

	applied := ApplyToolMetadata(td, "", false)

	for _, k := range []string{
		"anthropic/requiresUserInteraction",
		"google/requiresUserInteraction",
		"antigravity/requiresUserInteraction",
		"anthropic/alwaysLoad",
		"google/alwaysLoad",
		"antigravity/alwaysLoad",
		"anthropic/maxResultSizeChars",
		"google/maxResultSizeChars",
		"antigravity/maxResultSizeChars",
	} {
		if v, ok := ToolMetaField(applied.Tool, k); ok {
			t.Errorf("did not expect key %s to be set on read-only tool without hints, got %v", k, v)
		}
	}
}

func TestApplyToolMetadata_SkipRequiresUserInteraction(t *testing.T) {
	td := ToolDefinition{
		Tool:                        Tool{Name: "silent_append"},
		IsWrite:                     true,
		SkipRequiresUserInteraction: true,
	}

	applied := ApplyToolMetadata(td, "", false)

	for _, k := range []string{
		"anthropic/requiresUserInteraction",
		"google/requiresUserInteraction",
		"antigravity/requiresUserInteraction",
	} {
		if v, ok := ToolMetaField(applied.Tool, k); ok {
			t.Errorf("did not expect key %s when SkipRequiresUserInteraction is true, got %v", k, v)
		}
	}
}
