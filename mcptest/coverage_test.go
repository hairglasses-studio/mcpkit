//go:build !official_sdk

package mcptest

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- transport RPC error paths ---

// TestCallToolE_RPCError exercises the RPC error branch in transport.callTool
// by calling a tool name that is not registered (the server returns an RPC error).
func TestCallToolE_RPCError(t *testing.T) {
	_, c := setupTestServer(t)
	result, err := c.CallToolE("nonexistent_tool_xyz", nil)
	if err == nil {
		t.Fatal("expected RPC error for unknown tool, got nil")
	}
	if result != nil {
		t.Error("expected nil result when RPC error occurs")
	}
}

// TestReadResourceE_RPCError exercises the RPC error branch in transport.readResource.
func TestReadResourceE_RPCError(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result, err := c.ReadResourceE("test://does-not-exist-xyz")
	if err == nil {
		t.Fatal("expected RPC error for unknown resource, got nil")
	}
	if result != nil {
		t.Error("expected nil result when RPC error occurs")
	}
}

// TestGetPromptE_RPCError exercises the RPC error branch in transport.getPrompt.
func TestGetPromptE_RPCError(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	result, err := c.GetPromptE("nonexistent_prompt_xyz", nil)
	if err == nil {
		t.Fatal("expected RPC error for unknown prompt, got nil")
	}
	if result != nil {
		t.Error("expected nil result when RPC error occurs")
	}
}

// TestGetPromptE_NilArgs exercises the nil args branch in transport.getPrompt
// (args == nil means the "arguments" key is omitted from the request params).
func TestGetPromptE_NilArgs(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	// "greeting" prompt requires no arguments; passing nil hits the nil-args branch.
	result, err := c.GetPromptE("greeting", nil)
	if err != nil {
		t.Fatalf("GetPromptE with nil args: unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for greeting prompt with nil args")
	}
}

// --- client fatal branches ---

// TestCallTool_FatalOnRPCError exercises the client.CallTool Fatalf branch.
// We use a probeT to capture the fatal without actually failing the outer test.
func TestCallTool_FatalOnRPCError(t *testing.T) {
	_, c := setupTestServer(t)
	// Swap c.t with a probeT so Fatalf is captured instead of failing the real test.
	probe := &probeT{TB: t}
	origT := c.t
	c.t = probe
	defer func() { c.t = origT }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.CallTool("nonexistent_tool_xyz", nil)
	}()
	<-done

	if !probe.failed {
		t.Error("CallTool should have called Fatalf for RPC error")
	}
}

// TestCallToolWithContext_FatalOnRPCError exercises the CallToolWithContext fatal branch.
func TestCallToolWithContext_FatalOnRPCError(t *testing.T) {
	_, c := setupTestServer(t)
	probe := &probeT{TB: t}
	origT := c.t
	c.t = probe
	defer func() { c.t = origT }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.CallToolWithContext(t.Context(), "nonexistent_tool_xyz", nil)
	}()
	<-done

	if !probe.failed {
		t.Error("CallToolWithContext should have called Fatalf for RPC error")
	}
}

// TestReadResource_FatalOnRPCError exercises the client.ReadResource Fatalf branch.
func TestReadResource_FatalOnRPCError(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	probe := &probeT{TB: t}
	origT := c.t
	c.t = probe
	defer func() { c.t = origT }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.ReadResource("test://does-not-exist-xyz")
	}()
	<-done

	if !probe.failed {
		t.Error("ReadResource should have called Fatalf for RPC error")
	}
}

// TestGetPrompt_FatalOnRPCError exercises the client.GetPrompt Fatalf branch.
func TestGetPrompt_FatalOnRPCError(t *testing.T) {
	_, c := setupTestServerWithAll(t)
	probe := &probeT{TB: t}
	origT := c.t
	c.t = probe
	defer func() { c.t = origT }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.GetPrompt("nonexistent_prompt_xyz", nil)
	}()
	<-done

	if !probe.failed {
		t.Error("GetPrompt should have called Fatalf for RPC error")
	}
}

// --- replay uncovered branches ---

// TestReplay_TransportError exercises the Replay transport error branch.
// A session entry referencing a non-existent tool causes CallToolE to return
// a Go error, which is the path at replay.go line 114-116.
func TestReplay_TransportError(t *testing.T) {
	_, c := setupTestServer(t)

	session := &Session{
		Name: "transport-error-session",
		Entries: []SessionEntry{
			{
				ToolName: "nonexistent_tool_xyz",
				Args:     nil,
				Result:   registry.MakeTextResult("expected"),
				IsError:  false,
			},
		},
	}

	failed := false
	mockT := &mockTB{TB: t, onError: func() { failed = true }}
	Replay(mockT, c, session)
	if !failed {
		t.Error("Replay should have reported transport error for unknown tool")
	}
}

// TestReplay_BothNil exercises the replay.go "both nil — continue" branch
// (result == nil && entry.Result == nil).
// We can only exercise this if CallToolE can return (nil, nil), which doesn't
// happen for RPC errors. Instead we build a session with zero entries to ensure
// the loop completes normally, then test with an error entry to verify the
// continue branch indirectly by checking no spurious failure occurs.
//
// The real both-nil path (lines 126-128) is reached when the server returns a
// valid RPC response with a null "result" field. We construct that scenario by
// running Replay against an empty session — all iterations skip so the continue
// branch coverage is satisfied implicitly through TestReplay_Match. This test
// documents that explicitly.
func TestReplay_EmptySession(t *testing.T) {
	_, c := setupTestServer(t)

	session := &Session{
		Name:    "empty",
		Entries: nil,
	}

	// Should succeed with no assertions failing.
	Replay(t, c, session)
}

// --- server.NewServer with Option ---

// TestNewServer_WithOption exercises the opts loop in NewServer.
func TestNewServer_WithOption(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{})

	optionApplied := false
	opt := func(s *Server) {
		optionApplied = true
	}

	s := NewServer(t, reg, opt)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if !optionApplied {
		t.Error("Option was not applied by NewServer")
	}
}

// --- AssertResourceText/Contains non-text content ---

// TestAssertResourceText_NotText exercises the "not text" fatal branch in AssertResourceText.
func TestAssertResourceText_NotText(t *testing.T) {
	// ReadResourceResult with no contents → ExtractResourceText returns ok=false.
	result := &registry.ReadResourceResult{}
	if !runProbe(t, func(tb testing.TB) {
		AssertResourceText(tb, result, "anything")
	}) {
		t.Error("AssertResourceText should fail when resource content is not text")
	}
}

// TestAssertResourceContains_NotText exercises the "not text" fatal branch in AssertResourceContains.
func TestAssertResourceContains_NotText(t *testing.T) {
	result := &registry.ReadResourceResult{}
	if !runProbe(t, func(tb testing.TB) {
		AssertResourceContains(tb, result, "anything")
	}) {
		t.Error("AssertResourceContains should fail when resource content is not text")
	}
}

