package mcptest

import "testing"

// TestClient_ListToolNames exercises the round 5 (P52.6/P52.7) addition:
// Client.ListToolNames/ListResourceURIs/ListPromptNames, the list-surface
// gap that blocked porting secretstudios-mcp's
// TestMCPProtocolFrontdoorsListSurfaces/TestAutonomyProtocolSurfaceIsActuallyNarrowed
// off mcp-go's client.NewInProcessClient. setupTestServerWithAll registers
// an identical fixture (2 tools, 1 resource, 2 prompts) on both build tags.
func TestClient_ListToolNames(t *testing.T) {
	_, c := setupTestServerWithAll(t)

	names := c.ListToolNames()
	if len(names) != 2 {
		t.Fatalf("len(ListToolNames) = %d, want 2: %v", len(names), names)
	}
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	if !set["test_echo"] || !set["test_error"] {
		t.Errorf("unexpected tool names: %v", names)
	}
}

func TestClient_ListResourceURIs(t *testing.T) {
	_, c := setupTestServerWithAll(t)

	uris := c.ListResourceURIs()
	if len(uris) != 1 {
		t.Fatalf("len(ListResourceURIs) = %d, want 1: %v", len(uris), uris)
	}
	if uris[0] != "test://greeting" {
		t.Errorf("uris[0] = %q, want test://greeting", uris[0])
	}
}

func TestClient_ListPromptNames(t *testing.T) {
	_, c := setupTestServerWithAll(t)

	names := c.ListPromptNames()
	if len(names) != 2 {
		t.Fatalf("len(ListPromptNames) = %d, want 2: %v", len(names), names)
	}
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	if !set["greeting"] || !set["personalized"] {
		t.Errorf("unexpected prompt names: %v", names)
	}
}
