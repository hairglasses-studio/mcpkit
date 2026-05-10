package rdcycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/feedback"
)

func TestHandleScan_DefaultRepos(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"mark3labs/mcp-go", "anthropics/anthropic-sdk-go"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	if out.RepoCount != 2 {
		t.Errorf("RepoCount: want 2, got %d", out.RepoCount)
	}
	// ActionItems may be empty if GitHub API returns no recent activity
	// (e.g. in CI without a token or during quiet periods).
	if out.ArtifactID == "" {
		t.Error("ArtifactID: expected non-empty")
	}
}

func TestHandleScan_InputReposOverride(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"default/repo"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{
		Repos: []string{"override/repo1", "override/repo2", "override/repo3"},
	})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	if out.RepoCount != 3 {
		t.Errorf("RepoCount: want 3, got %d", out.RepoCount)
	}
}

func TestHandleScan_NoRepos(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{})

	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	if out.RepoCount != 0 {
		t.Errorf("RepoCount: want 0, got %d", out.RepoCount)
	}
	if out.Summary == "" {
		t.Error("Summary: expected non-empty even with no repos")
	}
}

func TestHandleScan_SincePropagated(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"owner/repo"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{
		Since: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}
	// Summary should contain the since value.
	if out.Summary == "" {
		t.Error("Summary: expected non-empty")
	}
}

func TestHandleScan_ArtifactStored(t *testing.T) {
	t.Parallel()
	m := NewModule(CycleConfig{
		ScanRepos: []string{"a/b"},
	})

	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: unexpected error: %v", err)
	}

	artifact, ok := m.store.Get(out.ArtifactID)
	if !ok {
		t.Fatal("artifact not stored")
	}
	if artifact.Type != "scan" {
		t.Errorf("artifact Type: want %q, got %q", "scan", artifact.Type)
	}
}

func TestHandleScan_IncludesFeedbackSignals(t *testing.T) {
	t.Parallel()

	collector, err := feedback.NewTelemetryCollector(feedback.TelemetryConfig{
		Enabled: true,
		OptIn:   true,
		Source:  "rdcycle-test",
	})
	if err != nil {
		t.Fatalf("NewTelemetryCollector: %v", err)
	}
	for _, event := range []feedback.TelemetryEvent{
		{Package: "feedback", Tool: feedback.ToolName},
		{Package: "feedback", Tool: feedback.ToolName, ErrorCode: "tool_error"},
		{Package: "registry", Tool: "tool_catalog", ErrorCode: "handler_error"},
	} {
		if err := collector.Record(context.Background(), event); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	m := NewModule(CycleConfig{}, WithFeedbackTelemetry(collector))
	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: %v", err)
	}
	if out.FeedbackSignals == nil {
		t.Fatal("FeedbackSignals = nil")
	}
	if out.FeedbackSignals.TotalCalls != 3 || out.FeedbackSignals.TotalErrors != 2 {
		t.Fatalf("unexpected feedback signals: %+v", out.FeedbackSignals)
	}
	if len(out.FeedbackSignals.TopTargets) == 0 || out.FeedbackSignals.TopTargets[0].Errors == 0 {
		t.Fatalf("expected error-heavy top target: %+v", out.FeedbackSignals.TopTargets)
	}
	if !containsSubstring(out.ActionItems, "Review feedback telemetry") {
		t.Fatalf("missing feedback action item: %+v", out.ActionItems)
	}

	artifact, ok := m.store.Get(out.ArtifactID)
	if !ok {
		t.Fatal("scan artifact not stored")
	}
	if _, ok := artifact.Content["feedback_signals"]; !ok {
		t.Fatalf("artifact missing feedback_signals: %+v", artifact.Content)
	}
}

