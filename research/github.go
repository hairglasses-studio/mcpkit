
package research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// GitHubActivityInput is the input for the research_github_activity tool.
type GitHubActivityInput struct {
	Repos    []string `json:"repos" jsonschema:"required,description=GitHub repos to check (owner/repo format)"`
	Since    string   `json:"since,omitempty" jsonschema:"description=ISO 8601 date to check activity since (default: 7 days ago)"`
	MaxItems int      `json:"max_items,omitempty" jsonschema:"description=Max items per repo (default 10)"`
}

// GitHubActivity holds activity data for a single repository.
type GitHubActivity struct {
	Repo     string           `json:"repo"`
	Commits  []ActivityCommit `json:"commits,omitempty"`
	Issues   []ActivityIssue  `json:"issues,omitempty"`
	Releases []ActivityRelease `json:"releases,omitempty"`
	Error    string           `json:"error,omitempty"`
}

// ActivityCommit is a simplified commit record for activity reporting.
type ActivityCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// ActivityIssue is a simplified issue record for activity reporting.
type ActivityIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Date   string `json:"date"`
}

// ActivityRelease is a simplified release record for activity reporting.
type ActivityRelease struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
	Date string `json:"date"`
}

// GitHubActivityOutput is the output of the research_github_activity tool.
type GitHubActivityOutput struct {
	Activities []GitHubActivity `json:"activities"`
	Summary    string           `json:"summary"`
}

// githubCommitResponse is the raw GitHub API response for a commit list item.
type githubCommitResponse struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// githubIssueResponse is the raw GitHub API response for an issue list item.
type githubIssueResponse struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

func (m *Module) githubActivityTool() registry.ToolDefinition {
	desc := "Monitor GitHub repository activity including commits, issues, and releases. " +
		"Fetches recent activity for each specified repository using the GitHub REST API. " +
		"Set a GitHub token in the module config for higher rate limits (5000 req/hr vs 60 req/hr)." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Check recent activity for mcp-go",
				Input: map[string]any{
					"repos": []any{"mark3labs/mcp-go"},
				},
				Output: "Commits, issues, and releases from the last 7 days",
			},
			{
				Description: "Check multiple repos since a specific date",
				Input: map[string]any{
					"repos":     []any{"mark3labs/mcp-go", "modelcontextprotocol/go-sdk"},
					"since":     "2025-03-01T00:00:00Z",
					"max_items": 5,
				},
				Output: "Activity summary for both repos",
			},
		})

	return handler.TypedHandler[GitHubActivityInput, GitHubActivityOutput](
		"research_github_activity",
		desc,
		m.handleGitHubActivity,
	)
}

func (m *Module) handleGitHubActivity(ctx context.Context, input GitHubActivityInput) (GitHubActivityOutput, error) {
	if len(input.Repos) == 0 {
		return GitHubActivityOutput{}, fmt.Errorf("at least one repo is required")
	}

	maxItems := input.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	}

	// Parse or default the since date
	since := input.Since
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	}

	out := GitHubActivityOutput{}

	for _, repo := range input.Repos {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 {
			out.Activities = append(out.Activities, GitHubActivity{
				Repo:  repo,
				Error: "invalid repo format: expected owner/repo",
			})
			continue
		}
		owner, repoName := parts[0], parts[1]

		activity := GitHubActivity{Repo: repo}

		// Fetch commits
		commits, err := m.fetchGitHubCommits(ctx, owner, repoName, since, maxItems)
		if err != nil {
			activity.Error = fmt.Sprintf("commits: %v", err)
			out.Activities = append(out.Activities, activity)
			continue
		}
		activity.Commits = commits

		// Fetch issues
		issues, err := m.fetchGitHubIssues(ctx, owner, repoName, since, maxItems)
		if err != nil {
			// Non-fatal: record error but continue
			activity.Error = fmt.Sprintf("issues: %v", err)
		} else {
			activity.Issues = issues
		}

		// Fetch releases
		releases, err := m.fetchActivityReleases(ctx, owner, repoName, maxItems)
		if err != nil {
			// Non-fatal
			if activity.Error == "" {
				activity.Error = fmt.Sprintf("releases: %v", err)
			}
		} else {
			activity.Releases = releases
		}

		out.Activities = append(out.Activities, activity)
	}

	out.Summary = buildActivitySummary(out.Activities, since)
	return out, nil
}

func (m *Module) fetchGitHubCommits(ctx context.Context, owner, repo, since string, limit int) ([]ActivityCommit, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?since=%s&per_page=%d",
		owner, repo, since, limit)

	body, err := m.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	var raw []githubCommitResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode commits: %w", err)
	}

	commits := make([]ActivityCommit, 0, len(raw))
	for _, c := range raw {
		msg := c.Commit.Message
		if idx := strings.Index(msg, "\n"); idx >= 0 {
			msg = msg[:idx]
		}
		commits = append(commits, ActivityCommit{
			SHA:     c.SHA[:min(7, len(c.SHA))],
			Message: msg,
			Author:  c.Commit.Author.Name,
			Date:    c.Commit.Author.Date,
		})
	}
	return commits, nil
}

func (m *Module) fetchGitHubIssues(ctx context.Context, owner, repo, since string, limit int) ([]ActivityIssue, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?since=%s&state=all&per_page=%d",
		owner, repo, since, limit)

	body, err := m.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	var raw []githubIssueResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode issues: %w", err)
	}

	issues := make([]ActivityIssue, 0, len(raw))
	for _, i := range raw {
		issues = append(issues, ActivityIssue{
			Number: i.Number,
			Title:  i.Title,
			State:  i.State,
			Date:   i.CreatedAt,
		})
	}
	return issues, nil
}

func (m *Module) fetchActivityReleases(ctx context.Context, owner, repo string, limit int) ([]ActivityRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d", owner, repo, limit)

	body, err := m.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	// Reuse the existing GitHubRelease type from fetch.go
	var raw []GitHubRelease
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	releases := make([]ActivityRelease, 0, len(raw))
	for _, r := range raw {
		date := ""
		if !r.PublishedAt.IsZero() {
			date = r.PublishedAt.UTC().Format(time.RFC3339)
		}
		releases = append(releases, ActivityRelease{
			Tag:  r.TagName,
			Name: r.Name,
			Date: date,
		})
	}
	return releases, nil
}

// doGitHubRequest performs an authenticated GET request to the GitHub API
// and returns the response body, or an error.
func (m *Module) doGitHubRequest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation: %w", err)
	}
	req.Header.Set("User-Agent", "mcpkit-research/1.0")
	req.Header.Set("Accept", "application/vnd.github+json")
	if m.githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.githubToken)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	return bodyBytes, nil
}

func buildActivitySummary(activities []GitHubActivity, since string) string {
	totalCommits, totalIssues, totalReleases, errors := 0, 0, 0, 0
	for _, a := range activities {
		if a.Error != "" {
			errors++
			continue
		}
		totalCommits += len(a.Commits)
		totalIssues += len(a.Issues)
		totalReleases += len(a.Releases)
	}

	s := fmt.Sprintf("Checked %d repo(s) since %s: %d commit(s), %d issue(s), %d release(s)",
		len(activities), since, totalCommits, totalIssues, totalReleases)
	if errors > 0 {
		s += fmt.Sprintf(" (%d repo(s) with errors)", errors)
	}
	return s
}

