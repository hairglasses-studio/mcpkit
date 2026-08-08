//go:build official_sdk

// compat_meta_official_test.go exercises RequestMetaCarrier/SetResultMetaCarrier
// (added for the observability official-sdk port, fleet flip-order #1)
// against the official SDK's plain map[string]any Meta shape. See
// compat_meta_test.go for mcp-go's *mcp.Meta/AdditionalFields counterpart.
package registry

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRequestMetaCarrier_NilParams(t *testing.T) {
	req := CallToolRequest{}
	if carrier := RequestMetaCarrier(req); carrier != nil {
		t.Errorf("expected nil carrier for request with nil Params, got %v", carrier)
	}
}

func TestRequestMetaCarrier_NilMeta(t *testing.T) {
	req := CallToolRequest{Params: &mcp.CallToolParamsRaw{}}
	if carrier := RequestMetaCarrier(req); carrier != nil {
		t.Errorf("expected nil carrier for request with no _meta, got %v", carrier)
	}
}

func TestRequestMetaCarrier_StringFields(t *testing.T) {
	req := CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Meta: mcp.Meta{
				"traceparent": "00-abc-def-01",
				"tracestate":  "vendor=1",
				"non_string":  42,
			},
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
	if result.Meta == nil {
		t.Fatal("expected Meta to be allocated")
	}
	if result.Meta["traceparent"] != "00-xyz-01" {
		t.Errorf("traceparent = %v, want 00-xyz-01", result.Meta["traceparent"])
	}

	// Merging into an already-populated Meta preserves existing keys.
	result.Meta["existing"] = "keep-me"
	SetResultMetaCarrier(result, map[string]string{"tracestate": "vendor=1"})
	if result.Meta["existing"] != "keep-me" {
		t.Error("expected pre-existing Meta entry to survive merge")
	}
	if result.Meta["tracestate"] != "vendor=1" {
		t.Error("expected new carrier entry to be merged in")
	}
}

func TestRequestResultMetaCarrier_RoundTrip(t *testing.T) {
	// Simulate cross-server propagation: inject into a result's _meta, then
	// read those same fields back via a request's _meta.
	result := &CallToolResult{}
	carrier := map[string]string{"traceparent": "00-roundtrip-01"}
	SetResultMetaCarrier(result, carrier)

	req := CallToolRequest{Params: &mcp.CallToolParamsRaw{Meta: result.Meta}}
	got := RequestMetaCarrier(req)
	if got["traceparent"] != "00-roundtrip-01" {
		t.Errorf("round-tripped traceparent = %v, want 00-roundtrip-01", got)
	}
}
