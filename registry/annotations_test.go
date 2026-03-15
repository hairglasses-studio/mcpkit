//go:build !official_sdk

package registry

import "testing"

// TestInferIsWrite_WriteSuffixes verifies that all documented write suffixes are detected.
func TestInferIsWrite_WriteSuffixes(t *testing.T) {
	writeCases := []string{
		"item_create",
		"item_delete",
		"item_remove",
		"item_reset",
		"item_send",
		"item_post",
		"item_update",
		"item_set",
		"item_add",
		"item_apply",
		"item_import",
		"item_publish",
		"item_start",
		"item_stop",
		"item_restart",
		"item_trigger",
		"item_execute",
		"item_run",
		"item_record",
		"item_assign",
		"item_unassign",
		"item_move",
		"item_copy",
		"item_rename",
		"item_enable",
		"item_disable",
		"item_clear",
		"item_flush",
		"item_purge",
		"item_archive",
		"item_restore",
		"item_sync",
		"item_push",
		"item_deploy",
		"item_install",
		"item_uninstall",
		"item_register",
		"item_deregister",
		"item_subscribe",
		"item_unsubscribe",
		"item_approve",
		"item_reject",
		"item_resolve",
		"item_close",
		"item_reopen",
	}

	for _, name := range writeCases {
		if !InferIsWrite(name) {
			t.Errorf("InferIsWrite(%q) = false, want true", name)
		}
	}
}

// TestInferIsWrite_ReadSuffixes verifies that common read-only names are NOT detected as writes.
func TestInferIsWrite_ReadSuffixes(t *testing.T) {
	readCases := []string{
		"item_list",
		"item_get",
		"item_fetch",
		"item_search",
		"item_find",
		"item_status",
		"item_check",
		"item_view",
		"item_show",
		"item_describe",
		"item_read",
		"item_count",
	}

	for _, name := range readCases {
		if InferIsWrite(name) {
			t.Errorf("InferIsWrite(%q) = true, want false", name)
		}
	}
}

// TestInferIsWrite_CaseInsensitive verifies that matching is case-insensitive.
func TestInferIsWrite_CaseInsensitive(t *testing.T) {
	cases := []string{
		"ITEM_CREATE",
		"Item_Delete",
		"ITEM_UPDATE",
		"MyTool_SEND",
	}
	for _, name := range cases {
		if !InferIsWrite(name) {
			t.Errorf("InferIsWrite(%q) should be case-insensitive and return true", name)
		}
	}
}

// TestInferIsWrite_SuffixOnly ensures partial matches in the middle of the name don't trigger.
func TestInferIsWrite_SuffixOnly(t *testing.T) {
	// "create" appears in the middle, not as a suffix — should not be inferred as write
	// Note: "item_create_sub" ends in "_sub" not "_create", so should be false
	if InferIsWrite("item_create_sub") {
		t.Error("InferIsWrite(\"item_create_sub\") should be false — _create is not a suffix here")
	}
}

// TestApplyMCPAnnotations_ReadOnlyTool verifies annotations for a non-write tool.
func TestApplyMCPAnnotations_ReadOnlyTool(t *testing.T) {
	td := ToolDefinition{
		Tool:    Tool{Name: "myapp_items_list"},
		IsWrite: false,
	}

	annotated := ApplyMCPAnnotations(td, "myapp_")

	assertReadOnlyHint(t, annotated, true)
	assertDestructiveHint(t, annotated, false)
	assertIdempotentHint(t, annotated, true)
	assertOpenWorldHint(t, annotated, true)

	if annotationTitle(annotated) != "Items List" {
		t.Errorf("title = %q, want 'Items List'", annotationTitle(annotated))
	}
}

// TestApplyMCPAnnotations_WriteTool_NonDestructive verifies annotations for a non-destructive write tool.
func TestApplyMCPAnnotations_WriteTool_NonDestructive(t *testing.T) {
	td := ToolDefinition{
		Tool:    Tool{Name: "myapp_items_create"},
		IsWrite: true,
	}

	annotated := ApplyMCPAnnotations(td, "myapp_")

	assertReadOnlyHint(t, annotated, false)
	assertDestructiveHint(t, annotated, false)
	assertIdempotentHint(t, annotated, false)
}

// TestApplyMCPAnnotations_WriteTool_Destructive verifies annotations for a destructive write tool.
func TestApplyMCPAnnotations_WriteTool_Destructive(t *testing.T) {
	destructiveCases := []string{
		"myapp_items_delete",
		"myapp_items_remove",
		"myapp_items_reset",
		"myapp_items_purge",
		"myapp_items_clear",
		"myapp_items_flush",
	}

	for _, name := range destructiveCases {
		td := ToolDefinition{
			Tool:    Tool{Name: name},
			IsWrite: true,
		}
		annotated := ApplyMCPAnnotations(td, "myapp_")
		assertDestructiveHint(t, annotated, true)
	}
}

