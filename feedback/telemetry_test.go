package feedback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestNewTelemetryCollectorRequiresOptIn(t *testing.T) {
	t.Parallel()

	_, err := NewTelemetryCollector(TelemetryConfig{Enabled: true})
	if !errors.Is(err, ErrTelemetryConsentRequired) {
		t.Fatalf("error = %v, want ErrTelemetryConsentRequired", err)
	}
}

func TestDisabledTelemetryCollectorNoops(t *testing.T) {
	t.Parallel()

	collector, err := NewTelemetryCollector(TelemetryConfig{Enabled: false})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}
	if collector.Enabled() {
		t.Fatal("collector should be disabled")
	}
	if err := collector.Record(context.Background(), TelemetryEvent{Package: "feedback", Tool: ToolName}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if got := len(collector.Snapshot().Counters); got != 0 {
		t.Fatalf("counters len = %d, want 0", got)
	}
}

func TestTelemetryCollectorAggregatesUsageAndErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	collector, err := NewTelemetryCollector(TelemetryConfig{
		Enabled: true,
		OptIn:   true,
		Source:  "unit-test",
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}

	events := []TelemetryEvent{
		{Package: "feedback", Tool: ToolName},
		{Package: "feedback", Tool: ToolName, ErrorCode: "tool_error"},
		{Package: "registry", Tool: "tool_catalog"},
	}
	for _, event := range events {
		if err := collector.Record(context.Background(), event); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	snapshot := collector.Snapshot()
	if !snapshot.Enabled || snapshot.Source != "unit-test" {
		t.Fatalf("unexpected snapshot metadata: %+v", snapshot)
	}
	if len(snapshot.Counters) != 2 {
		t.Fatalf("counters len = %d, want 2", len(snapshot.Counters))
	}
	first := snapshot.Counters[0]
	if first.Package != "feedback" || first.Tool != ToolName {
		t.Fatalf("first counter not sorted/normalized: %+v", first)
	}
	if first.Calls != 2 || first.Errors != 1 || first.ErrorRate != 0.5 || first.LastErrorCode != "tool_error" {
		t.Fatalf("unexpected feedback counter: %+v", first)
	}
}

func TestTelemetryMiddlewareRecordsResultErrors(t *testing.T) {
	t.Parallel()

	collector, err := NewTelemetryCollector(TelemetryConfig{Enabled: true, OptIn: true})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}

	middleware := collector.Middleware()
	next := func(context.Context, registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeErrorResult("failed"), nil
	}
	wrapped := middleware("feedback_submit", registry.ToolDefinition{Category: "feedback"}, next)
	if _, err := wrapped(context.Background(), registry.CallToolRequest{}); err != nil {
		t.Fatalf("wrapped: %v", err)
	}

	counters := collector.Snapshot().Counters
	if len(counters) != 1 {
		t.Fatalf("counters len = %d, want 1", len(counters))
	}
	if counters[0].Errors != 1 || counters[0].LastErrorCode != "tool_error" {
		t.Fatalf("unexpected counter: %+v", counters[0])
	}
}

func TestTelemetryMiddlewareRecordsHandlerErrors(t *testing.T) {
	t.Parallel()

	collector, err := NewTelemetryCollector(TelemetryConfig{Enabled: true, OptIn: true})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}

	middleware := collector.Middleware()
	nextErr := errors.New("boom")
	next := func(context.Context, registry.CallToolRequest) (*registry.CallToolResult, error) {
		return nil, nextErr
	}
	wrapped := middleware("unknown_tool", registry.ToolDefinition{RuntimeGroup: "runtime"}, next)
	if _, err := wrapped(context.Background(), registry.CallToolRequest{}); !errors.Is(err, nextErr) {
		t.Fatalf("wrapped error = %v, want %v", err, nextErr)
	}

	counter := collector.Snapshot().Counters[0]
	if counter.Package != "runtime" || counter.Tool != "unknown_tool" {
		t.Fatalf("unexpected counter identity: %+v", counter)
	}
	if counter.Errors != 1 || counter.LastErrorCode != "handler_error" {
		t.Fatalf("unexpected counter error data: %+v", counter)
	}
}
