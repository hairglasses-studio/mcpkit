//go:build !official_sdk

package security_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/security"
)

// ExampleNewRBAC demonstrates creating an RBAC instance, adding a role, and
// checking tool access.
func ExampleNewRBAC() {
	rbac := security.NewRBAC(security.RBACConfig{
		UserRoles: map[string][]security.Role{
			"alice": {security.RoleAdmin},
			"bob":   {security.RoleReadonly},
		},
	})

	fmt.Println(rbac.CanAccessTool("alice", "my_delete")) // admin can delete
	fmt.Println(rbac.CanAccessTool("bob", "my_delete"))   // readonly cannot delete
	fmt.Println(rbac.CanAccessTool("bob", "my_list"))     // readonly can list
	// Output:
	// true
	// false
	// true
}

// ExampleAuditMiddleware demonstrates wrapping a tool handler with AuditMiddleware
// and verifying that events are recorded.
func ExampleAuditMiddleware() {
	logger := security.NewAuditLogger(security.AuditLoggerConfig{MaxEvents: 100})

	userFunc := func(ctx context.Context) string { return "alice" }
	mw := security.AuditMiddleware(logger, userFunc)

	// Simple handler that echoes back a greeting.
	downstream := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("hello"), nil
	}

	handler := mw("greet", registry.ToolDefinition{}, registry.ToolHandlerFunc(downstream))
	_, _ = handler(context.Background(), registry.CallToolRequest{})

	events := logger.GetRecentEvents(10)
	fmt.Println(len(events)) // tool_call + tool_success
	// Output:
	// 2
}

// ExampleNewAuditLogger_withExporter demonstrates using a JSONLExporter to capture
// audit events as newline-delimited JSON.
func ExampleNewAuditLogger_withExporter() {
	var buf bytes.Buffer
	exp := security.NewJSONLExporter(&buf)

	logger := security.NewAuditLogger(security.AuditLoggerConfig{
		MaxEvents: 100,
		Exporters: []security.AuditExporter{exp},
	})

	logger.LogToolCall("alice", "my_list", nil)
	logger.LogToolCall("bob", "my_get", nil)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	fmt.Println(len(lines))
	// Output:
	// 2
}
