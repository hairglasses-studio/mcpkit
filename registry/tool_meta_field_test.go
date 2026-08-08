package registry

import "testing"

// TestToolMetaField_RoundTrip exercises the SetToolMetaField/ToolMetaField
// pair — write, then read back — on both tags despite mcp-go storing fields
// under tool.Meta.AdditionalFields and the official SDK storing them
// directly in the plain-map Meta.
func TestToolMetaField_RoundTrip(t *testing.T) {
	tool := Tool{Name: "test_tool"}
	SetToolMetaField(&tool, "anthropic/alwaysLoad", true)
	SetToolMetaField(&tool, "some_key", "some_value")

	v, ok := ToolMetaField(tool, "anthropic/alwaysLoad")
	if !ok {
		t.Fatal("expected anthropic/alwaysLoad to be present")
	}
	if b, ok := v.(bool); !ok || !b {
		t.Errorf("anthropic/alwaysLoad = %v (%T), want true", v, v)
	}

	v, ok = ToolMetaField(tool, "some_key")
	if !ok {
		t.Fatal("expected some_key to be present")
	}
	if s, ok := v.(string); !ok || s != "some_value" {
		t.Errorf("some_key = %v (%T), want %q", v, v, "some_value")
	}
}

func TestToolMetaField_Missing(t *testing.T) {
	tool := Tool{Name: "test_tool"}
	if _, ok := ToolMetaField(tool, "nonexistent"); ok {
		t.Error("expected ok=false for a tool with no Meta set at all")
	}

	SetToolMetaField(&tool, "present", 1)
	if _, ok := ToolMetaField(tool, "still_missing"); ok {
		t.Error("expected ok=false for a key never set")
	}
}