func TestHandleScan_IncludesResearchSignals(t *testing.T) {
	t.Parallel()

	provider := fakeResearchProvider{payload: researchDashboardExport{
		Summary:       "Competitive dashboard contains 3 rows.",
		A2ASignals:    2,
		SDKFrameworks: 4,
		SDKFeatures:   10,
		ActionItems: []string{
			"Validate mcpkit A2A compatibility against detected A2A versions: v1.0.",
			"Review SDK parity.",
		},
		Rows: []researchDashboardExportRow{
			{Area: "sdk", Subject: "mcp-go / Tools", Status: "implemented", Severity: "high", Score: 1},
			{Area: "a2a", Subject: "A2A Releases", Status: "removed", Severity: "breaking", Score: 0.95},
			{Area: "a2a", Subject: "A2A Specification", Status: "v1.0", Severity: "version", Score: 0.8},
		},
	}}

	m := NewModule(CycleConfig{}, WithResearchSignals(provider))
	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: %v", err)
	}
	if out.ResearchSignals == nil {
		t.Fatal("ResearchSignals = nil")
	}
	if out.ResearchSignals.A2ASignals != 2 || out.ResearchSignals.SDKFrameworks != 4 || out.ResearchSignals.SDKFeatures != 10 {
		t.Fatalf("unexpected research signals: %+v", out.ResearchSignals)
	}
	if out.ResearchSignals.A2ABreakingSignals != 1 {
		t.Fatalf("A2ABreakingSignals = %d, want 1", out.ResearchSignals.A2ABreakingSignals)
	}
	if len(out.ResearchSignals.TopRows) == 0 || out.ResearchSignals.TopRows[0].Severity != "breaking" {
		t.Fatalf("expected breaking row first: %+v", out.ResearchSignals.TopRows)
	}
	if !containsSubstring(out.ActionItems, "Validate mcpkit A2A compatibility") {
		t.Fatalf("missing research action item: %+v", out.ActionItems)
	}
	if !containsSubstring(out.ActionItems, "Prioritize A2A compatibility review") {
		t.Fatalf("missing breaking-signal action item: %+v", out.ActionItems)
	}

	artifact, ok := m.store.Get(out.ArtifactID)
	if !ok {
		t.Fatal("scan artifact not stored")
	}
	if _, ok := artifact.Content["research_signals"]; !ok {
		t.Fatalf("artifact missing research_signals: %+v", artifact.Content)
	}
}

func TestHandleScan_ResearchSignalsProviderError(t *testing.T) {
	t.Parallel()

	m := NewModule(CycleConfig{}, WithResearchSignals(fakeResearchProvider{err: fmt.Errorf("offline")}))
	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: %v", err)
	}
	if out.ResearchSignals == nil || out.ResearchSignals.Error == "" {
		t.Fatalf("expected research signal error: %+v", out.ResearchSignals)
	}
	if !containsSubstring(out.ActionItems, "Restore competitive research signal provider") {
		t.Fatalf("missing provider restore action item: %+v", out.ActionItems)
	}
}

func TestHandleScan_NoFeedbackCollector(t *testing.T) {
	t.Parallel()

	m := NewModule(CycleConfig{})
	out, err := m.handleScan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("handleScan: %v", err)
	}
	if out.FeedbackSignals != nil {
		t.Fatalf("FeedbackSignals = %+v, want nil", out.FeedbackSignals)
	}
}

func TestBuildScanSummary_SingleRepo(t *testing.T) {
	t.Parallel()
	s := buildScanSummary([]string{"owner/repo"}, "2026-01-01")
	if s == "" {
		t.Error("buildScanSummary: expected non-empty string")
	}
}

func TestBuildScanSummary_NoRepos(t *testing.T) {
	t.Parallel()
	s := buildScanSummary(nil, "2026-01-01")
	if s == "" {
		t.Error("buildScanSummary: expected non-empty string for no repos")
	}
}

func containsSubstring(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

type fakeResearchProvider struct {
	payload researchDashboardExport
	err     error
}

func (p fakeResearchProvider) DashboardJSON(context.Context) ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	return json.Marshal(p.payload)
}
