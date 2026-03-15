// Package health provides an HTTP health check endpoint and checker registry
// for MCP servers.
//
// [Checker] tracks server lifecycle status, tool count, active task count, and
// circuit breaker states. [Handler] serves a JSON [Status] payload at the
// configured path; it returns HTTP 503 when the server is draining or stopped,
// enabling Kubernetes readiness probes to gate traffic during graceful
// shutdown. [SetStatus] and [IsReady] are the primary integration points for
// the lifecycle package.
//
// Example:
//
//	checker := health.NewChecker(
//	    health.WithToolCount(reg.ToolCount),
//	)
//	mux.Handle("/health", health.Handler(checker))
package health
