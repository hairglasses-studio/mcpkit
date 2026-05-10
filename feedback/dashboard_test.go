package feedback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDashboardExportRows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC)
	collector, err := NewTelemetryCollector(TelemetryConfig{
		Enabled: true,
		OptIn:   true,
		Source:  "dashboard-test",
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}
	if err := collector.Record(context.Background(), TelemetryEvent{Package: "feedback", Tool: ToolName}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := collector.Record(context.Background(), TelemetryEvent{Package: "feedback", Tool: ToolName, ErrorCode: "tool_error"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	export := collector.DashboardExport()
	if export.SchemaVersion != 1 || export.Source != "dashboard-test" {
		t.Fatalf("unexpected export metadata: %+v", export)
	}
	if len(export.Rows) != 1 {
		t.Fatalf("rows len = %d, want 1", len(export.Rows))
	}
	row := export.Rows[0]
	if row.Target != "feedback.feedback_submit" {
		t.Fatalf("target = %q", row.Target)
	}
	if row.Calls != 2 || row.Errors != 1 || row.ErrorRate != 0.5 {
		t.Fatalf("unexpected row counts: %+v", row)
	}
	if row.LastSeenUnixMS != now.UnixMilli() || row.GeneratedUnixMS != now.UnixMilli() {
		t.Fatalf("unexpected row timestamps: %+v", row)
	}
}

func TestDashboardJSON(t *testing.T) {
	t.Parallel()

	collector, err := NewTelemetryCollector(TelemetryConfig{Enabled: true, OptIn: true})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}
	data, err := collector.DashboardJSON()
	if err != nil {
		t.Fatalf("DashboardJSON: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("DashboardJSON returned invalid JSON: %s", data)
	}
}

func TestDashboardHandler(t *testing.T) {
	t.Parallel()

	collector, err := NewTelemetryCollector(TelemetryConfig{Enabled: true, OptIn: true})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}
	if err := collector.Record(context.Background(), TelemetryEvent{Package: "feedback", Tool: ToolName}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/feedback/telemetry", nil)
	rec := httptest.NewRecorder()
	DashboardHandler(collector).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
	var export DashboardExport
	if err := json.Unmarshal(rec.Body.Bytes(), &export); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(export.Rows) != 1 || export.Rows[0].Calls != 1 {
		t.Fatalf("unexpected export: %+v", export)
	}
}

func TestDashboardTarget(t *testing.T) {
	t.Parallel()

	if got := dashboardTarget("", "tool"); got != "tool" {
		t.Fatalf("target = %q", got)
	}
	if got := dashboardTarget("pkg", ""); got != "pkg" {
		t.Fatalf("target = %q", got)
	}
	if got := dashboardTarget("", ""); got != "unknown" {
		t.Fatalf("target = %q", got)
	}
}
