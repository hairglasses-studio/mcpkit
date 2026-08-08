//go:build official_sdk

package registry

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerProgressReporter sends progress notifications through the MCP
// server session. go-sdk v1.7.0 fully supports per-session progress
// notifications (ServerSession.NotifyProgress, mcp/server.go), demonstrated
// by the SDK's own Example_progress (mcp/mcp_example_test.go) — there are
// two ways to construct one:
//
//   - NewServerProgressReporterFromRequest (preferred): binds directly to
//     the request's own *mcp.ServerSession (CallToolRequest.Session, an
//     exported field on go-sdk's ServerRequest[P]), so it always notifies
//     the correct client even when the server is handling multiple
//     concurrent sessions (e.g. a stateless HTTP deployment).
//   - NewServerProgressReporter (server-only): kept for signature stability
//     with existing callers that only have a *MCPServer at construction
//     time, not a request (e.g. secretstudios-mcp's progressBoxMiddleware).
//     Since a *mcp.Server can have zero or many concurrent sessions and
//     there is no way to determine from the server alone which one issued a
//     given token, this variant sends only when EXACTLY ONE session is
//     currently active; an ambiguous session count (0 or 2+) logs a warning
//     and no-ops rather than guessing or misattributing the notification.
//     Callers that can reach the request directly should prefer
//     NewServerProgressReporterFromRequest instead — ServerProgressMiddleware
//     below always does.
type ServerProgressReporter struct {
	session *mcp.ServerSession // set when constructed from a request; always correct
	server  *MCPServer         // set when constructed from a server only; resolved at Report time
	token   any
	total   float64
}

// NewServerProgressReporter creates a ProgressReporter bound to a server
// (not a specific session) and a progress token. See the type doc comment
// for this constructor's single-active-session limitation; prefer
// NewServerProgressReporterFromRequest when a request is available.
func NewServerProgressReporter(server *MCPServer, token any, total float64) *ServerProgressReporter {
	return &ServerProgressReporter{server: server, token: token, total: total}
}

// NewServerProgressReporterFromRequest creates a ProgressReporter bound
// directly to the request's own session — always correct, no session
// ambiguity. The returned reporter no-ops on Report if req carries no
// progress token.
func NewServerProgressReporterFromRequest(req CallToolRequest, total float64) *ServerProgressReporter {
	return &ServerProgressReporter{
		session: req.Session,
		token:   ProgressTokenFromRequest(req),
		total:   total,
	}
}

// Report sends a progress notification to the client via
// ServerSession.NotifyProgress.
func (r *ServerProgressReporter) Report(ctx context.Context, progress float64, message string) error {
	if r.token == nil {
		return nil
	}
	session := r.session
	if session == nil {
		session = r.resolveSingleSession()
		if session == nil {
			return nil
		}
	}
	return session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: r.token,
		Progress:      progress,
		Total:         r.total,
		Message:       message,
	})
}

// resolveSingleSession returns r.server's one active session, or nil (with
// a logged warning) if there are zero or more than one — see
// ServerProgressReporter's doc comment for why this is the best available
// answer for a reporter constructed without a request.
func (r *ServerProgressReporter) resolveSingleSession() *mcp.ServerSession {
	if r.server == nil {
		return nil
	}
	var found *mcp.ServerSession
	count := 0
	for s := range r.server.Sessions() {
		found = s
		count++
		if count > 1 {
			break
		}
	}
	if count != 1 {
		slog.Warn("registry: cannot resolve a single session for progress notification (server-only reporter)",
			"active_sessions", count)
		return nil
	}
	return found
}

// ServerProgressMiddleware returns middleware that injects a
// request-bound ProgressReporter into the handler context when the request
// carries a progress token. The server parameter is accepted for signature
// parity with the mcp-go build (whose middleware needs a server reference
// to send notifications) but is unused here: the official-SDK reporter is
// built from the request itself via NewServerProgressReporterFromRequest,
// which is always session-correct and never hits the server-only
// constructor's single-session ambiguity.
func ServerProgressMiddleware(server *MCPServer, total float64) Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, request CallToolRequest) (*CallToolResult, error) {
			if token := ProgressTokenFromRequest(request); token != nil {
				reporter := NewServerProgressReporterFromRequest(request, total)
				ctx = WithProgressReporter(ctx, reporter)
			}
			return next(ctx, request)
		}
	}
}
