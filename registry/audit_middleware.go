//go:build !official_sdk

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// AuditEntry represents a single audit log record written as a JSON line.
type AuditEntry struct {
	Timestamp time.Time  `json:"timestamp"`
	Tool      string     `json:"tool"`
	Tier      SafetyTier `json:"tier"`
	Duration  string     `json:"duration"`
	Error     bool       `json:"error"`
	ParamKeys []string   `json:"param_keys"`
}

// maxAuditFileSize is the threshold at which the audit log is rotated (10 MB).
const maxAuditFileSize = 10 * 1024 * 1024

// defaultAuditLogPath returns the default audit log path using XDG_STATE_HOME.
func defaultAuditLogPath() string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, _ := os.UserHomeDir()
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "mcpkit", "audit.jsonl")
}

// auditWriter manages thread-safe, rotation-aware JSONL writing.
type auditWriter struct {
	mu      sync.Mutex
	logPath string
}

func (w *auditWriter) write(entry AuditEntry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeLockedAny(entry)
}

// writeRaw serializes any value and appends it as a JSON line, using the same
// rotation logic as write. Intended for enhanced audit entries with extra fields.
func (w *auditWriter) writeRaw(v any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeLockedAny(v)
}

// writeLockedAny writes v as a JSON line; caller must hold w.mu.
func (w *auditWriter) writeLockedAny(v any) {
	// Ensure parent directory exists.
	dir := filepath.Dir(w.logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("audit: mkdir failed", "path", dir, "error", err)
		return
	}

	// Check for rotation before writing.
	if info, err := os.Stat(w.logPath); err == nil && info.Size() >= maxAuditFileSize {
		rotated := w.logPath + ".1"
		if err := os.Rename(w.logPath, rotated); err != nil {
			slog.Warn("audit: rotate failed", "error", err)
		}
	}

	f, err := os.OpenFile(w.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("audit: open failed", "path", w.logPath, "error", err)
		return
	}
	defer f.Close()

	data, err := json.Marshal(v)
	if err != nil {
		slog.Warn("audit: marshal failed", "error", err)
		return
	}
	fmt.Fprintf(f, "%s\n", data)
}

// extractParamKeys returns sorted top-level keys from the request arguments.
func extractParamKeys(req CallToolRequest) []string {
	args := ExtractArguments(req)
	if len(args) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ParamSanitizer is a function that sanitizes a map of tool parameters before
// they are included in audit log entries. It must return a new map without
// modifying the original.
type ParamSanitizer func(params map[string]any) map[string]any

// AuditEntryWithValues extends AuditEntry with sanitized parameter values.
type AuditEntryWithValues struct {
	AuditEntry
	// ParamValues contains sanitized parameter values. Only present when
	// AuditMiddlewareWithValues is used and a sanitizer is provided.
	ParamValues map[string]any `json:"param_values,omitempty"`
}

// AuditMiddleware returns a Middleware that logs each tool invocation as a
// JSON line to the specified log path. If logPath is empty, the default
// XDG_STATE_HOME/mcpkit/audit.jsonl is used.
//
// Each entry records the tool name, safety tier (from context, if available),
// call duration, error status, and the parameter keys (not values) for security.
func AuditMiddleware(logPath string) Middleware {
	return AuditMiddlewareWithValues(logPath, nil)
}

// WithSanitizedValues is a constructor option type for AuditMiddlewareWithValues.
// Pass a secrets.Sanitize-compatible function to enable param-value logging
// with sensitive values masked.
type WithSanitizedValues = ParamSanitizer

// AuditMiddlewareWithValues returns an enhanced AuditMiddleware that can
// optionally log sanitized parameter values alongside the usual keys.
// If sanitizer is nil, behavior is identical to AuditMiddleware (keys only).
// If sanitizer is non-nil, it is applied to the arguments map before logging;
// the sanitized values are written to the audit log as param_values.
func AuditMiddlewareWithValues(logPath string, sanitizer ParamSanitizer) Middleware {
	if logPath == "" {
		logPath = defaultAuditLogPath()
	}
	w := &auditWriter{logPath: logPath}

	return func(name string, td ToolDefinition, next ToolHandlerFunc) ToolHandlerFunc {
		// Pre-classify the tier from the tool definition so the audit
		// entry is correct regardless of middleware ordering.
		tier := ClassifySafetyTier(td)

		return func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
			start := time.Now()

			result, err := next(ctx, req)

			isErr := err != nil || IsResultError(result)

			base := AuditEntry{
				Timestamp: start.UTC(),
				Tool:      name,
				Tier:      tier,
				Duration:  time.Since(start).Round(time.Millisecond).String(),
				Error:     isErr,
				ParamKeys: extractParamKeys(req),
			}

			if sanitizer != nil {
				args := ExtractArguments(req)
				sanitized := sanitizer(args)
				entry := AuditEntryWithValues{AuditEntry: base, ParamValues: sanitized}
				w.writeRaw(entry)
			} else {
				w.write(base)
			}

			return result, err
		}
	}
}
