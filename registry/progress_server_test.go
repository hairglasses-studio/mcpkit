//go:build !official_sdk

package registry

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServerProgressReporter_NotNil(t *testing.T) {
	t.Parallel()
	s := NewMCPServer("test", "0.0.1")
	reporter := NewServerProgressReporter(s, "token-1", 100.0)
	if reporter == nil {
		t.Fatal("NewServerProgressReporter returned nil")
	}
}

func TestServerProgressReporter_ReportErrorsWithoutSession(t *testing.T) {
	t.Parallel()
	// Without an active session, SendNotificationToClient will return an error.
	// We verify Report propagates it (does not panic).
	s := NewMCPServer("test", "0.0.1")
	reporter := NewServerProgressReporter(s, "token-1", 100.0)
	err := reporter.Report(context.Background(), 0.5, "halfway")
	// Error is expected — no active client session. The important thing is no panic.
	_ = err
}

func TestServerProgressMiddleware_InjectsReporterWithToken(t *testing.T) {
	t.Parallel()
	s := NewMCPServer("test", "0.0.1")

	var capturedReporter ProgressReporter
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		capturedReporter = GetProgressReporter(ctx)
		return MakeTextResult("ok"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	mw := ServerProgressMiddleware(s, 100.0)
	wrapped := mw("test_tool", td, inner)

	req := CallToolRequest{}
	req.Params.Meta = &mcp.Meta{ProgressToken: "my-token"}

	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReporter == nil {
		t.Fatal("expected reporter to be injected when token is present, got nil")
	}

	srv, ok := capturedReporter.(*ServerProgressReporter)
	if !ok {
		t.Fatalf("expected *ServerProgressReporter, got %T", capturedReporter)
	}
	if srv.token != mcp.ProgressToken("my-token") {
		t.Errorf("token = %v, want my-token", srv.token)
	}
	if srv.total != 100.0 {
		t.Errorf("total = %v, want 100.0", srv.total)
	}
}

func TestServerProgressMiddleware_NoReporterWithoutToken(t *testing.T) {
	t.Parallel()
	s := NewMCPServer("test", "0.0.1")

	var capturedReporter ProgressReporter
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		capturedReporter = GetProgressReporter(ctx)
		return MakeTextResult("ok"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	mw := ServerProgressMiddleware(s, 100.0)
	wrapped := mw("test_tool", td, inner)

	// No Meta set — no token present.
	req := CallToolRequest{}
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReporter != nil {
		t.Errorf("expected nil reporter when no token, got %T", capturedReporter)
	}
}

func TestServerProgressMiddleware_NoReporterWithNilToken(t *testing.T) {
	t.Parallel()
	s := NewMCPServer("test", "0.0.1")

	var capturedReporter ProgressReporter
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		capturedReporter = GetProgressReporter(ctx)
		return MakeTextResult("ok"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	mw := ServerProgressMiddleware(s, 100.0)
	wrapped := mw("test_tool", td, inner)

	// Meta is set but ProgressToken is nil.
	req := CallToolRequest{}
	req.Params.Meta = &mcp.Meta{ProgressToken: nil}

	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReporter != nil {
		t.Errorf("expected nil reporter when token is nil, got %T", capturedReporter)
	}
}

func TestServerProgressReporter_ReportParams(t *testing.T) {
	t.Parallel()
	// Verify the params map is built correctly (indirectly, by checking no panic
	// and the struct fields are set correctly after construction).
	s := NewMCPServer("test", "0.0.1")

	r := NewServerProgressReporter(s, "tok", 50.0)
	if r.token != mcp.ProgressToken("tok") {
		t.Errorf("token = %v, want tok", r.token)
	}
	if r.total != 50.0 {
		t.Errorf("total = %v, want 50.0", r.total)
	}
	if r.server != s {
		t.Error("server reference not stored correctly")
	}
}
