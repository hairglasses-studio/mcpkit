package security

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// okResult is a minimal non-nil CallToolResult for use in tests.
func okResult() *registry.CallToolResult {
	return &registry.CallToolResult{}
}

func TestWithTenant_GetTenant_RoundTrip(t *testing.T) {
	t.Parallel()

	tc := TenantContext{
		TenantID:  "tenant-abc",
		UserID:    "user-123",
		AgentID:   "agent-xyz",
		SessionID: "sess-001",
	}

	ctx := WithTenant(context.Background(), tc)
	got, ok := GetTenant(ctx)

	if !ok {
		t.Fatal("GetTenant returned false, want true")
	}
	if got != tc {
		t.Errorf("GetTenant = %+v, want %+v", got, tc)
	}
}

func TestGetTenant_EmptyContext_ReturnsFalse(t *testing.T) {
	t.Parallel()

	_, ok := GetTenant(context.Background())
	if ok {
		t.Error("GetTenant on empty context returned true, want false")
	}
}

func TestGetTenant_ZeroValueContext_ReturnsFalse(t *testing.T) {
	t.Parallel()

	// A context carrying some other value should not yield a TenantContext.
	type otherKey struct{}
	ctx := context.WithValue(context.Background(), otherKey{}, "something")
	_, ok := GetTenant(ctx)
	if ok {
		t.Error("GetTenant returned true for context with unrelated value, want false")
	}
}

func TestTenantMiddleware_InjectsContext(t *testing.T) {
	t.Parallel()

	expected := TenantContext{
		TenantID:  "acme",
		UserID:    "u1",
		AgentID:   "a1",
		SessionID: "s1",
	}

	extractor := func(ctx context.Context, req registry.CallToolRequest) TenantContext {
		return expected
	}

	var gotCtx context.Context
	downstream := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		gotCtx = ctx
		return okResult(), nil
	}

	mw := TenantMiddleware(extractor)
	handler := mw("my-tool", registry.ToolDefinition{}, registry.ToolHandlerFunc(downstream))

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := GetTenant(gotCtx)
	if !ok {
		t.Fatal("GetTenant returned false in downstream handler, want true")
	}
	if got != expected {
		t.Errorf("downstream TenantContext = %+v, want %+v", got, expected)
	}
}

func TestTenantMiddleware_NilExtractor_PassesThrough(t *testing.T) {
	t.Parallel()

	called := false
	downstream := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		_, ok := GetTenant(ctx)
		if ok {
			t.Error("GetTenant returned true with nil extractor, want false")
		}
		return okResult(), nil
	}

	mw := TenantMiddleware(nil)
	handler := mw("my-tool", registry.ToolDefinition{}, registry.ToolHandlerFunc(downstream))

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("downstream handler was not called")
	}
}

func TestTenantMiddleware_EmptyTenantContext(t *testing.T) {
	t.Parallel()

	// An extractor that returns a zero-value TenantContext should still inject it.
	extractor := func(ctx context.Context, req registry.CallToolRequest) TenantContext {
		return TenantContext{}
	}

	var gotCtx context.Context
	downstream := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		gotCtx = ctx
		return okResult(), nil
	}

	mw := TenantMiddleware(extractor)
	handler := mw("my-tool", registry.ToolDefinition{}, registry.ToolHandlerFunc(downstream))

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := GetTenant(gotCtx)
	if !ok {
		t.Fatal("GetTenant returned false for zero-value TenantContext, want true")
	}
	if got != (TenantContext{}) {
		t.Errorf("unexpected TenantContext: %+v", got)
	}
}

func TestTenantMiddleware_Integration_DownstreamReadsFields(t *testing.T) {
	t.Parallel()

	// Simulate a real scenario: extractor reads from incoming context (e.g., from
	// auth middleware that set a subject), builds TenantContext, downstream reads it.
	type authKey struct{}
	baseCtx := context.WithValue(context.Background(), authKey{}, "tenant-42|user-7")

	extractor := func(ctx context.Context, req registry.CallToolRequest) TenantContext {
		raw, _ := ctx.Value(authKey{}).(string)
		// Naively split for test purposes.
		tenantID, userID := "", ""
		for i, ch := range raw {
			if ch == '|' {
				tenantID = raw[:i]
				userID = raw[i+1:]
				break
			}
		}
		return TenantContext{TenantID: tenantID, UserID: userID}
	}

	var tenantID, userID string
	downstream := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		tc, ok := GetTenant(ctx)
		if !ok {
			t.Error("GetTenant returned false in integration downstream")
		}
		tenantID = tc.TenantID
		userID = tc.UserID
		return okResult(), nil
	}

	mw := TenantMiddleware(extractor)
	handler := mw("my-tool", registry.ToolDefinition{}, registry.ToolHandlerFunc(downstream))

	_, err := handler(baseCtx, registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenantID != "tenant-42" {
		t.Errorf("tenantID = %q, want %q", tenantID, "tenant-42")
	}
	if userID != "user-7" {
		t.Errorf("userID = %q, want %q", userID, "user-7")
	}
}
