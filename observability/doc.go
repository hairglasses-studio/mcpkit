// Package observability provides OpenTelemetry tracing and metrics for MCP
// servers via a drop-in [registry.Middleware].
//
// Call [Init] with a [Config] to initialize a [Provider] that wires up an
// OTLP trace exporter, an OTLP metrics exporter, and an optional Prometheus
// HTTP server. The [Middleware] function returns a [registry.Middleware] that
// creates a span per tool invocation, records tool name, error status, and
// genai token usage attributes, and increments the invocation/error counters
// and latency histogram exposed through [Metrics].
//
// Example:
//
//	provider, _ := observability.Init(observability.Config{
//	    ServiceName:    "my-mcp-server",
//	    OTLPEndpoint:   "localhost:4317",
//	    PrometheusPort: "9091",
//	    EnableTracing:  true,
//	    EnableMetrics:  true,
//	})
//	defer provider.Shutdown(ctx)
//	reg := registry.New(registry.Config{
//	    Middleware: []registry.Middleware{observability.Middleware(provider)},
//	})
package observability
