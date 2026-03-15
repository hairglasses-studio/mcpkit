//go:build official_sdk

package registry

import "context"

// ServerProgressReporter sends progress notifications through the MCP server.
// NOTE: The official go-sdk does not yet expose a per-session notification API
// comparable to mcp-go's SendNotificationToClient. This is a placeholder that
// silently succeeds until the official SDK adds progress support.
type ServerProgressReporter struct {
	total float64
}

// NewServerProgressReporter creates a ProgressReporter. In the official SDK build
// this is a no-op stub because per-session notifications are not yet supported.
func NewServerProgressReporter(server *MCPServer, token any, total float64) *ServerProgressReporter {
	return &ServerProgressReporter{total: total}
}

// Report is a no-op until the official SDK supports per-session notifications.
func (r *ServerProgressReporter) Report(ctx context.Context, progress float64, message string) error {
	return nil
}

// ServerProgressMiddleware returns middleware that injects a progress reporter.
// Currently a pass-through in the official SDK build.
func ServerProgressMiddleware(server *MCPServer, total float64) Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return next
	}
}