// TestApplyMCPAnnotations_WriteTool_Idempotent verifies annotations for idempotent write tools.
func TestApplyMCPAnnotations_WriteTool_Idempotent(t *testing.T) {
	idempotentCases := []string{
		"myapp_config_set",
		"myapp_config_update",
		"myapp_data_sync",
		"myapp_feature_enable",
		"myapp_feature_disable",
		"myapp_user_assign",
	}

	for _, name := range idempotentCases {
		td := ToolDefinition{
			Tool:    Tool{Name: name},
			IsWrite: true,
		}
		annotated := ApplyMCPAnnotations(td, "myapp_")
		assertIdempotentHint(t, annotated, true)
	}
}

// TestApplyMCPAnnotations_OpenWorldAlwaysTrue verifies that OpenWorldHint is always true.
func TestApplyMCPAnnotations_OpenWorldAlwaysTrue(t *testing.T) {
	cases := []struct {
		name    string
		isWrite bool
	}{
		{"myapp_items_list", false},
		{"myapp_items_create", true},
		{"myapp_items_delete", true},
	}

	for _, c := range cases {
		td := ToolDefinition{
			Tool:    Tool{Name: c.name},
			IsWrite: c.isWrite,
		}
		annotated := ApplyMCPAnnotations(td, "myapp_")
		assertOpenWorldHint(t, annotated, true)
	}
}

// TestApplyMCPAnnotations_TitleGenerationNoPrefix verifies title generation with no prefix.
func TestApplyMCPAnnotations_TitleGenerationNoPrefix(t *testing.T) {
	td := ToolDefinition{
		Tool:    Tool{Name: "send_message"},
		IsWrite: true,
	}

	annotated := ApplyMCPAnnotations(td, "")
	if annotationTitle(annotated) != "Send Message" {
		t.Errorf("title = %q, want 'Send Message'", annotationTitle(annotated))
	}
}

// TestApplyMCPAnnotations_TitleGenerationWithPrefix verifies prefix stripping in title.
func TestApplyMCPAnnotations_TitleGenerationWithPrefix(t *testing.T) {
	cases := []struct {
		name      string
		prefix    string
		wantTitle string
	}{
		{"myapp_gmail_send", "myapp_", "Gmail Send"},
		{"myapp_inventory_list", "myapp_", "Inventory List"},
		{"svc_items_delete", "svc_", "Items Delete"},
		{"no_prefix_tool", "", "No Prefix Tool"},
	}

	for _, c := range cases {
		td := ToolDefinition{
			Tool: Tool{Name: c.name},
		}
		annotated := ApplyMCPAnnotations(td, c.prefix)
		got := annotationTitle(annotated)
		if got != c.wantTitle {
			t.Errorf("ApplyMCPAnnotations(%q, %q) title = %q, want %q", c.name, c.prefix, got, c.wantTitle)
		}
	}
}

// TestApplyMCPAnnotations_AllHintsSet verifies that all hint pointers are non-nil.
func TestApplyMCPAnnotations_AllHintsSet(t *testing.T) {
	td := ToolDefinition{
		Tool:    Tool{Name: "my_tool"},
		IsWrite: false,
	}
	annotated := ApplyMCPAnnotations(td, "")

	if annotated.Tool.Annotations.ReadOnlyHint == nil {
		t.Error("ReadOnlyHint should not be nil")
	}
	if annotated.Tool.Annotations.DestructiveHint == nil {
		t.Error("DestructiveHint should not be nil")
	}
	if annotated.Tool.Annotations.IdempotentHint == nil {
		t.Error("IdempotentHint should not be nil")
	}
	if annotated.Tool.Annotations.OpenWorldHint == nil {
		t.Error("OpenWorldHint should not be nil")
	}
}

// assertIdempotentHint checks that the tool's IdempotentHint annotation matches expected.
func assertIdempotentHint(t *testing.T, td ToolDefinition, expected bool) {
	t.Helper()
	hint := td.Tool.Annotations.IdempotentHint
	if hint == nil {
		t.Errorf("IdempotentHint is nil, want %v", expected)
		return
	}
	if *hint != expected {
		t.Errorf("IdempotentHint = %v, want %v for tool %q", *hint, expected, td.Tool.Name)
	}
}

// assertOpenWorldHint checks that the tool's OpenWorldHint annotation matches expected.
func assertOpenWorldHint(t *testing.T, td ToolDefinition, expected bool) {
	t.Helper()
	hint := td.Tool.Annotations.OpenWorldHint
	if hint == nil {
		t.Errorf("OpenWorldHint is nil, want %v", expected)
		return
	}
	if *hint != expected {
		t.Errorf("OpenWorldHint = %v, want %v for tool %q", *hint, expected, td.Tool.Name)
	}
}
