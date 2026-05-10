package feedback

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ErrTelemetryConsentRequired is returned when telemetry is enabled without
// explicit opt-in consent.
var ErrTelemetryConsentRequired = errors.New("telemetry requires explicit opt-in consent")

// TelemetryConfig configures anonymous usage telemetry.
type TelemetryConfig struct {
	// Enabled controls whether telemetry records anything.
	Enabled bool

	// OptIn must be true when Enabled is true. This makes consent explicit at
	// construction time instead of hidden behind defaults.
	OptIn bool

	// Source identifies the server or product emitting telemetry.
	Source string

	// Now overrides the timestamp source. It is primarily useful in tests.
	Now func() time.Time
}

// TelemetryEvent is one anonymous usage event.
type TelemetryEvent struct {
	Package   string
	Tool      string
	ErrorCode string
}

// TelemetryCounter is an aggregate for one package/tool pair.
type TelemetryCounter struct {
	Package       string    `json:"package"`
	Tool          string    `json:"tool"`
	Calls         int64     `json:"calls"`
	Errors        int64     `json:"errors"`
	ErrorRate     float64   `json:"error_rate"`
	LastErrorCode string    `json:"last_error_code,omitempty"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

// TelemetrySnapshot is an exportable anonymous telemetry summary.
type TelemetrySnapshot struct {
	Enabled     bool               `json:"enabled"`
	Source      string             `json:"source,omitempty"`
	StartedAt   time.Time          `json:"started_at"`
	GeneratedAt time.Time          `json:"generated_at"`
	Counters    []TelemetryCounter `json:"counters"`
}

// TelemetryCollector aggregates anonymous package/tool usage and error rates.
type TelemetryCollector struct {
	mu        sync.Mutex
	enabled   bool
	source    string
	startedAt time.Time
	now       func() time.Time
	counters  map[telemetryKey]*TelemetryCounter
}

type telemetryKey struct {
	pkg  string
	tool string
}

// NewTelemetryCollector creates a telemetry collector. Enabled telemetry
// requires explicit opt-in consent.
func NewTelemetryCollector(config TelemetryConfig) (*TelemetryCollector, error) {
	if config.Enabled && !config.OptIn {
		return nil, ErrTelemetryConsentRequired
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &TelemetryCollector{
		enabled:   config.Enabled,
		source:    strings.TrimSpace(config.Source),
		startedAt: now().UTC(),
		now:       now,
		counters:  make(map[telemetryKey]*TelemetryCounter),
	}, nil
}

// Enabled reports whether the collector records telemetry.
func (c *TelemetryCollector) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled
}

// Record adds an anonymous telemetry event. Disabled collectors are no-ops.
func (c *TelemetryCollector) Record(ctx context.Context, event TelemetryEvent) error {
	if c == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	pkg := strings.TrimSpace(event.Package)
	if pkg == "" {
		pkg = "uncategorized"
	}
	tool := strings.TrimSpace(event.Tool)
	if tool == "" {
		tool = "unknown"
	}
	errorCode := strings.TrimSpace(event.ErrorCode)

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled {
		return nil
	}

	key := telemetryKey{pkg: pkg, tool: tool}
	counter := c.counters[key]
	if counter == nil {
		counter = &TelemetryCounter{Package: pkg, Tool: tool}
		c.counters[key] = counter
	}
	counter.Calls++
	if errorCode != "" {
		counter.Errors++
		counter.LastErrorCode = errorCode
	}
	counter.LastSeenAt = c.now().UTC()
	counter.ErrorRate = float64(counter.Errors) / float64(counter.Calls)
	return nil
}

// Snapshot returns a sorted copy of the aggregate telemetry counters.
func (c *TelemetryCollector) Snapshot() TelemetrySnapshot {
	if c == nil {
		return TelemetrySnapshot{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	counters := make([]TelemetryCounter, 0, len(c.counters))
	for _, counter := range c.counters {
		counters = append(counters, *counter)
	}
	sort.Slice(counters, func(i, j int) bool {
		if counters[i].Package == counters[j].Package {
			return counters[i].Tool < counters[j].Tool
		}
		return counters[i].Package < counters[j].Package
	})

	return TelemetrySnapshot{
		Enabled:     c.enabled,
		Source:      c.source,
		StartedAt:   c.startedAt,
		GeneratedAt: c.now().UTC(),
		Counters:    counters,
	}
}

// Middleware records one anonymous event for each tool call. It stores package
// usage as the tool category when available, then falls back to runtime group
// and finally "uncategorized".
func (c *TelemetryCollector) Middleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			result, err := next(ctx, req)
			errorCode := ""
			switch {
			case err != nil:
				errorCode = "handler_error"
			case registry.IsResultError(result):
				errorCode = "tool_error"
			}
			_ = c.Record(ctx, TelemetryEvent{
				Package:   telemetryPackage(td),
				Tool:      name,
				ErrorCode: errorCode,
			})
			return result, err
		}
	}
}

func telemetryPackage(td registry.ToolDefinition) string {
	if strings.TrimSpace(td.Category) != "" {
		return td.Category
	}
	if strings.TrimSpace(td.RuntimeGroup) != "" {
		return td.RuntimeGroup
	}
	return "uncategorized"
}
