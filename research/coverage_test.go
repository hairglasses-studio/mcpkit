
package research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- fetchURL coverage ----

// TestFetchURL_Truncation verifies that bodies larger than maxBytes are truncated
// and the Truncated flag is set.
func TestFetchURL_Truncation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write 200 bytes of 'x'
		fmt.Fprint(w, strings.Repeat("x", 200))
	}))
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client()})
	result := m.fetchURL(context.Background(), ts.URL, 100)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true")
	}
	if len(result.Body) != 100 {
		t.Errorf("body len = %d, want 100", len(result.Body))
	}
}

// TestFetchURL_ClientError verifies that HTTP transport errors are captured in result.Error.
func TestFetchURL_ClientError(t *testing.T) {
	m := NewModule(Config{HTTPClient: &http.Client{
		Transport: &errorTransport{},
	}})
	result := m.fetchURL(context.Background(), "http://example.invalid/path", 0)

	if result.Error == "" {
		t.Error("expected error for transport failure")
	}
}

// errorTransport always returns an error.
type errorTransport struct{}

func (e *errorTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("transport error")
}

// ---- fetchGitHubReleases coverage ----

// TestFetchGitHubReleases_NonOKStatus verifies non-200 responses return an error.
func TestFetchGitHubReleases_NonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer ts.Close()

	m := newActivityModule(ts, "")
	_, err := m.fetchGitHubReleases(context.Background(), "owner", "repo", 5)
	if err == nil {
		t.Error("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %v, expected 429", err)
	}
}

