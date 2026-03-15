// Package logging bridges the standard slog package with MCP clients.
//
// [Handler] implements [slog.Handler] by forwarding structured log records to
// connected MCP clients as LoggingMessageNotifications via the [LogSender]
// interface. It supports attribute groups, per-session minimum level
// filtering, and optional rate limiting to throttle noisy log streams.
// [InvocationMiddleware] wraps tool handlers to emit structured log entries
// for every tool call, its arguments, and its outcome without any boilerplate
// in individual handlers.
//
// Example:
//
//	logger := slog.New(logging.NewHandler(mcpServer))
//	reg := registry.New(registry.Config{
//	    Middleware: []registry.Middleware{logging.InvocationMiddleware(logger)},
//	})
package logging
