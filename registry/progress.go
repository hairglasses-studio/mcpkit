package registry

import (
	"context"
)

// ProgressReporter allows tool handlers to report progress for long-running
// operations. This integrates with the MCP progress notification system.
//
// Usage in a tool handler:
//
//	reporter := registry.GetProgressReporter(ctx)
//	if reporter != nil {
//	    reporter.Report(ctx, 0.25, "Loading data...")
//	    reporter.Report(ctx, 0.75, "Processing...")
//	    reporter.Report(ctx, 1.0, "Complete")
//	}
type ProgressReporter interface {
	// Report sends a progress update. Progress is a value between 0.0 and 1.0.
	// Message is an optional human-readable status message.
	Report(ctx context.Context, progress float64, message string) error
}

type progressKey struct{}

// WithProgressReporter returns a context with the given ProgressReporter attached.
func WithProgressReporter(ctx context.Context, reporter ProgressReporter) context.Context {
	return context.WithValue(ctx, progressKey{}, reporter)
}

// GetProgressReporter extracts the ProgressReporter from the context, or nil
// if none is set.
func GetProgressReporter(ctx context.Context) ProgressReporter {
	r, _ := ctx.Value(progressKey{}).(ProgressReporter)
	return r
}

// ProgressMiddleware returns a Middleware that injects a ProgressReporter into
// the handler context. The reporterFactory creates a reporter for each tool
// invocation, receiving the tool name and definition.
func ProgressMiddleware(reporterFactory func(name string, td ToolDefinition) ProgressReporter) Middleware {
	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, request CallToolRequest) (*CallToolResult, error) {
			reporter := reporterFactory(name, td)
			if reporter != nil {
				ctx = WithProgressReporter(ctx, reporter)
			}
			return next(ctx, request)
		}
	}
}
