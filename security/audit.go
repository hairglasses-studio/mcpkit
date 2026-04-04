package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// AuditEventType represents the type of audit event.
type AuditEventType string

const (
	AuditToolCall    AuditEventType = "tool_call"
	AuditToolSuccess AuditEventType = "tool_success"
	AuditToolError   AuditEventType = "tool_error"
	AuditAccessDeny  AuditEventType = "access_denied"
	AuditLogin       AuditEventType = "login"
	AuditSecretRead  AuditEventType = "secret_read"
)

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Type      AuditEventType `json:"type"`
	User      string         `json:"user"`
	Tool      string         `json:"tool,omitempty"`
	Action    string         `json:"action,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	Result    string         `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
	Duration  time.Duration  `json:"duration_ms,omitempty"`
	IP        string         `json:"ip,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
}

// AuditLoggerConfig configures the audit logger.
type AuditLoggerConfig struct {
	// LogFile is the path to write audit events. Empty disables file logging.
	LogFile string

	// MaxEvents is the maximum in-memory events to retain. Default: 1000.
	MaxEvents int

	// QueueSize is the channel buffer for async writes. Default: 100.
	QueueSize int

	// Exporters are additional sinks that receive every audit event synchronously.
	Exporters []AuditExporter
}

// AuditLogger handles audit event logging.
type AuditLogger struct {
	mu           sync.Mutex
	events       []AuditEvent
	maxEvents    int
	logFile      string
	writeQueue   chan AuditEvent
	stopCh       chan struct{}
	exporters    []AuditExporter
	dropped      atomic.Int64
	exportErrors atomic.Int64
	seq          atomic.Uint64
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(config AuditLoggerConfig) *AuditLogger {
	if config.MaxEvents <= 0 {
		config.MaxEvents = 1000
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}

	exporters := make([]AuditExporter, len(config.Exporters))
	copy(exporters, config.Exporters)

	logger := &AuditLogger{
		events:     make([]AuditEvent, 0, config.MaxEvents),
		maxEvents:  config.MaxEvents,
		logFile:    config.LogFile,
		writeQueue: make(chan AuditEvent, config.QueueSize),
		stopCh:     make(chan struct{}),
		exporters:  exporters,
	}

	if config.LogFile != "" {
		go logger.backgroundWriter()
	}

	return logger
}

// Log records an audit event.
func (l *AuditLogger) Log(event AuditEvent) {
	event.Timestamp = time.Now()
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d-%d", event.Timestamp.UnixNano(), l.seq.Add(1))
	}

	l.mu.Lock()
	if len(l.events) >= l.maxEvents {
		l.events = l.events[1:]
	}
	l.events = append(l.events, event)
	l.mu.Unlock()

	if l.logFile != "" {
		select {
		case l.writeQueue <- event:
		default:
			l.dropped.Add(1)
		}
	}

	for _, exp := range l.exporters {
		if err := exp.Export(event); err != nil {
			l.exportErrors.Add(1)
		}
	}
}

// LogToolCall logs a tool invocation.
func (l *AuditLogger) LogToolCall(user, tool string, params map[string]any) {
	l.Log(AuditEvent{
		Type:   AuditToolCall,
		User:   user,
		Tool:   tool,
		Params: SanitizeAuditParams(params),
	})
}

// LogToolResult logs a tool result.
func (l *AuditLogger) LogToolResult(user, tool string, duration time.Duration, err error) {
	event := AuditEvent{
		User:     user,
		Tool:     tool,
		Duration: duration,
	}
	if err != nil {
		event.Type = AuditToolError
		event.Error = err.Error()
	} else {
		event.Type = AuditToolSuccess
		event.Result = "success"
	}
	l.Log(event)
}

// LogAccessDenied logs an access denied event.
func (l *AuditLogger) LogAccessDenied(user, tool, reason string) {
	l.Log(AuditEvent{
		Type:  AuditAccessDeny,
		User:  user,
		Tool:  tool,
		Error: reason,
	})
}

// GetRecentEvents returns recent audit events.
func (l *AuditLogger) GetRecentEvents(limit int) []AuditEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > len(l.events) {
		limit = len(l.events)
	}
	start := len(l.events) - limit
	result := make([]AuditEvent, limit)
	copy(result, l.events[start:])
	return result
}

// GetEventsByUser returns events for a specific user.
func (l *AuditLogger) GetEventsByUser(user string, limit int) []AuditEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	var result []AuditEvent
	for i := len(l.events) - 1; i >= 0 && len(result) < limit; i-- {
		if l.events[i].User == user {
			result = append(result, l.events[i])
		}
	}
	return result
}

// GetEventsByTool returns events for a specific tool.
func (l *AuditLogger) GetEventsByTool(tool string, limit int) []AuditEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	var result []AuditEvent
	for i := len(l.events) - 1; i >= 0 && len(result) < limit; i-- {
		if l.events[i].Tool == tool {
			result = append(result, l.events[i])
		}
	}
	return result
}

// GetErrorEvents returns recent error events.
func (l *AuditLogger) GetErrorEvents(limit int) []AuditEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	var result []AuditEvent
	for i := len(l.events) - 1; i >= 0 && len(result) < limit; i-- {
		if l.events[i].Type == AuditToolError || l.events[i].Type == AuditAccessDeny {
			result = append(result, l.events[i])
		}
	}
	return result
}

// AuditStats contains summary statistics.
type AuditStats struct {
	TotalEvents    int            `json:"total_events"`
	EventsByType   map[string]int `json:"events_by_type"`
	EventsByUser   map[string]int `json:"events_by_user"`
	ErrorCount     int            `json:"error_count"`
	AccessDenied   int            `json:"access_denied"`
	TopTools       map[string]int `json:"top_tools"`
	AverageLatency time.Duration  `json:"average_latency_ms"`
	DroppedEvents  int64          `json:"dropped_events"`
	ExportErrors   int64          `json:"export_errors"`
}

// GetStats returns summary statistics.
func (l *AuditLogger) GetStats() AuditStats {
	l.mu.Lock()
	defer l.mu.Unlock()

	stats := AuditStats{
		TotalEvents:  len(l.events),
		EventsByType: make(map[string]int),
		EventsByUser: make(map[string]int),
		TopTools:     make(map[string]int),
	}
	var totalLatency time.Duration
	var latencyCount int
	for _, e := range l.events {
		stats.EventsByType[string(e.Type)]++
		stats.EventsByUser[e.User]++
		if e.Tool != "" {
			stats.TopTools[e.Tool]++
		}
		if e.Type == AuditToolError {
			stats.ErrorCount++
		}
		if e.Type == AuditAccessDeny {
			stats.AccessDenied++
		}
		if e.Duration > 0 {
			totalLatency += e.Duration
			latencyCount++
		}
	}
	if latencyCount > 0 {
		stats.AverageLatency = totalLatency / time.Duration(latencyCount)
	}
	stats.DroppedEvents = l.dropped.Load()
	stats.ExportErrors = l.exportErrors.Load()
	return stats
}

// Close stops the background writer and closes all exporters.
func (l *AuditLogger) Close() {
	close(l.stopCh)
	for _, exp := range l.exporters {
		_ = exp.Close()
	}
}

func (l *AuditLogger) backgroundWriter() {
	dir := filepath.Dir(l.logFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "audit: backgroundWriter: failed to create directory %q: %v\n", dir, err)
		return
	}
	for {
		select {
		case event := <-l.writeQueue:
			l.writeEvent(event)
		case <-l.stopCh:
			return
		}
	}
}

func (l *AuditLogger) writeEvent(event AuditEvent) {
	f, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	f.Write(data)
	f.WriteString("\n")
}

// SanitizeAuditParams removes sensitive values from parameters for audit logging.
func SanitizeAuditParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}

	sensitiveKeys := []string{
		"password", "secret", "token", "key", "credential",
		"auth", "bearer", "api_key", "private", "oauth",
	}

	result := make(map[string]any)
	for k, v := range params {
		isSensitive := false
		kLower := strings.ToLower(k)
		for _, sensitive := range sensitiveKeys {
			if kLower == sensitive || strings.Contains(kLower, sensitive) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			result[k] = "[REDACTED]"
		} else {
			result[k] = v
		}
	}
	return result
}
