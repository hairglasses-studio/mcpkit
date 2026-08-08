//go:build !official_sdk

package registry

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// ServerProgressReporter sends progress notifications through the MCP server.
// It implements ProgressReporter by calling SendNotificationToClient.
type ServerProgressReporter struct {
	server *MCPServer
	token  mcp.ProgressToken
	total  float64
}

// NewServerProgressReporter creates a ProgressReporter bound to a specific progress token.
// total is the total progress value (e.g., 100.0 for percentage-based progress, or
// the number of items to process). Use 0 if total is unknown.
func NewServerProgressReporter(server *MCPServer, token mcp.ProgressToken, total float64) *ServerProgressReporter {
	return &ServerProgressReporter{server: server, token: token, total: total}
}

// NewServerProgressReporterFromRequest creates a ProgressReporter bound to
// server, extracting the progress token from req via ProgressTokenFromRequest
// instead of requiring the caller to pull it off req.Params.Meta directly —
// the mcp-go counterpart to progress_server_official.go's constructor of the
// same name, added for name/call-site discoverability parity.
//
// Unlike the official build, this still takes server explicitly: the
// official SDK's CallToolRequest carries its own *mcp.ServerSession
// (req.Session), so its FromRequest constructor needs nothing else, but
// mcp-go's CallToolRequest carries no session/server reference at all —
// Report() resolves the target session by calling
// server.SendNotificationToClient(ctx, ...), which needs a concrete
// *MCPServer to call the method on, resolving the actual session
// internally via ctx at call time (see Report's doc comment: this is why
// mcp-go's reporter was never subject to official's session-count
// ambiguity in the first place — ctx-based resolution is always exact).
// This is the "server-parameter variant" this pair's official-side sibling
// doc comment calls out as the least-breaking shape for this asymmetry.
func NewServerProgressReporterFromRequest(server *MCPServer, req CallToolRequest, total float64) *ServerProgressReporter {
	return NewServerProgressReporter(server, mcp.ProgressToken(ProgressTokenFromRequest(req)), total)
}

// Report sends a progress notification to the client.
func (r *ServerProgressReporter) Report(ctx context.Context, progress float64, message string) error {
	params := map[string]any{
		"progressToken": r.token,
		"progress":      progress,
	}
	if r.total > 0 {
		params["total"] = r.total
	}
	if message != "" {
		params["message"] = message
	}
	return r.server.SendNotificationToClient(ctx, "notifications/progress", params)
}

// ServerProgressMiddleware returns middleware that injects a server-backed
// ProgressReporter into the handler context when the request includes a progress token.
// If the request has no progress token, no reporter is injected and existing context
// state is preserved.
func ServerProgressMiddleware(server *MCPServer, total float64) Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, request CallToolRequest) (*CallToolResult, error) {
			if request.Params.Meta != nil && request.Params.Meta.ProgressToken != nil {
				reporter := NewServerProgressReporter(server, request.Params.Meta.ProgressToken, total)
				ctx = WithProgressReporter(ctx, reporter)
			}
			return next(ctx, request)
		}
	}
}
