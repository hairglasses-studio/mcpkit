// Package feedback provides an MCP feedback submission tool, pluggable sinks,
// and opt-in anonymous usage telemetry.
//
// The package is intentionally small: servers register [Module] to expose the
// feedback_submit tool, then choose where records are stored by providing a
// [Sink]. [MemorySink] is useful for tests and embedded demos. [JSONLSink]
// writes append-only local JSON Lines files for lightweight deployments.
// [TelemetryCollector] adds package/tool usage counts and error rates only
// when [TelemetryConfig.OptIn] is explicitly enabled. [DashboardHandler] serves
// the aggregate telemetry as dashboard-friendly JSON.
//
// Example:
//
//	sink := feedback.NewMemorySink()
//	telemetry, _ := feedback.NewTelemetryCollector(feedback.TelemetryConfig{Enabled: true, OptIn: true})
//	reg := registry.NewToolRegistry()
//	reg.RegisterModule(feedback.NewModule(sink, feedback.WithSource("my-server")))
//	reg.SetMiddleware([]registry.Middleware{telemetry.Middleware()})
package feedback
