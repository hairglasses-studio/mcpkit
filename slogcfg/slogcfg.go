package slogcfg

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

// TracingHandler is a slog.Handler that adds trace_id and span_id to log records.
type TracingHandler struct {
	slog.Handler
}

// Handle adds trace and span IDs to the record if an active span is present in ctx.
func (h *TracingHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

// WithTracing returns a slog.Handler that adds trace and span IDs to log records
// when an active OpenTelemetry span is present in the context.
func WithTracing(h slog.Handler) slog.Handler {
	return &TracingHandler{Handler: h}
}

// WithOTel returns an ExtraHandler function that bridges slog to OpenTelemetry Logs.
// The name is used as the instrumentation scope name.
func WithOTel(name string) func(slog.Handler) slog.Handler {
	return func(h slog.Handler) slog.Handler {
		// Note: otelslog.NewHandler does not wrap h; it is a terminal handler.
		// To use both, use a fan-out handler or configure OTel to export to stdout.
		return otelslog.NewHandler(name)
	}
}

// Config controls the structured logging setup.
type Config struct {
	// ServiceName is added as a "service" attribute to all log entries.
	// If empty, no service attribute is added.
	ServiceName string

	// Level sets the minimum log level. Defaults to slog.LevelInfo.
	Level slog.Level

	// Output is the writer for log output. Defaults to os.Stderr.
	Output io.Writer

	// JSON selects JSON output (true, default) or text output (false).
	// JSON is recommended for production; text is useful for development.
	JSON bool

	// AddSource includes file:line in log output. Defaults to false.
	AddSource bool

	// ExtraHandler wraps the base handler, enabling integration with
	// OpenTelemetry (otelslog) or other log pipelines. If nil, the base
	// handler is used directly.
	ExtraHandler func(slog.Handler) slog.Handler
}

// Init configures the global slog default logger and returns it.
// Call this once at the start of main().
func Init(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var h slog.Handler
	if cfg.JSON {
		h = slog.NewJSONHandler(out, opts)
	} else {
		h = slog.NewTextHandler(out, opts)
	}

	if cfg.ExtraHandler != nil {
		h = cfg.ExtraHandler(h)
	}

	logger := slog.New(h)
	if cfg.ServiceName != "" {
		logger = logger.With("service", cfg.ServiceName)
	}

	slog.SetDefault(logger)
	return logger
}