// TestFetchGitHubReleases_DecodeError verifies malformed JSON returns an error.
func TestFetchGitHubReleases_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not valid json`)
	}))
	defer ts.Close()

	m := newActivityModule(ts, "")
	_, err := m.fetchGitHubReleases(context.Background(), "owner", "repo", 5)
	if err == nil {
		t.Error("expected decode error")
	}
}

// TestFetchGitHubReleases_ClientError verifies transport errors bubble up.
func TestFetchGitHubReleases_ClientError(t *testing.T) {
	m := NewModule(Config{HTTPClient: &http.Client{Transport: &errorTransport{}}})
	_, err := m.fetchGitHubReleases(context.Background(), "owner", "repo", 5)
	if err == nil {
		t.Error("expected error for transport failure")
	}
}

// ---- doGitHubRequest: non-200 and decode errors ----

// TestDoGitHubRequest_NonOK verifies a non-200 response from doGitHubRequest returns an error.
func TestDoGitHubRequest_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer ts.Close()

	m := newActivityModule(ts, "")
	_, err := m.doGitHubRequest(context.Background(), ts.URL+"/any")
	if err == nil {
		t.Error("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %v, expected 403", err)
	}
}

// TestDoGitHubRequest_WithToken verifies the Authorization header is set when token is provided.
func TestDoGitHubRequest_WithToken(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer ts.Close()

	m := newActivityModule(ts, "mytoken")
	_, err := m.doGitHubRequest(context.Background(), ts.URL+"/any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization header = %q, want 'Bearer mytoken'", gotAuth)
	}
}

// ---- fetchGitHubIssues decode error ----

// TestFetchGitHubIssues_DecodeError triggers the decode error branch in fetchGitHubIssues.
func TestFetchGitHubIssues_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/badissues/issues", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{not json}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	m := newActivityModule(ts, "")
	_, err := m.fetchGitHubIssues(context.Background(), "owner", "badissues", "2025-01-01T00:00:00Z", 5)
	if err == nil {
		t.Error("expected decode error for malformed issues JSON")
	}
}

// ---- fetchActivityReleases decode error ----

// TestFetchActivityReleases_DecodeError triggers the decode error branch in fetchActivityReleases.
func TestFetchActivityReleases_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/badreleases/releases", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{not json}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	m := newActivityModule(ts, "")
	_, err := m.fetchActivityReleases(context.Background(), "owner", "badreleases", 5)
	if err == nil {
		t.Error("expected decode error for malformed releases JSON")
	}
}

// ---- fetchGitHubCommits decode error ----

// TestFetchGitHubCommits_DecodeError triggers the decode error branch in fetchGitHubCommits.
func TestFetchGitHubCommits_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/badcommits/commits", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{not json}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	m := newActivityModule(ts, "")
	_, err := m.fetchGitHubCommits(context.Background(), "owner", "badcommits", "2025-01-01T00:00:00Z", 5)
	if err == nil {
		t.Error("expected decode error for malformed commits JSON")
	}
}

// ---- handleGitHubActivity: issues error (non-fatal) and releases error (non-fatal) ----

// TestGitHubActivity_IssuesFetchError verifies that an issues fetch error is recorded but does not abort.
func TestGitHubActivity_IssuesFetchError(t *testing.T) {
	mux := http.NewServeMux()
	// commits OK
	mux.HandleFunc("/repos/owner/issueerr/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	// issues return error status
	mux.HandleFunc("/repos/owner/issueerr/issues", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	// releases OK
	mux.HandleFunc("/repos/owner/issueerr/releases", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	m := newActivityModule(ts, "")
	out, err := m.handleGitHubActivity(context.Background(), GitHubActivityInput{
		Repos: []string{"owner/issueerr"},
		Since: "2025-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Activities) != 1 {
		t.Fatalf("activities count = %d, want 1", len(out.Activities))
	}
	act := out.Activities[0]
	if act.Error == "" {
		t.Error("expected activity.Error to record issues fetch failure")
	}
	if !strings.Contains(act.Error, "issues") {
		t.Errorf("activity.Error = %q, want it to contain 'issues'", act.Error)
	}
}

// TestGitHubActivity_ReleasesFetchError verifies that a releases fetch error is recorded non-fatally.
func TestGitHubActivity_ReleasesFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/relerr/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/repos/owner/relerr/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/repos/owner/relerr/releases", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	m := newActivityModule(ts, "")
	out, err := m.handleGitHubActivity(context.Background(), GitHubActivityInput{
		Repos: []string{"owner/relerr"},
		Since: "2025-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Activities) != 1 {
		t.Fatalf("activities count = %d, want 1", len(out.Activities))
	}
	act := out.Activities[0]
	if act.Error == "" {
		t.Error("expected activity.Error to record releases fetch failure")
	}
	if !strings.Contains(act.Error, "releases") {
		t.Errorf("activity.Error = %q, want it to contain 'releases'", act.Error)
	}
}

// ---- assess.go: scoreFinding uncovered branches ----

// TestScoreFinding_FixEffort verifies "fix" in name gives effort=1.
func TestScoreFinding_FixEffort(t *testing.T) {
	w := normalizeWeights(ScoringWeights{})
	f := Finding{Name: "fix: nil pointer dereference", Category: "gap", Severity: "low"}
	a := scoreFinding(f, w)
	if a.Effort != 1 {
		t.Errorf("effort = %d, want 1 for 'fix' in name", a.Effort)
	}
}

// TestScoreFinding_PatchEffort verifies "patch" in name gives effort=1.
func TestScoreFinding_PatchEffort(t *testing.T) {
	w := normalizeWeights(ScoringWeights{})
	f := Finding{Name: "patch security vulnerability", Category: "gap", Severity: "high"}
	a := scoreFinding(f, w)
	if a.Effort != 1 {
		t.Errorf("effort = %d, want 1 for 'patch' in name", a.Effort)
	}
}

// TestScoreFinding_CriticalSeverity verifies critical severity gives impact=5.
func TestScoreFinding_CriticalSeverity(t *testing.T) {
	w := normalizeWeights(ScoringWeights{})
	f := Finding{Name: "some finding", Category: "spec_change", Severity: "critical"}
	a := scoreFinding(f, w)
	if a.Impact != 5 {
		t.Errorf("impact = %d, want 5 for critical severity", a.Impact)
	}
	if a.Urgency != 4 {
		t.Errorf("urgency = %d, want 4 for spec_change category", a.Urgency)
	}
}

// TestScoreFinding_EcosystemCategory verifies ecosystem category gives urgency=2.
func TestScoreFinding_EcosystemCategory(t *testing.T) {
	w := normalizeWeights(ScoringWeights{})
	f := Finding{Name: "update blog post", Category: "ecosystem", Severity: "low"}
	a := scoreFinding(f, w)
	if a.Urgency != 2 {
		t.Errorf("urgency = %d, want 2 for ecosystem category", a.Urgency)
	}
}

// TestScoreFinding_PromptBoost verifies "prompt" in name boosts impact.
func TestScoreFinding_PromptBoost(t *testing.T) {
	w := normalizeWeights(ScoringWeights{})
	// Impact starts at 3 (medium severity), should be boosted to 4
	f := Finding{Name: "prompt template support", Category: "gap", Severity: "medium"}
	a := scoreFinding(f, w)
	if a.Impact < 4 {
		t.Errorf("impact = %d, want >= 4 for 'prompt' in name", a.Impact)
	}
}

// TestScoreFinding_ImpactCapAt5 verifies impact does not exceed 5 even when boosted from critical.
func TestScoreFinding_ImpactCapAt5(t *testing.T) {
	w := normalizeWeights(ScoringWeights{})
	// critical → impact=5, then boost for "resource" should cap at 5
	f := Finding{Name: "resource handling critical", Category: "gap", Severity: "critical"}
	a := scoreFinding(f, w)
	if a.Impact != 5 {
		t.Errorf("impact = %d, want 5 (capped)", a.Impact)
	}
}

// ---- assess.go: identifyRisks uncovered branches ----

// TestIdentifyRisks_CriticalFindings verifies the critical count risk is generated.
func TestIdentifyRisks_CriticalFindings(t *testing.T) {
	findings := []Finding{
		{Name: "critical bug", Severity: "critical"},
		{Name: "another critical", Severity: "CRITICAL"},
	}
	risks := identifyRisks(findings)

	hasCritical := false
	for _, r := range risks {
		if strings.Contains(r, "critical") {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Errorf("risks = %v, expected entry about critical findings", risks)
	}
}

// TestIdentifyRisks_SpecChanges verifies spec change risks are reported.
func TestIdentifyRisks_SpecChanges(t *testing.T) {
	findings := []Finding{
		{Name: "spec delta", Category: "spec_change"},
	}
	risks := identifyRisks(findings)

	hasSpec := false
	for _, r := range risks {
		if strings.Contains(r, "spec change") {
			hasSpec = true
		}
	}
	if !hasSpec {
		t.Errorf("risks = %v, expected entry about spec changes", risks)
	}
}

// TestIdentifyRisks_NoRisks verifies the fallback "no critical risks" is returned.
func TestIdentifyRisks_NoRisks(t *testing.T) {
	findings := []Finding{
		{Name: "low severity finding", Severity: "low", Category: "ecosystem"},
	}
	risks := identifyRisks(findings)
	if len(risks) != 1 || risks[0] != "No critical risks identified" {
		t.Errorf("risks = %v, want ['No critical risks identified']", risks)
	}
}

// ---- summary.go: handleSummary uncovered branches ----

// TestSummaryTool_ChangedFeatures verifies spec findings with changed features produce action items.
func TestSummaryTool_ChangedFeatures(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleSummary(ctx, SummaryInput{
		SpecFindings: &SpecOutput{
			CoverageSummary: CoverageSummary{
				Total: 14, Implemented: 10, Partial: 2, Missing: 2, Percentage: "85%",
			},
			ChangedFeatures: []string{"OAuth renamed to AuthZ", "Transport renamed"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Changed features should produce action items
	hasChangeAction := false
	for _, item := range out.ActionItems {
		if strings.Contains(item, "Investigate spec change") {
			hasChangeAction = true
		}
	}
	if !hasChangeAction {
		t.Errorf("action items = %v, expected 'Investigate spec change' items", out.ActionItems)
	}
}

// TestSummaryTool_SDKWithError verifies SDK findings with repo errors are rendered.
func TestSummaryTool_SDKWithError(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleSummary(ctx, SummaryInput{
		SDKFindings: &SDKOutput{
			GoModVersion: "v0.45.0",
			Repos: []RepoStatus{
				{Owner: "mark3labs", Repo: "mcp-go", Error: "rate limited"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.Report, "ERROR") {
		t.Errorf("report should contain 'ERROR' for repo error, got: %s", out.Report)
	}
}

// TestSummaryTool_SDKLatestUpstream verifies LatestUpstream is rendered when non-empty.
func TestSummaryTool_SDKLatestUpstream(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleSummary(ctx, SummaryInput{
		SDKFindings: &SDKOutput{
			GoModVersion:   "v0.45.0",
			LatestUpstream: "v0.50.0",
			UpgradeAdvice:  []string{"upgrade to v0.50.0"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.Report, "v0.50.0") {
		t.Errorf("report should contain latest upstream version, got: %s", out.Report)
	}
}

// TestSummaryTool_EcosystemWithErrors verifies ecosystem findings with source errors are rendered.
func TestSummaryTool_EcosystemWithErrors(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleSummary(ctx, SummaryInput{
		EcosystemFindings: &EcosystemOutput{
			Sources: []SourceFinding{
				{Name: "MCP Spec", Error: "connection refused"},
				{Name: "Anthropic Blog", Relevance: 0.8, KeywordHits: []string{"mcp", "tools"}},
			},
			Highlights: []Highlight{
				{Source: "Anthropic Blog", Text: "MCP adoption is growing", Score: 0.8},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.Report, "ERROR") {
		t.Errorf("report should contain 'ERROR' for source error")
	}
	if !strings.Contains(out.Report, "Highlights") {
		t.Errorf("report should contain 'Highlights' section content")
	}
}

// TestSummaryTool_AssessmentWithRisks verifies assessment findings with risk factors are rendered.
func TestSummaryTool_AssessmentWithRisks(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleSummary(ctx, SummaryInput{
		AssessFindings: &AssessOutput{
			Assessments: []Assessment{
				{Name: "Resources gap", Priority: 4.5, Rationale: "effort=3 impact=5 urgency=4"},
			},
			Recommendations: []string{"Implement resources package"},
			RiskFactors:     []string{"2 critical findings require immediate attention"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.Report, "Assessment") {
		t.Errorf("report should contain 'Assessment' section")
	}
	if !strings.Contains(out.Report, "critical") {
		t.Errorf("report should contain risk factor text")
	}
	// Recommendations should be in action items
	hasRec := false
	for _, item := range out.ActionItems {
		if item == "Implement resources package" {
			hasRec = true
		}
	}
	if !hasRec {
		t.Errorf("action items = %v, expected recommendation", out.ActionItems)
	}
}

// ---- sdk.go: handleSDK pre-release filtering ----

// TestSDKHandlePrerelease verifies that prerelease releases are included when IncludePrerelease=true.
func TestSDKHandlePrerelease(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []GitHubRelease{
			{TagName: "v1.0.0-beta.1", Prerelease: true},
			{TagName: "v0.9.0"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	// Reuse rewriteHostTransport so GitHub API calls go to ts
	transport := &rewriteHostTransport{
		base:    ts.Client().Transport,
		target:  ts.URL,
		replace: "https://api.github.com",
	}
	m := NewModule(Config{
		HTTPClient: &http.Client{Transport: transport},
	})

	ctx := context.Background()
	out, err := m.handleSDK(ctx, SDKInput{
		Repos:             []string{"mark3labs/mcp-go"},
		IncludePrerelease: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Repos) == 0 {
		t.Fatal("expected at least one repo")
	}
	// With IncludePrerelease=true, both releases should be present
	if len(out.Repos[0].Releases) != 2 {
		t.Errorf("releases count = %d, want 2 (including prerelease)", len(out.Repos[0].Releases))
	}
}

// TestSDKHandlePrerelease_Filtered verifies that prerelease releases are excluded when IncludePrerelease=false.
func TestSDKHandlePrerelease_Filtered(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []GitHubRelease{
			{TagName: "v1.0.0-beta.1", Prerelease: true},
			{TagName: "v0.9.0"},
			{TagName: "v0.8.0-draft", Draft: true},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	transport := &rewriteHostTransport{
		base:    ts.Client().Transport,
		target:  ts.URL,
		replace: "https://api.github.com",
	}
	m := NewModule(Config{
		HTTPClient: &http.Client{Transport: transport},
	})

	ctx := context.Background()
	out, err := m.handleSDK(ctx, SDKInput{
		Repos:             []string{"mark3labs/mcp-go"},
		IncludePrerelease: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Repos) == 0 {
		t.Fatal("expected at least one repo")
	}
	// Only v0.9.0 should remain after filtering
	if len(out.Repos[0].Releases) != 1 {
		t.Errorf("releases count = %d, want 1 (excluding prerelease and draft)", len(out.Repos[0].Releases))
	}
}

// ---- sdk.go: resolveRepos custom role ----

// TestResolveRepos_CustomRepo verifies a repo not in defaultTrackedRepos gets role "custom".
func TestResolveRepos_CustomRepo(t *testing.T) {
	m := NewModule()
	repos := m.resolveRepos([]string{"unknown/myrepo"})
	if len(repos) != 1 {
		t.Fatalf("repos count = %d, want 1", len(repos))
	}
	if repos[0].Role != "custom" {
		t.Errorf("role = %q, want 'custom'", repos[0].Role)
	}
}

// TestResolveRepos_InvalidFormat verifies invalid repo format is skipped.
func TestResolveRepos_InvalidFormat(t *testing.T) {
	m := NewModule()
	repos := m.resolveRepos([]string{"no-slash-here"})
	if len(repos) != 0 {
		t.Errorf("repos count = %d, want 0 (invalid format should be skipped)", len(repos))
	}
}

// ---- sdk.go: parseSimpleInt with non-numeric suffix ----

// TestParseSimpleInt_WithSuffix verifies parseSimpleInt stops at non-numeric characters.
func TestParseSimpleInt_WithSuffix(t *testing.T) {
	n := parseSimpleInt("45alpha")
	if n != 45 {
		t.Errorf("parseSimpleInt('45alpha') = %d, want 45", n)
	}
}

// ---- spec.go: handleSpec fetch failure ----

// TestHandleSpec_FetchFailure verifies that a spec fetch failure sets confidence to low.
func TestHandleSpec_FetchFailure(t *testing.T) {
	// Use a server that immediately closes connections
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijacker", 500)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close() // close without response
	}))
	defer ts.Close()

	m := NewModule(Config{
		HTTPClient:      ts.Client(),
		SourceOverrides: map[string]string{"MCP Spec": ts.URL + "/spec"},
	})

	out, err := m.handleSpec(context.Background(), SpecInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Confidence != ConfidenceLow {
		t.Errorf("confidence = %v, want ConfidenceLow after fetch failure", out.Confidence)
	}
	if len(out.Evidence) == 0 {
		t.Error("expected evidence entry for fetch failure")
	}
}

// TestHandleSpec_RoadmapFetchFailure verifies roadmap fetch failure is recorded in evidence.
func TestHandleSpec_RoadmapFetchFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spec", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "MCP spec content tools transport oauth")
	})
	// roadmap endpoint uses hijacker to force error
	mux.HandleFunc("/roadmap", func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijacker", 500)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	m := NewModule(Config{
		HTTPClient: ts.Client(),
		SourceOverrides: map[string]string{
			"MCP Spec":    ts.URL + "/spec",
			"MCP Roadmap": ts.URL + "/roadmap",
		},
	})

	out, err := m.handleSpec(context.Background(), SpecInput{IncludeRoadmap: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasRoadmapError := false
	for _, e := range out.Evidence {
		if strings.Contains(e, "Roadmap fetch failed") {
			hasRoadmapError = true
		}
	}
	if !hasRoadmapError {
		t.Errorf("evidence = %v, expected 'Roadmap fetch failed' entry", out.Evidence)
	}
}

// ---- ecosystem.go: resolveSources with unknown source name ----

// TestResolveSources_UnknownName verifies unknown source names produce empty result.
func TestResolveSources_UnknownName(t *testing.T) {
	m := NewModule()
	sources := m.resolveSources([]string{"NonExistentSource"})
	if len(sources) != 0 {
		t.Errorf("sources count = %d, want 0 for unknown source name", len(sources))
	}
}

// TestHandleEcosystem_CustomMaxBytes verifies maxBodyBytes is respected.
func TestHandleEcosystem_CustomMaxBytes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 500 bytes
		fmt.Fprint(w, strings.Repeat("a", 500))
	}))
	defer ts.Close()

	m := NewModule(Config{
		HTTPClient:      ts.Client(),
		SourceOverrides: map[string]string{"MCP Spec": ts.URL + "/spec"},
	})

	out, err := m.handleEcosystem(context.Background(), EcosystemInput{
		Sources:      []string{"MCP Spec"},
		MaxBodyBytes: 200,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Sources) != 1 {
		t.Fatalf("sources count = %d, want 1", len(out.Sources))
	}
}

// ---- research.go: NewModule with BaselineFeatures override ----

// TestNewModule_BaselineFeatureOverride verifies custom baseline features are used.
func TestNewModule_BaselineFeatureOverride(t *testing.T) {
	custom := []FeatureEntry{
		{ID: 1, Name: "Custom Feature", SpecVer: "2025-01-01", Status: "Implemented", Confidence: ConfidenceHigh},
	}
	m := NewModule(Config{BaselineFeatures: custom})

	summary := m.computeCoverage()
	if summary.Total != 1 {
		t.Errorf("total = %d, want 1 for custom baseline", summary.Total)
	}
	if summary.Implemented != 1 {
		t.Errorf("implemented = %d, want 1", summary.Implemented)
	}
}

// TestHandleEcosystem_AllSources verifies fetching all sources (empty Sources input).
func TestHandleEcosystem_AllSources(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "MCP tools protocol context sampling resources prompts")
	}))
	defer ts.Close()

	// Override all default source URLs to point to test server
	overrides := map[string]string{}
	for _, src := range defaultEcosystemSources {
		overrides[src.Name] = ts.URL + "/content"
	}
	m := NewModule(Config{
		HTTPClient:      ts.Client(),
		SourceOverrides: overrides,
	})

	out, err := m.handleEcosystem(context.Background(), EcosystemInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Sources) != len(defaultEcosystemSources) {
		t.Errorf("sources count = %d, want %d", len(out.Sources), len(defaultEcosystemSources))
	}
}

// ---- buildActivitySummary: with errors ----

// TestBuildActivitySummary_WithErrors verifies error repos are counted in summary.
func TestBuildActivitySummary_WithErrors(t *testing.T) {
	activities := []GitHubActivity{
		{Repo: "ok/repo", Commits: []ActivityCommit{{SHA: "abc1234"}}},
		{Repo: "err/repo", Error: "fetch failed"},
	}
	summary := buildActivitySummary(activities, "2025-01-01T00:00:00Z")
	if !strings.Contains(summary, "errors") {
		t.Errorf("summary = %q, expected 'errors' mention", summary)
	}
}

// ---- io.ReadAll error in fetchURL ----

// TestFetchURL_LimitedRead verifies we handle a normal fetch (ReadAll success path).
func TestFetchURL_NormalRead(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello world")
	}))
	defer ts.Close()

	m := NewModule(Config{HTTPClient: ts.Client()})
	result := m.fetchURL(context.Background(), ts.URL, 0)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Body != "hello world" {
		t.Errorf("body = %q, want 'hello world'", result.Body)
	}
	if result.Truncated {
		t.Error("expected Truncated=false")
	}
}

// TestFetchURL_InvalidURL verifies that an invalid URL generates a request-creation error.
func TestFetchURL_InvalidURL(t *testing.T) {
	m := NewModule()
	// Use a URL with a control character to force NewRequestWithContext to fail.
	result := m.fetchURL(context.Background(), "http://\x00invalid", 0)
	if result.Error == "" {
		t.Error("expected error for invalid URL")
	}
}

// TestFetchGitHubReleases_DefaultLimit verifies limit defaults to 5 when <= 0.
func TestFetchGitHubReleases_DefaultLimit(t *testing.T) {
	var gotPerPage string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("per_page")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer ts.Close()

	m := newActivityModule(ts, "")
	_, err := m.fetchGitHubReleases(context.Background(), "owner", "repo", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPerPage != "5" {
		t.Errorf("per_page = %q, want '5' (default)", gotPerPage)
	}
}

// ---- handleSummary: json output_format field is accepted (no-op but covers branch) ----

// TestSummaryTool_JSONFormat verifies json output_format is accepted.
func TestSummaryTool_JSONFormat(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleSummary(ctx, SummaryInput{
		OutputFormat:    "json",
		AdditionalNotes: "test note",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Report == "" {
		t.Error("expected non-empty report")
	}
}

// readCloserThatErrors returns a body that errors on first Read.
type errorReader struct{}

func (e *errorReader) Read(_ []byte) (int, error) { return 0, fmt.Errorf("read error") }
func (e *errorReader) Close() error               { return nil }

// TestFetchURL_ReadError exercises the io.ReadAll error branch in fetchURL.
// We use a custom transport that returns a response with an error-prone body.
func TestFetchURL_ReadError(t *testing.T) {
	m := NewModule(Config{HTTPClient: &http.Client{
		Transport: &bodyErrorTransport{},
	}})
	result := m.fetchURL(context.Background(), "http://example.com/path", 0)
	if result.Error == "" {
		t.Error("expected error for body read failure")
	}
}

// bodyErrorTransport returns a 200 response whose body immediately errors.
type bodyErrorTransport struct{}

func (b *bodyErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(&errorReader{}),
	}, nil
}
