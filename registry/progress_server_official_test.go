//go:build official_sdk

package registry

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewServerProgressReporter_NotNil(t *testing.T) {
	t.Parallel()
	s := NewMCPServer("test", "0.0.1")
	reporter := NewServerProgressReporter(s, "token-1", 100.0)
	if reporter == nil {
		t.Fatal("NewServerProgressReporter returned nil")
	}
}

func TestServerProgressReporter_ReportNoopsWithoutSession(t *testing.T) {
	t.Parallel()
	// Unlike the mcp-go build (where SendNotificationToClient errors without
	// an active session), the server-only constructor's resolveSingleSession
	// finds zero active sessions here and Report honestly no-ops (logging a
	// warning) rather than erroring — see ServerProgressReporter's doc
	// comment for why zero/multiple sessions can't be resolved from a
	// server alone.
	s := NewMCPServer("test", "0.0.1")
	reporter := NewServerProgressReporter(s, "token-1", 100.0)
	if err := reporter.Report(context.Background(), 0.5, "halfway"); err != nil {
		t.Fatalf("Report should no-op (nil error) with zero active sessions, got: %v", err)
	}
}

func TestServerProgressReporterFromRequest_NoopWithoutToken(t *testing.T) {
	t.Parallel()
	req := CallToolRequest{Params: &mcp.CallToolParamsRaw{}}
	reporter := NewServerProgressReporterFromRequest(req, 100.0)
	if reporter.token != nil {
		t.Errorf("token = %v, want nil", reporter.token)
	}
	if err := reporter.Report(context.Background(), 0.5, "halfway"); err != nil {
		t.Fatalf("Report should no-op with no token, got: %v", err)
	}
}

func TestServerProgressMiddleware_InjectsRequestBoundReporterWithToken(t *testing.T) {
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

	req := CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Meta: mcp.Meta{"progressToken": "my-token"},
	}}

	if _, err := wrapped(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReporter == nil {
		t.Fatal("expected reporter to be injected when token is present, got nil")
	}

	srv, ok := capturedReporter.(*ServerProgressReporter)
	if !ok {
		t.Fatalf("expected *ServerProgressReporter, got %T", capturedReporter)
	}
	if srv.token != "my-token" {
		t.Errorf("token = %v, want my-token", srv.token)
	}
	if srv.total != 100.0 {
		t.Errorf("total = %v, want 100.0", srv.total)
	}
	// The middleware path is request-bound, never the server-only
	// (ambiguous-session) constructor.
	if srv.session != nil {
		t.Error("expected session to be nil here (no real transport connected in this unit test), but the reporter must be the request-bound kind (session field present, not server-only)")
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

	req := CallToolRequest{Params: &mcp.CallToolParamsRaw{}}
	if _, err := wrapped(context.Background(), req); err != nil {
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

	req := CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Meta: mcp.Meta{"progressToken": nil},
	}}
	if _, err := wrapped(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReporter != nil {
		t.Errorf("expected nil reporter when token is nil, got %T", capturedReporter)
	}
}

// TestServerProgressMiddleware_NotifiesRealSession is the end-to-end
// assertion team-lead's round 9 scope required: a middleware-driven handler
// that calls Report() through the neutral registry.ProgressReporter
// interface must produce a real notifications/progress message the client
// actually receives — not just an injected-but-inert reporter. This follows
// the SDK's own Example_progress pattern (mcp/mcp_example_test.go):
// mcp.NewInMemoryTransports, a real client with ProgressNotificationHandler,
// server.Connect/client.Connect, then a real CallTool round-trip.
func TestServerProgressMiddleware_NotifiesRealSession(t *testing.T) {
	t.Parallel()

	s := NewMCPServer("test", "0.0.1")

	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		reporter := GetProgressReporter(ctx)
		if reporter == nil {
			t.Error("expected a progress reporter in context")
			return MakeTextResult("no reporter"), nil
		}
		for i := 0; i < 3; i++ {
			if err := reporter.Report(ctx, float64(i), "frobbing widgets"); err != nil {
				t.Errorf("Report(%d) returned error: %v", i, err)
			}
		}
		return MakeTextResult("done"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "make_progress"}}
	mw := ServerProgressMiddleware(s, 2.0)
	wrapped := mw("make_progress", td, inner)

	AddToolToServer(s, mcp.Tool{
		Name:        "make_progress",
		InputSchema: map[string]any{"type": "object"},
	}, wrapped)

	type notification struct {
		message  string
		progress float64
		total    float64
	}
	var received []notification

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			received = append(received, notification{
				message:  req.Params.Message,
				progress: req.Params.Progress,
				total:    req.Params.Total,
			})
		},
	})

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}

	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer session.Close()

	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "make_progress",
		Meta: mcp.Meta{"progressToken": "abc123"},
	}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if len(received) != 3 {
		t.Fatalf("got %d progress notifications, want 3: %+v", len(received), received)
	}
	for i, n := range received {
		if n.message != "frobbing widgets" {
			t.Errorf("notification %d: message = %q, want %q", i, n.message, "frobbing widgets")
		}
		if n.progress != float64(i) {
			t.Errorf("notification %d: progress = %v, want %v", i, n.progress, float64(i))
		}
		if n.total != 2.0 {
			t.Errorf("notification %d: total = %v, want 2.0", i, n.total)
		}
	}
}
