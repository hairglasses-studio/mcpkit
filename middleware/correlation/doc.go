// Package correlation provides MCP tool-handler middleware that injects a
// per-request correlation ID into the request context and (optionally)
// enriches a structured logger so every log line emitted during the
// handler call carries the same ID.
//
// Use this when you want correlation tracking WITHOUT the broader debug
// middleware's parameter-logging / redaction overhead. The two middlewares
// are mutually compatible — when both are wired, this one runs first so
// debug logging picks up the ID from the same context key.
//
// Typical usage:
//
//	reg := registry.NewToolRegistry()
//	reg.Use(correlation.Middleware(correlation.Config{
//		Logger:      slog.Default(),
//		HeaderField: "request_id",
//	}))
//
//	// Inside a handler:
//	func myHandler(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
//		id := correlation.FromContext(ctx) // non-empty when middleware is wired
//		// ...
//	}
//
// The package is designed to be cheap (no allocations when disabled) and
// safe to enable in production hot paths.
package correlation
