// Package logging provides an slog.Handler that forwards structured log
// messages to MCP clients as LoggingMessageNotifications, plus middleware
// for automatic tool invocation logging.
package logging

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// LogSender abstracts the MCP server's ability to send log notifications
// to connected clients.
type LogSender interface {
	SendLog(ctx context.Context, level, logger string, data any) error
}

// DefaultMinLevel is the minimum log level when none is configured.
// Set to LevelDebug so all messages are forwarded by default.
const DefaultMinLevel = slog.LevelDebug

// Config configures the logging handler.
type Config struct {
	// LoggerName is the "logger" field in MCP log notifications.
	// Defaults to empty string (omitted).
	LoggerName string

	// MinLevel is the server-side minimum log level.
	// Messages below this level are silently dropped.
	// Use NewConfig() or set explicitly; the zero value (LevelInfo)
	// may not be what you want — use DefaultMinLevel for all levels.
	MinLevel slog.Level

	// RateLimit is the minimum interval between log sends.
	// Messages arriving faster than this are dropped.
	// Zero means no rate limiting.
	RateLimit time.Duration
}

// Handler implements slog.Handler by forwarding log records to MCP clients
// via the LogSender interface. It supports attribute groups, per-session
// level filtering, and optional rate limiting.
type Handler struct {
	sender     LogSender
	config     Config
	attrs      []slog.Attr
	groups     []string
	mu         sync.Mutex
	lastSendAt time.Time
}

// NewHandler creates an slog.Handler that forwards log messages to MCP clients.
// When no Config is provided, MinLevel defaults to LevelDebug (all messages forwarded).
func NewHandler(sender LogSender, config ...Config) *Handler {
	cfg := Config{MinLevel: DefaultMinLevel}
	if len(config) > 0 {
		cfg = config[0]
	}
	return &Handler{
		sender: sender,
		config: cfg,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.config.MinLevel
}

// Handle sends the log record to MCP clients as a notification.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}

	if h.config.RateLimit > 0 {
		h.mu.Lock()
		now := time.Now()
		if now.Sub(h.lastSendAt) < h.config.RateLimit {
			h.mu.Unlock()
			return nil
		}
		h.lastSendAt = now
		h.mu.Unlock()
	}

	data := h.buildData(record)
	mcpLevel := slogToMCPLevel(record.Level)

	return h.sender.SendLog(ctx, mcpLevel, h.config.LoggerName, data)
}

// WithAttrs returns a new Handler with the given attributes added.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.clone()
	h2.attrs = append(h2.attrs, attrs...)
	return h2
}

// WithGroup returns a new Handler with the given group name.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *Handler) clone() *Handler {
	return &Handler{
		sender: h.sender,
		config: h.config,
		attrs:  append([]slog.Attr(nil), h.attrs...),
		groups: append([]string(nil), h.groups...),
	}
}

func (h *Handler) buildData(record slog.Record) map[string]any {
	data := make(map[string]any)

	// Add message
	data["message"] = record.Message

	// Add pre-configured attributes
	target := h.targetMap(data)
	for _, attr := range h.attrs {
		addAttr(target, attr)
	}

	// Add record attributes
	record.Attrs(func(attr slog.Attr) bool {
		addAttr(target, attr)
		return true
	})

	return data
}

// targetMap resolves the nested map for group-prefixed attributes.
func (h *Handler) targetMap(root map[string]any) map[string]any {
	m := root
	for _, g := range h.groups {
		sub, ok := m[g].(map[string]any)
		if !ok {
			sub = make(map[string]any)
			m[g] = sub
		}
		m = sub
	}
	return m
}

func addAttr(m map[string]any, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		sub := make(map[string]any)
		for _, a := range attr.Value.Group() {
			addAttr(sub, a)
		}
		if len(sub) > 0 {
			if attr.Key != "" {
				m[attr.Key] = sub
			} else {
				// Inline group (empty key): merge into parent.
				for k, v := range sub {
					m[k] = v
				}
			}
		}
		return
	}
	m[attr.Key] = attr.Value.Any()
}

// slogToMCPLevel converts an slog.Level to an MCP log level string.
func slogToMCPLevel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "error"
	case level >= slog.LevelWarn:
		return "warning"
	case level >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}
