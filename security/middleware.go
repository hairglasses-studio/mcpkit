package security

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// AuditMiddleware returns a registry.Middleware that logs tool invocations
// and completions to the provided AuditLogger.
//
// The userFunc extracts the username from the context. If nil, the user
// field is left empty in audit events.
func AuditMiddleware(logger *AuditLogger, userFunc func(context.Context) string) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			user := ""
			if userFunc != nil {
				user = userFunc(ctx)
			}

			var auditParams map[string]any
			if request.Params.Arguments != nil {
				if args, ok := request.Params.Arguments.(map[string]interface{}); ok {
					auditParams = args
				}
			}

			logger.LogToolCall(user, name, auditParams)
			start := time.Now()

			result, err := next(ctx, request)

			logger.LogToolResult(user, name, time.Since(start), err)
			return result, err
		}
	}
}

// RBACMiddleware returns a registry.Middleware that checks access control
// before executing the tool handler.
//
// The userFunc extracts the username from the context. If the user lacks
// permission, the middleware returns an error result without calling the handler.
func RBACMiddleware(rbac *RBAC, logger *AuditLogger, userFunc func(context.Context) string) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			user := ""
			if userFunc != nil {
				user = userFunc(ctx)
			}

			if !rbac.CanAccessTool(user, name) {
				if logger != nil {
					logger.LogAccessDenied(user, name, "insufficient permissions")
				}
				return mcp.NewToolResultError("[PERMISSION_DENIED] access denied for tool " + name), nil
			}

			return next(ctx, request)
		}
	}
}
