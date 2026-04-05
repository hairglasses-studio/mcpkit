//go:build !official_sdk

package mcptest

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/session"
)

// sessionEchoModule is a test module whose tool returns the session ID.
type sessionEchoModule struct{}

func (m *sessionEchoModule) Name() string        { return "session_echo" }
func (m *sessionEchoModule) Description() string { return "Session echo module for testing" }
func (m *sessionEchoModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		{
			Tool: mcp.Tool{
				Name:        "session_echo",
				Description: "Returns the session ID from context",
				InputSchema: mcp.ToolInputSchema{Type: "object"},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sess, ok := session.FromContext(ctx)
				if !ok {
					return handler.TextResult("no-session"), nil
				}
				return handler.TextResult(sess.ID()), nil
			},
			Category: "test",
		},
		{
			Tool: mcp.Tool{
				Name:        "session_set",
				Description: "Stores a value in the session",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"key":   map[string]any{"type": "string"},
						"value": map[string]any{"type": "string"},
					},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sess, ok := session.FromContext(ctx)
				if !ok {
					return handler.TextResult("no-session"), nil
				}
				key := handler.GetStringParam(req, "key")
				val := handler.GetStringParam(req, "value")
				sess.Set(key, val)
				return handler.TextResult(sess.ID()), nil
			},
			Category: "test",
		},
		{
			Tool: mcp.Tool{
				Name:        "session_get",
				Description: "Retrieves a value from the session",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"key": map[string]any{"type": "string"},
					},
				},
			},
			Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				sess, ok := session.FromContext(ctx)
				if !ok {
					return handler.TextResult("no-session"), nil
				}
				key := handler.GetStringParam(req, "key")
				val, exists := sess.Get(key)
				if !exists {
					return handler.TextResult("not-found"), nil
				}
				return handler.TextResult(val.(string)), nil
			},
			Category: "test",
		},
	}
}

// --- NewServerWithSessions ---

func TestNewServerWithSessions(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	if !srv.HasTool("session_echo") {
		t.Error("server should have session_echo tool")
	}
	if !srv.HasTool("session_set") {
		t.Error("server should have session_set tool")
	}
	if !srv.HasTool("session_get") {
		t.Error("server should have session_get tool")
	}
	if srv.Store == nil {
		t.Error("server Store should not be nil")
	}
}

func TestNewServerWithSessions_NoModules(t *testing.T) {
	srv := NewServerWithSessions(t)
	if len(srv.ToolNames()) != 0 {
		t.Errorf("expected 0 tools, got %d", len(srv.ToolNames()))
	}
}

// --- SessionClient ---

func TestNewSessionClient(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc := NewSessionClient(t, srv)

	if sc.SessionID() == "" {
		t.Fatal("session client should have a non-empty session ID")
	}
}

func TestSessionClient_CallTool_ReturnsSessionID(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc := NewSessionClient(t, srv)

	result := sc.CallTool("session_echo", nil)
	AssertNotError(t, result)

	id := GetSessionID(t, result)
	if id == "" {
		t.Fatal("expected non-empty session ID in result")
	}
	if id == "no-session" {
		t.Fatal("session middleware did not attach a session to the context")
	}
}

func TestSessionClient_SameSessionAcrossCalls(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc := NewSessionClient(t, srv)

	result1 := sc.CallTool("session_echo", nil)
	result2 := sc.CallTool("session_echo", nil)

	AssertSameSession(t, result1, result2)
}

func TestSessionClient_DifferentClients_DifferentSessions(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc1 := NewSessionClient(t, srv)
	sc2 := NewSessionClient(t, srv)

	result1 := sc1.CallTool("session_echo", nil)
	result2 := sc2.CallTool("session_echo", nil)

	AssertDifferentSession(t, result1, result2)
}

func TestSessionClient_SessionDataPersists(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc := NewSessionClient(t, srv)

	// Store a value
	sc.CallTool("session_set", map[string]any{
		"key":   "color",
		"value": "blue",
	})

	// Retrieve it in a subsequent call
	result := sc.CallTool("session_get", map[string]any{
		"key": "color",
	})
	AssertToolResult(t, result, "blue")
}

func TestSessionClient_CallToolE(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc := NewSessionClient(t, srv)

	result, err := sc.CallToolE("session_echo", nil)
	if err != nil {
		t.Fatalf("CallToolE: %v", err)
	}
	AssertNotError(t, result)

	id := GetSessionID(t, result)
	if id == "" || id == "no-session" {
		t.Fatal("expected valid session ID")
	}
}

// --- AssertSessionCreated ---

func TestAssertSessionCreated_Pass(t *testing.T) {
	result := registry.MakeTextResult("some-id")
	if runProbe(t, func(tb testing.TB) { AssertSessionCreated(tb, result) }) {
		t.Error("AssertSessionCreated should not fail on a valid result")
	}
}

func TestAssertSessionCreated_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertSessionCreated(tb, nil) }) {
		t.Error("AssertSessionCreated should fail on nil result")
	}
}

// --- AssertSameSession ---

func TestAssertSameSession_Pass(t *testing.T) {
	r1 := registry.MakeTextResult("session-abc")
	r2 := registry.MakeTextResult("session-abc")
	if runProbe(t, func(tb testing.TB) { AssertSameSession(tb, r1, r2) }) {
		t.Error("AssertSameSession should not fail when IDs match")
	}
}

