package tasks

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

type taskContextKey struct{}

// GetTaskEntry returns the TaskEntry from the context, if the tool is running as a task.
func GetTaskEntry(ctx context.Context) *TaskEntry {
	e, _ := ctx.Value(taskContextKey{}).(*TaskEntry)
	return e
}

// TaskMiddleware returns a registry.Middleware that enables async task execution
// for tools with TaskSupport set to optional or required.
//
// When a tool call includes task params (via the "task" field in CallToolParams),
// the middleware:
// 1. Creates a task via the Manager
// 2. Runs the handler asynchronously
// 3. Returns the task info immediately
//
// The ToolDefinition's Tool.Execution.TaskSupport must be "optional" or "required".
func TaskMiddleware(mgr Manager) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		// Skip tools that don't support tasks
		support := taskSupportFor(td)
		if support == mcp.TaskSupportForbidden || support == "" {
			return next
		}

		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Check if the request includes task params
			if !hasTaskParams(request) {
				if support == mcp.TaskSupportRequired {
					return mcp.NewToolResultError("[INVALID_PARAM] tool " + name + " requires task augmentation"), nil
				}
				return next(ctx, request)
			}

			// Extract TTL from request
			ttl := extractTTL(request)

			// Create the task
			entry := mgr.Create(ttl)

			// Set up cancellation
			taskCtx, cancel := context.WithCancel(context.Background())
			entry.CancelFn = cancel

			// Inject task entry into context
			taskCtx = context.WithValue(taskCtx, taskContextKey{}, entry)

			// Run handler asynchronously
			go func() {
				defer cancel()
				result, err := next(taskCtx, request)
				if err != nil {
					entry.Update(mcp.TaskStatusFailed, err.Error())
					return
				}
				entry.SetResult(result)
				if result != nil && result.IsError {
					entry.Update(mcp.TaskStatusFailed, "tool returned error")
				} else {
					entry.Update(mcp.TaskStatusCompleted, "completed")
				}
			}()

			// Return task info immediately as a tool result
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: "Task created: " + entry.Task.TaskId,
					},
				},
			}, nil
		}
	}
}

func taskSupportFor(td registry.ToolDefinition) mcp.TaskSupport {
	if td.Tool.Execution == nil {
		return mcp.TaskSupportForbidden
	}
	return td.Tool.Execution.TaskSupport
}

func hasTaskParams(req mcp.CallToolRequest) bool {
	return req.Params.Task != nil
}

func extractTTL(req mcp.CallToolRequest) time.Duration {
	if req.Params.Task == nil || req.Params.Task.TTL == nil {
		return 0
	}
	return time.Duration(*req.Params.Task.TTL) * time.Millisecond
}
