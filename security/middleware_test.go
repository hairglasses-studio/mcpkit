//go:build !official_sdk

package security

import (
	"context"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// echoHandler is a minimal handler that returns text content without error.
func echoHandler(text string) registry.ToolHandlerFunc {
	return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult(text), nil
	}
}

func TestAuditMiddleware_LogsCallAndResult(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	mw := AuditMiddleware(logger, nil)
	handler := mw("my_tool", registry.ToolDefinition{}, echoHandler("ok"))

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := logger.GetRecentEvents(10)
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events (call + result), got %d", len(events))
	}
	if events[0].Type != AuditToolCall {
		t.Errorf("first event type = %q, want %q", events[0].Type, AuditToolCall)
	}
	if events[1].Type != AuditToolSuccess {
		t.Errorf("second event type = %q, want %q", events[1].Type, AuditToolSuccess)
	}
	if events[0].Tool != "my_tool" {
		t.Errorf("tool = %q, want my_tool", events[0].Tool)
	}
}

func TestAuditMiddleware_NilUserFunc(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	mw := AuditMiddleware(logger, nil)
	handler := mw("tool_a", registry.ToolDefinition{}, echoHandler("hi"))

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := logger.GetRecentEvents(10)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].User != "" {
		t.Errorf("user = %q, want empty string when no userFunc set", events[0].User)
	}
}

func TestAuditMiddleware_UserFuncPopulatesUser(t *testing.T) {
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	userFunc := func(ctx context.Context) string { return "alice" }
	mw := AuditMiddleware(logger, userFunc)
	handler := mw("tool_b", registry.ToolDefinition{}, echoHandler("hello"))

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := logger.GetRecentEvents(10)
	if len(events) == 0 {
		t.Fatal("expected events")
	}
	if events[0].User != "alice" {
		t.Errorf("user = %q, want alice", events[0].User)
	}
}

func TestRBACMiddleware_AllowsAccess(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"admin": {RoleAdmin},
		},
	})
	// Override tool permission so _delete requires PermDelete for clarity.
	rbac.SetToolPermission("my_delete", PermDelete)

	userFunc := func(ctx context.Context) string { return "admin" }
	mw := RBACMiddleware(rbac, nil, userFunc)

	called := false
	downstream := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("deleted"), nil
	}

	handler := mw("my_delete", registry.ToolDefinition{}, registry.ToolHandlerFunc(downstream))
	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("downstream handler should have been called for admin user")
	}
	if result == nil || result.IsError {
		t.Errorf("expected non-error result for allowed access, got %+v", result)
	}
}

func TestRBACMiddleware_DeniesAccess(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"viewer": {RoleReadonly},
		},
	})

	userFunc := func(ctx context.Context) string { return "viewer" }
	mw := RBACMiddleware(rbac, nil, userFunc)

	called := false
	downstream := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		called = true
		return registry.MakeTextResult("deleted"), nil
	}

	handler := mw("my_delete", registry.ToolDefinition{}, registry.ToolHandlerFunc(downstream))
	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("downstream should NOT be called when access is denied")
	}
	if result == nil || !result.IsError {
		t.Error("expected error result for denied access")
	}
	// Verify the error message mentions PERMISSION_DENIED.
	if len(result.Content) == 0 {
		t.Fatal("expected content in error result")
	}
	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content in error result")
	}
	if !strings.Contains(text, "PERMISSION_DENIED") {
		t.Errorf("error text should contain PERMISSION_DENIED, got: %s", text)
	}
}

func TestRBACMiddleware_LogsAccessDenied(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"viewer": {RoleReadonly},
		},
	})
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	userFunc := func(ctx context.Context) string { return "viewer" }
	mw := RBACMiddleware(rbac, logger, userFunc)
	handler := mw("secret_delete", registry.ToolDefinition{}, echoHandler("done"))

	_, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := logger.GetRecentEvents(10)
	if len(events) == 0 {
		t.Fatal("expected at least one audit event")
	}
	found := false
	for _, e := range events {
		if e.Type == AuditAccessDeny {
			found = true
			if e.User != "viewer" {
				t.Errorf("access deny event user = %q, want viewer", e.User)
			}
			if e.Tool != "secret_delete" {
				t.Errorf("access deny event tool = %q, want secret_delete", e.Tool)
			}
		}
	}
	if !found {
		t.Error("expected an AuditAccessDeny event to be logged")
	}
}

func TestRBACMiddleware_NilLogger(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"viewer": {RoleReadonly},
		},
	})

	userFunc := func(ctx context.Context) string { return "viewer" }
	// nil logger should not panic even on access denied
	mw := RBACMiddleware(rbac, nil, userFunc)
	handler := mw("my_delete", registry.ToolDefinition{}, echoHandler("ok"))

	// Should not panic.
	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result when access denied")
	}
}

func TestMiddlewareStack_RBACThenAudit(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"admin": {RoleAdmin},
		},
	})
	logger := NewAuditLogger(AuditLoggerConfig{MaxEvents: 100})

	userFunc := func(ctx context.Context) string { return "admin" }

	// Chain: RBAC outer, audit inner (RBAC runs first, then audit wraps next).
	rbacMW := RBACMiddleware(rbac, logger, userFunc)
	auditMW := AuditMiddleware(logger, userFunc)

	// Build chain: rbacMW -> auditMW -> downstream
	downstream := echoHandler("success")
	auditWrapped := auditMW("my_list", registry.ToolDefinition{}, downstream)
	fullHandler := rbacMW("my_list", registry.ToolDefinition{}, registry.ToolHandlerFunc(auditWrapped))

	result, err := fullHandler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected successful result from stacked middleware, got %+v", result)
	}

	// Audit should have recorded call + success from AuditMiddleware.
	events := logger.GetRecentEvents(10)
	hasCall := false
	hasSuccess := false
	for _, e := range events {
		if e.Type == AuditToolCall {
			hasCall = true
		}
		if e.Type == AuditToolSuccess {
			hasSuccess = true
		}
	}
	if !hasCall {
		t.Error("expected AuditToolCall event in stacked middleware run")
	}
	if !hasSuccess {
		t.Error("expected AuditToolSuccess event in stacked middleware run")
	}
}
