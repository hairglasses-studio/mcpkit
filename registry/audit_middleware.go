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

	data, err := json.Marshal(entry)
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

// AuditMiddleware returns a Middleware that logs each tool invocation as a
// JSON line to the specified log path. If logPath is empty, the default
// XDG_STATE_HOME/mcpkit/audit.jsonl is used.
//
// Each entry records the tool name, safety tier (from context, if available),
// call duration, error status, and the parameter keys (not values) for security.
func AuditMiddleware(logPath string) Middleware {
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

			entry := AuditEntry{
				Timestamp: start.UTC(),
				Tool:      name,
				Tier:      tier,
				Duration:  time.Since(start).Round(time.Millisecond).String(),
				Error:     isErr,
				ParamKeys: extractParamKeys(req),
			}

			w.write(entry)

			return result, err
		}
	}
}
