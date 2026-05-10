package research

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func competitiveTestServer(t *testing.T) (*httptest.Server, map[string]string) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a-spec", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			A2A Specification Version 1.0.
			The Agent Card declares Agent Skill entries, well-known discovery, push notification support, JSON-RPC, HTTP+JSON, and gRPC.
			Migration guidance covers removed deprecated fields.
		`)
	})
	mux.HandleFunc("/a2a-releases", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			v1.0 release: removed deprecated fields from a2a.proto. Breaking migration notes mention AgentCard compatibility.
		`)
	})
	mux.HandleFunc("/a2a-samples", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			A2A samples include AgentCard data, a2a-mcp, server, client, Go, Python, and JavaScript.
			Treat skills.description as untrusted input to avoid prompt injection.
		`)
	})
	mux.HandleFunc("/mcp-go", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			mcp-go supports tools, resources, prompts, streamable HTTP, SSE, middleware, logging, and output schema.
		`)
	})
	mux.HandleFunc("/go-sdk", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			Official Go SDK includes client and server support for tools, resources, prompts, streamable HTTP, stdio, and sampling.
		`)
	})
	mux.HandleFunc("/fastmcp", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			FastMCP includes tools, resources, prompts, authentication, middleware, proxy support, elicitation, and structured output.
		`)
	})
	mux.HandleFunc("/typescript-sdk", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
			TypeScript SDK supports tools, resources, prompts, streamable HTTP, SSE, OAuth authorization, sampling, and elicitation.
		`)
	})

	ts := httptest.NewServer(mux)
	return ts, map[string]string{
		"A2A Specification": ts.URL + "/a2a-spec",
		"A2A Releases":      ts.URL + "/a2a-releases",
		"A2A Samples":       ts.URL + "/a2a-samples",
		"mcp-go":            ts.URL + "/mcp-go",
		"go-sdk":            ts.URL + "/go-sdk",
		"FastMCP":           ts.URL + "/fastmcp",
		"TypeScript SDK":    ts.URL + "/typescript-sdk",
	}
}

func TestA2ATrackingTool(t *testing.T) {
	ts, overrides := competitiveTestServer(t)
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client(), SourceOverrides: overrides})
	out, err := m.handleA2ATracking(context.Background(), A2ATrackingInput{})
	if err != nil {
		t.Fatalf("handleA2ATracking: %v", err)
	}
	if len(out.Sources) != 3 {
		t.Fatalf("sources count = %d, want 3", len(out.Sources))
	}
	if !containsString(out.VersionCandidates, "v1.0") {
		t.Fatalf("version candidates = %+v, want v1.0", out.VersionCandidates)
	}
	if out.AgentCardSignals == 0 {
		t.Fatal("expected agent-card signals")
	}
	if out.BreakingSignals == 0 {
		t.Fatal("expected breaking signals")
	}
	if len(out.ActionItems) == 0 {
		t.Fatal("expected A2A action items")
	}
}

func TestA2ATrackingTool_FiltersSources(t *testing.T) {
	ts, overrides := competitiveTestServer(t)
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client(), SourceOverrides: overrides})
	out, err := m.handleA2ATracking(context.Background(), A2ATrackingInput{
		Sources: []string{"releases"},
	})
	if err != nil {
		t.Fatalf("handleA2ATracking: %v", err)
	}
	if len(out.Sources) != 1 || out.Sources[0].Name != "A2A Releases" {
		t.Fatalf("unexpected sources: %+v", out.Sources)
	}
}

func TestSDKCompareTool(t *testing.T) {
	ts, overrides := competitiveTestServer(t)
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client(), SourceOverrides: overrides})
	out, err := m.handleSDKCompare(context.Background(), SDKCompareInput{})
	if err != nil {
		t.Fatalf("handleSDKCompare: %v", err)
	}
	if len(out.Frameworks) != 4 {
		t.Fatalf("framework count = %d, want 4", len(out.Frameworks))
	}
	if len(out.Matrix) != len(defaultSDKCompareFeatures) {
		t.Fatalf("matrix rows = %d, want %d", len(out.Matrix), len(defaultSDKCompareFeatures))
	}
	if out.Reports["competitive_summary"] == "" || out.Reports["gap_summary"] == "" {
		t.Fatalf("missing template reports: %+v", out.Reports)
	}
	if !strings.Contains(out.Summary, "Compared 4 SDK frameworks") {
		t.Fatalf("unexpected summary: %s", out.Summary)
	}
}

func TestSDKCompareTool_FiltersFrameworks(t *testing.T) {
	ts, overrides := competitiveTestServer(t)
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client(), SourceOverrides: overrides})
	out, err := m.handleSDKCompare(context.Background(), SDKCompareInput{
		Frameworks: []string{"fastmcp"},
	})
	if err != nil {
		t.Fatalf("handleSDKCompare: %v", err)
	}
	if len(out.Frameworks) != 1 || out.Frameworks[0].Name != "FastMCP" {
		t.Fatalf("unexpected frameworks: %+v", out.Frameworks)
	}
	for _, row := range out.Matrix {
		if len(row.Supports) != 1 || row.Supports[0].SDK != "FastMCP" {
			t.Fatalf("unexpected row supports: %+v", row.Supports)
		}
	}
}

func TestCompetitiveDashboardTool(t *testing.T) {
	m := NewModule()
	out, err := m.handleCompetitiveDashboard(context.Background(), CompetitiveDashboardInput{
		A2AFindings: &A2ATrackingOutput{
			Summary:           "A2A scan",
			VersionCandidates: []string{"v1.0"},
			Signals: []A2ASignal{
				{Source: "A2A Spec", Keyword: "v1.0", Snippet: "Version 1.0", Severity: "version", Score: 0.8},
			},
			ActionItems: []string{"Validate A2A v1.0."},
		},
		SDKCompareFindings: &SDKCompareOutput{
			Summary: "SDK scan",
			Frameworks: []SDKFrameworkFinding{
				{Name: "mcp-go"},
			},
			Matrix: []SDKFeatureComparison{
				{
					Feature: "Tools",
					Supports: []SDKFeatureSupport{
						{SDK: "mcp-go", Status: "implemented", Confidence: ConfidenceHigh, Evidence: []string{"tools"}},
					},
				},
			},
			Reports:     map[string]string{"competitive_summary": "# SDK"},
			ActionItems: []string{"Review SDK parity."},
		},
	})
	if err != nil {
		t.Fatalf("handleCompetitiveDashboard: %v", err)
	}
	if len(out.Rows) != 2 {
		t.Fatalf("rows count = %d, want 2", len(out.Rows))
	}
	if len(out.ActionItems) != 2 {
		t.Fatalf("action items = %+v, want 2", out.ActionItems)
	}
	if out.Reports["a2a_tracking"] == "" || out.Reports["competitive_summary"] == "" {
		t.Fatalf("missing reports: %+v", out.Reports)
	}
}

func TestDashboardJSONRunsCompetitiveResearch(t *testing.T) {
	ts, overrides := competitiveTestServer(t)
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client(), SourceOverrides: overrides})
	data, err := m.DashboardJSON(context.Background())
	if err != nil {
		t.Fatalf("DashboardJSON: %v", err)
	}
	var out CompetitiveDashboardOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if out.A2ASignals == 0 || out.SDKFrameworks != 4 || out.SDKFeatures == 0 {
		t.Fatalf("unexpected dashboard summary: %+v", out)
	}
	if len(out.Rows) == 0 {
		t.Fatal("expected dashboard rows")
	}
}

func TestSummaryIncludesCompetitiveFindings(t *testing.T) {
	m := NewModule()
	out, err := m.handleSummary(context.Background(), SummaryInput{
		A2AFindings: &A2ATrackingOutput{
			Summary:           "A2A scan",
			VersionCandidates: []string{"v1.0"},
			Signals: []A2ASignal{
				{Source: "A2A Spec", Keyword: "agent card", Snippet: "Agent Card", Severity: "agent-card"},
			},
			ActionItems: []string{"Refresh A2A AgentCard tests."},
		},
		SDKCompareFindings: &SDKCompareOutput{
			Summary: "SDK scan",
			Matrix: []SDKFeatureComparison{
				{Feature: "Tools", Supports: []SDKFeatureSupport{{SDK: "mcp-go", Status: "implemented"}}},
			},
			ActionItems: []string{"Review SDK parity."},
		},
		CompetitiveFindings: &CompetitiveDashboardOutput{
			Summary: "Competitive dashboard",
			Rows: []CompetitiveDashboardRow{
				{Area: "sdk", Subject: "mcp-go / Tools", Status: "implemented", Severity: "high"},
			},
			ActionItems: []string{"Publish dashboard."},
		},
	})
	if err != nil {
		t.Fatalf("handleSummary: %v", err)
	}
	for _, section := range []string{"A2A Tracking", "SDK Comparison", "Competitive Dashboard"} {
		if !strings.Contains(out.Report, section) {
			t.Fatalf("report missing %s: %s", section, out.Report)
		}
	}
	if len(out.ActionItems) != 3 {
		t.Fatalf("action item count = %d, want 3: %+v", len(out.ActionItems), out.ActionItems)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
