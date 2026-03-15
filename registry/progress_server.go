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
