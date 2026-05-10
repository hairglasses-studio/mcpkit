// Package feedback provides an MCP feedback submission tool and pluggable sinks.
//
// The package is intentionally small: servers register [Module] to expose the
// feedback_submit tool, then choose where records are stored by providing a
// [Sink]. [MemorySink] is useful for tests and embedded demos. [JSONLSink]
// writes append-only local JSON Lines files for lightweight deployments.
//
// Example:
//
//	sink := feedback.NewMemorySink()
//	reg := registry.NewToolRegistry()
//	reg.RegisterModule(feedback.NewModule(sink, feedback.WithSource("my-server")))
package feedback