func TestAssertSameSession_Fail(t *testing.T) {
	r1 := registry.MakeTextResult("session-abc")
	r2 := registry.MakeTextResult("session-xyz")
	if !runProbe(t, func(tb testing.TB) { AssertSameSession(tb, r1, r2) }) {
		t.Error("AssertSameSession should fail when IDs differ")
	}
}

func TestAssertSameSession_EmptyID1(t *testing.T) {
	r1 := &registry.CallToolResult{} // no content
	r2 := registry.MakeTextResult("session-abc")
	if !runProbe(t, func(tb testing.TB) { AssertSameSession(tb, r1, r2) }) {
		t.Error("AssertSameSession should fail when result1 has no session ID")
	}
}

func TestAssertSameSession_EmptyID2(t *testing.T) {
	r1 := registry.MakeTextResult("session-abc")
	r2 := &registry.CallToolResult{} // no content
	if !runProbe(t, func(tb testing.TB) { AssertSameSession(tb, r1, r2) }) {
		t.Error("AssertSameSession should fail when result2 has no session ID")
	}
}

// --- AssertDifferentSession ---

func TestAssertDifferentSession_Pass(t *testing.T) {
	r1 := registry.MakeTextResult("session-abc")
	r2 := registry.MakeTextResult("session-xyz")
	if runProbe(t, func(tb testing.TB) { AssertDifferentSession(tb, r1, r2) }) {
		t.Error("AssertDifferentSession should not fail when IDs differ")
	}
}

func TestAssertDifferentSession_Fail(t *testing.T) {
	r1 := registry.MakeTextResult("session-abc")
	r2 := registry.MakeTextResult("session-abc")
	if !runProbe(t, func(tb testing.TB) { AssertDifferentSession(tb, r1, r2) }) {
		t.Error("AssertDifferentSession should fail when IDs match")
	}
}

func TestAssertDifferentSession_EmptyID(t *testing.T) {
	r1 := &registry.CallToolResult{}
	r2 := registry.MakeTextResult("session-abc")
	if !runProbe(t, func(tb testing.TB) { AssertDifferentSession(tb, r1, r2) }) {
		t.Error("AssertDifferentSession should fail when result1 has no session ID")
	}
}

// --- GetSessionID ---

func TestGetSessionID_ValidResult(t *testing.T) {
	result := registry.MakeTextResult("my-session-id")
	id := GetSessionID(t, result)
	if id != "my-session-id" {
		t.Errorf("GetSessionID = %q, want %q", id, "my-session-id")
	}
}

func TestGetSessionID_EmptyContent(t *testing.T) {
	result := &registry.CallToolResult{}
	id := GetSessionID(t, result)
	if id != "" {
		t.Errorf("GetSessionID = %q, want empty", id)
	}
}

func TestGetSessionID_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { GetSessionID(tb, nil) }) {
		t.Error("GetSessionID should fail on nil result")
	}
}

// --- WithSessionID ---

func TestWithSessionID(t *testing.T) {
	ctx := WithSessionID(context.Background(), "test-id")
	id, ok := ctx.Value(sessionClientKey{}).(string)
	if !ok || id != "test-id" {
		t.Errorf("WithSessionID did not store the expected ID")
	}
}

// --- sessionMiddleware ---

func TestSessionMiddleware_AttachesSession(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc := NewSessionClient(t, srv)

	result := sc.CallTool("session_echo", nil)
	id := GetSessionID(t, result)
	if id == "" || id == "no-session" {
		t.Fatalf("session middleware should attach a session, got %q", id)
	}
}

func TestSessionMiddleware_SkipsIfAlreadyAttached(t *testing.T) {
	store := session.NewMemStore(session.Options{})
	defer store.Close()

	// Create a session and attach it to context before the middleware runs.
	sess, err := store.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	mw := sessionMiddleware(store)
	called := false
	inner := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		s, ok := session.FromContext(ctx)
		if !ok {
			t.Fatal("expected session in context")
		}
		// Should be the original session, not a new one.
		if s.ID() != sess.ID() {
			t.Errorf("session ID = %q, want %q", s.ID(), sess.ID())
		}
		return handler.TextResult(s.ID()), nil
	}

	wrapped := mw("test", registry.ToolDefinition{}, inner)

	ctx := session.WithSession(context.Background(), sess)
	_, err = wrapped(ctx, registry.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("inner handler was not called")
	}
}

// --- Integration: store tracks sessions ---

func TestSessionServer_StoreTracksCreatedSessions(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{})
	sc := NewSessionClient(t, srv)

	_ = sc.CallTool("session_echo", nil)

	// The store should have at least the client's pre-created session.
	if srv.Store.Len() == 0 {
		t.Error("store should contain at least one session")
	}
}

func TestSessionServer_MultipleModules(t *testing.T) {
	srv := NewServerWithSessions(t, &sessionEchoModule{}, &testModule{})
	if !srv.HasTool("session_echo") {
		t.Error("should have session_echo")
	}
	if !srv.HasTool("test_echo") {
		t.Error("should have test_echo")
	}
}
