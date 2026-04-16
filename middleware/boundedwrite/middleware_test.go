//go:build !official_sdk

package boundedwrite

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// helpers

func okHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("ok"), nil
}

func makeReqWithArgs(args map[string]any) registry.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func makeReqNoArgs() registry.CallToolRequest {
	return mcp.CallToolRequest{}
}

func applyMiddleware(td registry.ToolDefinition) registry.ToolHandlerFunc {
	mw := Middleware()
	// Use the tool's own Name so rejection messages are accurate in tests.
	name := td.Tool.Name
	if name == "" {
		name = "test_tool"
	}
	return mw(name, td, okHandler)
}

// --- tests ---

// TestNoConfirmRequired_PassesThrough verifies that tools without ConfirmTag
// are completely transparent — the middleware is a no-op.
func TestNoConfirmRequired_PassesThrough(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("safe_read", mcp.WithDescription("Read something")),
		Tags: []string{"read", "safe"},
	}

	handler := applyMiddleware(td)
	result, err := handler(context.Background(), makeReqNoArgs())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if registry.IsResultError(result) {
		t.Error("expected success result for non-confirmation-required tool")
	}
}

// TestConfirmRequired_MissingParam_Rejects verifies that when confirm is absent
// the middleware returns an error result with a human-readable prompt.
func TestConfirmRequired_MissingParam_Rejects(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("payment_charge", mcp.WithDescription("Charge a payment card")),
		Tags: []string{ConfirmTag},
	}

	handler := applyMiddleware(td)
	result, err := handler(context.Background(), makeReqNoArgs())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result when confirm param is absent")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "CONFIRM_REQUIRED") {
		t.Errorf("rejection message missing CONFIRM_REQUIRED marker, got: %q", text)
	}
	if !strings.Contains(text, "payment_charge") {
		t.Errorf("rejection message should mention the tool name, got: %q", text)
	}
	if !strings.Contains(text, "confirm: true") {
		t.Errorf("rejection message should include confirm: true instruction, got: %q", text)
	}
}

// TestConfirmRequired_ConfirmTrue_PassesThrough verifies that confirm=true
// causes the middleware to call the next handler.
func TestConfirmRequired_ConfirmTrue_PassesThrough(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("payment_charge", mcp.WithDescription("Charge a payment card")),
		Tags: []string{ConfirmTag},
	}

	handler := applyMiddleware(td)
	result, err := handler(context.Background(), makeReqWithArgs(map[string]any{
		"confirm": true,
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if registry.IsResultError(result) {
		t.Errorf("expected success result when confirm=true, got error: %v", extractText(t, result))
	}
}

// TestConfirmRequired_ConfirmFalse_Rejects verifies that explicitly setting
// confirm=false is treated the same as absent.
func TestConfirmRequired_ConfirmFalse_Rejects(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("delete_account", mcp.WithDescription("Permanently delete an account")),
		Tags: []string{ConfirmTag},
	}

	handler := applyMiddleware(td)
	result, err := handler(context.Background(), makeReqWithArgs(map[string]any{
		"confirm": false,
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result when confirm=false")
	}
}

// TestConfirmRequired_EmptyTags_PassesThrough verifies empty Tags slice is treated
// as no confirmation required.
func TestConfirmRequired_EmptyTags_PassesThrough(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("info_get", mcp.WithDescription("Get some info")),
		Tags: []string{},
	}

	handler := applyMiddleware(td)
	result, err := handler(context.Background(), makeReqNoArgs())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success for tool with empty tags")
	}
}

// TestConfirmRequired_OtherTagsOnly_PassesThrough verifies that unrelated tags
// don't trigger the confirmation gate.
func TestConfirmRequired_OtherTagsOnly_PassesThrough(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("data_export", mcp.WithDescription("Export data")),
		Tags: []string{"export", "data", "write"},
	}

	handler := applyMiddleware(td)
	result, err := handler(context.Background(), makeReqNoArgs())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Error("expected success for tool with unrelated tags")
	}
}

// TestRejectionMessage_IncludesDescription verifies the rejection message
// includes the tool description so callers understand what they are confirming.
func TestRejectionMessage_IncludesDescription(t *testing.T) {
	desc := "Permanently delete all user data including billing history"
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("nuke_account", mcp.WithDescription(desc)),
		Tags: []string{ConfirmTag},
	}

	handler := applyMiddleware(td)
	result, err := handler(context.Background(), makeReqNoArgs())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(t, result)
	if !strings.Contains(text, desc) {
		t.Errorf("rejection message should include tool description %q, got: %q", desc, text)
	}
}

// TestRequireConfirmation_AddsTag verifies the helper function adds ConfirmTag.
func TestRequireConfirmation_AddsTag(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("charge", mcp.WithDescription("Charge")),
		Tags: []string{"financial"},
	}

	out := RequireConfirmation(td)

	found := false
	for _, tag := range out.Tags {
		if tag == ConfirmTag {
			found = true
		}
	}
	if !found {
		t.Errorf("RequireConfirmation did not add %q tag; tags: %v", ConfirmTag, out.Tags)
	}
	// Original tags must be preserved.
	if len(out.Tags) < 2 {
		t.Errorf("RequireConfirmation dropped existing tags; tags: %v", out.Tags)
	}
}

// TestRequireConfirmation_Idempotent verifies the helper does not add duplicate tags.
func TestRequireConfirmation_Idempotent(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("charge", mcp.WithDescription("Charge")),
		Tags: []string{ConfirmTag},
	}

	out := RequireConfirmation(td)

	count := 0
	for _, tag := range out.Tags {
		if tag == ConfirmTag {
			count++
		}
	}
	if count != 1 {
		t.Errorf("RequireConfirmation should be idempotent; found %d %q tags", count, ConfirmTag)
	}
}

// TestRequireConfirmation_NilTags verifies nil Tags slice is handled safely.
func TestRequireConfirmation_NilTags(t *testing.T) {
	td := registry.ToolDefinition{
		Tool: mcp.NewTool("charge", mcp.WithDescription("Charge")),
	}

	out := RequireConfirmation(td)

	found := false
	for _, tag := range out.Tags {
		if tag == ConfirmTag {
			found = true
		}
	}
	if !found {
		t.Errorf("RequireConfirmation should add tag even when Tags was nil; tags: %v", out.Tags)
	}
}

// --- helpers ---

func extractText(t *testing.T, result *registry.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	for _, c := range result.Content {
		if text, ok := registry.ExtractTextContent(c); ok {
			return text
		}
	}
	return ""
}
