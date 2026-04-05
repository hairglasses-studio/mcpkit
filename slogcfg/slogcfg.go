package slogcfg

import (
	"io"
	"log/slog"
	"os"
)

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
