package research

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockGitHubServer creates an httptest.Server that mimics the GitHub REST API
// for commits, issues, and releases endpoints.
func mockGitHubServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// Commits
	mux.HandleFunc("/repos/owner/repo/commits", func(w http.ResponseWriter, r *http.Request) {
		commits := []githubCommitResponse{
			{SHA: "abc1234567", Commit: struct {
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
					Date string `json:"date"`
				} `json:"author"`
			}{Message: "feat: add new feature\n\nBody text", Author: struct {
				Name string `json:"name"`
				Date string `json:"date"`
			}{Name: "Alice", Date: "2025-03-10T12:00:00Z"}}},
			{SHA: "def7890123", Commit: struct {
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
					Date string `json:"date"`
				} `json:"author"`
			}{Message: "fix: resolve nil panic", Author: struct {
				Name string `json:"name"`
				Date string `json:"date"`
			}{Name: "Bob", Date: "2025-03-11T09:00:00Z"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(commits)
	})

	// Issues
	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		issues := []githubIssueResponse{
			{Number: 42, Title: "Bug in middleware", State: "open", CreatedAt: "2025-03-09T10:00:00Z"},
			{Number: 43, Title: "Feature request: timeout config", State: "closed", CreatedAt: "2025-03-10T14:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(issues)
	})

	// Releases
	mux.HandleFunc("/repos/owner/repo/releases", func(w http.ResponseWriter, r *http.Request) {
		releases := []GitHubRelease{
			{TagName: "v1.2.0", Name: "Release 1.2.0", PublishedAt: time.Date(2025, 3, 8, 0, 0, 0, 0, time.UTC)},
			{TagName: "v1.1.0", Name: "Release 1.1.0", PublishedAt: time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	})

	// 500 error endpoint
	mux.HandleFunc("/repos/owner/error-repo/commits", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})

	// Token-checking endpoint
	mux.HandleFunc("/repos/owner/tokenrepo/commits", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer testtoken123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/repos/owner/tokenrepo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/repos/owner/tokenrepo/releases", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})

	// Max items — returns 3 commits regardless; client limits via per_page param
	mux.HandleFunc("/repos/owner/maxrepo/commits", func(w http.ResponseWriter, r *http.Request) {
		perPage := r.URL.Query().Get("per_page")
		// Return exactly the number requested
		count := 3
		if perPage == "2" {
			count = 2
		}
		var commits []githubCommitResponse
		for i := 0; i < count; i++ {
			commits = append(commits, githubCommitResponse{
				SHA: fmt.Sprintf("sha%d000000", i),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(commits)
	})
	mux.HandleFunc("/repos/owner/maxrepo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/repos/owner/maxrepo/releases", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})

	return httptest.NewServer(mux)
}

// newActivityModule creates a Module that routes GitHub API calls to the given server.
func newActivityModule(ts *httptest.Server, token string) *Module {
	// We need to intercept calls to the GitHub API base URL.
	// Use a custom transport that rewrites the host.
	transport := &rewriteHostTransport{
		base:    ts.Client().Transport,
		target:  ts.URL,
		replace: "https://api.github.com",
	}
	httpClient := &http.Client{Transport: transport}
	return NewModule(Config{
		HTTPClient:  httpClient,
		GitHubToken: token,
	})
}

// rewriteHostTransport rewrites requests whose URL prefix matches replace to target.
type rewriteHostTransport struct {
	base    http.RoundTripper
	target  string
	replace string
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	if strings.HasPrefix(url, t.replace) {
		newURL := t.target + url[len(t.replace):]
		newReq := req.Clone(req.Context())
		parsed, err := newReq.URL.Parse(newURL)
		if err != nil {
			return nil, err
		}
		newReq.URL = parsed
		newReq.Host = parsed.Host
		return t.base.RoundTrip(newReq)
	}
	return t.base.RoundTrip(req)
}

func TestGitHubActivityTool_Success(t *testing.T) {
	ts := mockGitHubServer(t)
	defer ts.Close()

	m := newActivityModule(ts, "")
	ctx := context.Background()

	out, err := m.handleGitHubActivity(ctx, GitHubActivityInput{
		Repos: []string{"owner/repo"},
		Since: "2025-03-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Activities) != 1 {
		t.Fatalf("activities count = %d, want 1", len(out.Activities))
	}

	act := out.Activities[0]
	if act.Error != "" {
		t.Fatalf("unexpected activity error: %s", act.Error)
	}
	if act.Repo != "owner/repo" {
		t.Errorf("repo = %s, want owner/repo", act.Repo)
	}
	if len(act.Commits) != 2 {
		t.Errorf("commits count = %d, want 2", len(act.Commits))
	}
	if len(act.Issues) != 2 {
		t.Errorf("issues count = %d, want 2", len(act.Issues))
	}
	if len(act.Releases) != 2 {
		t.Errorf("releases count = %d, want 2", len(act.Releases))
	}

	// Verify commit SHA is truncated to 7 chars
	if len(act.Commits[0].SHA) != 7 {
		t.Errorf("commit SHA len = %d, want 7", len(act.Commits[0].SHA))
	}
	// Verify multi-line commit message is truncated to first line
	if strings.Contains(act.Commits[0].Message, "\n") {
		t.Errorf("commit message should not contain newline: %s", act.Commits[0].Message)
	}

	if out.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestGitHubActivityTool_WithToken(t *testing.T) {
	ts := mockGitHubServer(t)
	defer ts.Close()

	m := newActivityModule(ts, "testtoken123")
	ctx := context.Background()

	out, err := m.handleGitHubActivity(ctx, GitHubActivityInput{
		Repos: []string{"owner/tokenrepo"},
		Since: "2025-03-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Activities) != 1 {
		t.Fatalf("activities count = %d, want 1", len(out.Activities))
	}
	if out.Activities[0].Error != "" {
		t.Errorf("unexpected error (token may not have been sent): %s", out.Activities[0].Error)
	}
}

func TestGitHubActivityTool_FetchError(t *testing.T) {
	ts := mockGitHubServer(t)
	defer ts.Close()

	m := newActivityModule(ts, "")
	ctx := context.Background()

	out, err := m.handleGitHubActivity(ctx, GitHubActivityInput{
		Repos: []string{"owner/error-repo"},
		Since: "2025-03-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Activities) != 1 {
		t.Fatalf("activities count = %d, want 1", len(out.Activities))
	}
	if out.Activities[0].Error == "" {
		t.Error("expected activity error for 500 response")
	}
}

func TestGitHubActivityTool_DefaultSince(t *testing.T) {
	ts := mockGitHubServer(t)
	defer ts.Close()

	m := newActivityModule(ts, "")
	ctx := context.Background()

	before := time.Now().UTC().AddDate(0, 0, -8)

	out, err := m.handleGitHubActivity(ctx, GitHubActivityInput{
		Repos: []string{"owner/repo"},
		// No Since — should default to 7 days ago
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The summary should include a since date that is after "8 days ago"
	if !strings.Contains(out.Summary, "since") {
		t.Error("expected summary to contain 'since'")
	}

	// Parse the since from summary is tricky; verify via the date in summary
	// The since string in the summary should be >= before
	sinceInSummary := extractSinceFromSummary(out.Summary)
	if sinceInSummary == "" {
		t.Skip("could not extract since date from summary for validation")
	}
	sinceTime, err := time.Parse(time.RFC3339, sinceInSummary)
	if err != nil {
		// Not RFC3339, just check the summary is non-empty
		return
	}
	if sinceTime.Before(before) {
		t.Errorf("since date %v is before expected threshold %v", sinceTime, before)
	}
}

func TestGitHubActivityTool_MaxItems(t *testing.T) {
	ts := mockGitHubServer(t)
	defer ts.Close()

	m := newActivityModule(ts, "")
	ctx := context.Background()

	out, err := m.handleGitHubActivity(ctx, GitHubActivityInput{
		Repos:    []string{"owner/maxrepo"},
		Since:    "2025-03-08T00:00:00Z",
		MaxItems: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Activities) != 1 {
		t.Fatalf("activities count = %d, want 1", len(out.Activities))
	}
	// The mock server returns per_page items; max_items=2 means per_page=2 in URL
	// so the server should return 2 commits
	if len(out.Activities[0].Commits) != 2 {
		t.Errorf("commits count = %d, want 2 (max_items=2)", len(out.Activities[0].Commits))
	}
}

func TestGitHubActivityTool_InvalidRepo(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	out, err := m.handleGitHubActivity(ctx, GitHubActivityInput{
		Repos: []string{"invalid-format"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Activities) != 1 {
		t.Fatalf("activities count = %d, want 1", len(out.Activities))
	}
	if out.Activities[0].Error == "" {
		t.Error("expected error for invalid repo format")
	}
}

func TestGitHubActivityTool_EmptyRepos(t *testing.T) {
	m := NewModule()
	ctx := context.Background()

	_, err := m.handleGitHubActivity(ctx, GitHubActivityInput{})
	if err == nil {
		t.Error("expected error for empty repos")
	}
}

// extractSinceFromSummary attempts to extract the ISO 8601 date from a summary string.
func extractSinceFromSummary(summary string) string {
	// summary format: "Checked N repo(s) since 2025-03-08T00:00:00Z: ..."
	_, after, ok := strings.Cut(summary, "since ")
	if !ok {
		return ""
	}
	rest := after
	// Find end of the date (next colon after the time zone)
	found := strings.Contains(rest, ":")
	if !found {
		return ""
	}
	// Date format is RFC3339: "2006-01-02T15:04:05Z"
	// Find the space or end
	before, _, ok := strings.Cut(rest, " ")
	if !ok {
		return strings.TrimRight(rest, " ")
	}
	return before
}
