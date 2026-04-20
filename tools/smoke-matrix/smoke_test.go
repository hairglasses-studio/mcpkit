//go:build !official_sdk

package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// validateTools
// ---------------------------------------------------------------------------

func TestValidateTools_Empty(t *testing.T) {
	err := validateTools(nil)
	if err == nil {
		t.Fatal("expected error for empty tools slice, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected 'empty' in error, got: %v", err)
	}
}

func TestValidateTools_MissingName(t *testing.T) {
	tools := []ToolInfo{{Name: "", Description: "some desc"}}
	err := validateTools(tools)
	if err == nil {
		t.Fatal("expected error for tool with empty name")
	}
	if !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("expected 'missing name' in error, got: %v", err)
	}
}

func TestValidateTools_MissingDescription(t *testing.T) {
	tools := []ToolInfo{{Name: "my_tool", Description: ""}}
	err := validateTools(tools)
	if err == nil {
		t.Fatal("expected error for tool with empty description")
	}
	if !strings.Contains(err.Error(), "missing description") {
		t.Fatalf("expected 'missing description' in error, got: %v", err)
	}
}

func TestValidateTools_Valid(t *testing.T) {
	tools := []ToolInfo{
		{Name: "greet", Description: "Greet a user."},
		{Name: "echo", Description: "Echo a message."},
	}
	if err := validateTools(tools); err != nil {
		t.Fatalf("unexpected error for valid tools: %v", err)
	}
}

// ---------------------------------------------------------------------------
// extractTools
// ---------------------------------------------------------------------------

func TestExtractTools_ValidResponse(t *testing.T) {
	body := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"result": {
			"tools": [
				{"name": "greet", "description": "Greet a user by name."},
				{"name": "word_count", "description": "Count words in text."}
			]
		}
	}`)

	tools, err := extractTools(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "greet" {
		t.Errorf("expected tools[0].Name=greet, got %q", tools[0].Name)
	}
	if tools[1].Description != "Count words in text." {
		t.Errorf("unexpected description: %q", tools[1].Description)
	}
}

func TestExtractTools_RPCError(t *testing.T) {
	body := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"error": {"code": -32600, "message": "Invalid Request"}
	}`)

	_, err := extractTools(body)
	if err == nil {
		t.Fatal("expected error for RPC error response, got nil")
	}
	if !strings.Contains(err.Error(), "RPC error") {
		t.Fatalf("expected 'RPC error' in error, got: %v", err)
	}
}

func TestExtractTools_EmptyTools(t *testing.T) {
	body := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"result": {
			"tools": []
		}
	}`)

	tools, err := extractTools(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(tools))
	}
}

func TestExtractTools_MalformedJSON(t *testing.T) {
	_, err := extractTools([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// stripSSEFrame
// ---------------------------------------------------------------------------

func TestStripSSEFrame_DataLine(t *testing.T) {
	sse := []byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n")
	got := stripSSEFrame(sse)
	want := `{"jsonrpc":"2.0","id":1,"result":{}}`
	if string(got) != want {
		t.Errorf("stripSSEFrame = %q, want %q", string(got), want)
	}
}

func TestStripSSEFrame_NoDataLine(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)
	got := stripSSEFrame(raw)
	if string(got) != string(raw) {
		t.Errorf("expected passthrough for non-SSE data, got %q", string(got))
	}
}

func TestStripSSEFrame_EventAndData(t *testing.T) {
	sse := []byte("event: message\ndata: {\"result\":\"ok\"}\n\n")
	got := stripSSEFrame(sse)
	want := `{"result":"ok"}`
	if string(got) != want {
		t.Errorf("stripSSEFrame = %q, want %q", string(got), want)
	}
}

// ---------------------------------------------------------------------------
// statusCell
// ---------------------------------------------------------------------------

func TestStatusCell(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{StatusPass, "PASS"},
		{StatusFail, "FAIL"},
		{StatusSkip, "skip"},
		{"unknown", "  ?  "},
	}
	for _, c := range cases {
		got := statusCell(c.in)
		if got != c.want {
			t.Errorf("statusCell(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// rpcRequest
// ---------------------------------------------------------------------------

func TestRPCRequest_Structure(t *testing.T) {
	body, err := rpcRequest(42, "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(body), `"method":"tools/list"`) {
		t.Errorf("expected method in body, got: %s", body)
	}
	if !strings.Contains(string(body), `"id":42`) {
		t.Errorf("expected id 42 in body, got: %s", body)
	}
	if !strings.Contains(string(body), `"jsonrpc":"2.0"`) {
		t.Errorf("expected jsonrpc 2.0 in body, got: %s", body)
	}
}

func TestRPCRequest_WithParams(t *testing.T) {
	params := map[string]any{"name": "greet"}
	body, err := rpcRequest(1, "tools/call", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(body), `"params"`) {
		t.Errorf("expected params in body, got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// printTable (smoke test — should not panic)
// ---------------------------------------------------------------------------

func TestPrintTable_NoPanic(t *testing.T) {
	report := SmokeReport{
		Timestamp: "2026-01-01T00:00:00Z",
		Pass:      1,
		Fail:      1,
		Skip:      2,
		Examples: []ExampleResult{
			{
				Example: "minimal",
				Results: []TransportResult{
					{Transport: TransportStdio, Status: StatusPass, Tools: []ToolInfo{{Name: "greet", Description: "Greet."}}},
					{Transport: TransportHTTP, Status: StatusSkip, Reason: "stdio-only"},
				},
			},
			{
				Example: "http",
				Results: []TransportResult{
					{Transport: TransportStdio, Status: StatusSkip, Reason: "http-only"},
					{Transport: TransportHTTP, Status: StatusFail, Error: "connection refused"},
				},
			},
		},
	}

	var sb strings.Builder
	// Should not panic.
	printTable(&sb, report)

	out := sb.String()
	if !strings.Contains(out, "minimal") {
		t.Error("expected 'minimal' in table output")
	}
	if !strings.Contains(out, "PASS") {
		t.Error("expected 'PASS' in table output")
	}
	if !strings.Contains(out, "FAIL") {
		t.Error("expected 'FAIL' in table output")
	}
}
