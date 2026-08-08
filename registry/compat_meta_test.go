//go:build !official_sdk

// compat_meta_test.go exercises RequestMetaCarrier/SetResultMetaCarrier
// (added for the observability official-sdk port, fleet flip-order #1)
// against mcp-go's *mcp.Meta/AdditionalFields shape. See
// compat_meta_official_test.go for the official SDK's plain-map
// counterpart.
package registry

import "testing"

func TestRequestMetaCarrier_NilMeta(t *testing.T) {
	req := CallToolRequest{}
	if carrier := RequestMetaCarrier(req); carrier != nil {
		t.Errorf("expected nil carrier for request with no _meta, got %v", carrier)
	}
}

func TestRequestMetaCarrier_StringFields(t *testing.T) {
	req := CallToolRequest{}
	req.Params.Meta = &ToolMeta{
		AdditionalFields: map[string]any{
			"traceparent": "00-abc-def-01",
			"tracestate":  "vendor=1",
			"non_string":  42,
		},
	}
	carrier := RequestMetaCarrier(req)
	if carrier == nil {
		t.Fatal("expected non-nil carrier")
	}
	if carrier["traceparent"] != "00-abc-def-01" {
		t.Errorf("traceparent = %q, want 00-abc-def-01", carrier["traceparent"])
	}
	if carrier["tracestate"] != "vendor=1" {
		t.Errorf("tracestate = %q, want vendor=1", carrier["tracestate"])
	}
	if _, ok := carrier["non_string"]; ok {
		t.Error("non-string field should have been skipped")
	}
}

func TestSetResultMetaCarrier_NilResult(t *testing.T) {
	// Must not panic.
	SetResultMetaCarrier(nil, map[string]string{"k": "v"})
}

func TestSetResultMetaCarrier_EmptyCarrier(t *testing.T) {
	result := &CallToolResult{}
	SetResultMetaCarrier(result, nil)
	if result.Meta != nil {
		t.Error("expected Meta to remain nil for empty carrier")
	}
}

func TestSetResultMetaCarrier_AllocatesAndMerges(t *testing.T) {
	result := &CallToolResult{}
	SetResultMetaCarrier(result, map[string]string{"traceparent": "00-xyz-01"})
	if result.Meta == nil || result.Meta.AdditionalFields == nil {
		t.Fatal("expected Meta.AdditionalFields to be allocated")
	}
	if result.Meta.AdditionalFields["traceparent"] != "00-xyz-01" {
		t.Errorf("traceparent = %v, want 00-xyz-01", result.Meta.AdditionalFields["traceparent"])
	}

	// Merging into an already-populated Meta preserves existing keys.
	result.Meta.AdditionalFields["existing"] = "keep-me"
	SetResultMetaCarrier(result, map[string]string{"tracestate": "vendor=1"})
	if result.Meta.AdditionalFields["existing"] != "keep-me" {
		t.Error("expected pre-existing AdditionalFields entry to survive merge")
	}
	if result.Meta.AdditionalFields["tracestate"] != "vendor=1" {
		t.Error("expected new carrier entry to be merged in")
	}
}

func TestRequestResultMetaCarrier_RoundTrip(t *testing.T) {
	// Simulate cross-server propagation: inject into a result's _meta, then
	// read those same fields back via a request's _meta (the shape both
	// accessors work against, AdditionalFields, is symmetric).
	result := &CallToolResult{}
	carrier := map[string]string{"traceparent": "00-roundtrip-01"}
	SetResultMetaCarrier(result, carrier)

	req := CallToolRequest{}
	req.Params.Meta = result.Meta
	got := RequestMetaCarrier(req)
	if got["traceparent"] != "00-roundtrip-01" {
		t.Errorf("round-tripped traceparent = %v, want 00-roundtrip-01", got)
	}
}
