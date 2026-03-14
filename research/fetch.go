//go:build !official_sdk

package research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultMaxBytes = 64 * 1024 // 64KB

// fetchResult holds the outcome of an HTTP fetch.
type fetchResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
	Truncated  bool   `json:"truncated"`
	Error      string `json:"error,omitempty"`
}

// fetchURL fetches a URL and returns the body, truncated to maxBytes.
func (m *Module) fetchURL(ctx context.Context, url string, maxBytes int) fetchResult {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fetchResult{URL: url, Error: fmt.Sprintf("request creation failed: %v", err)}
	}
	req.Header.Set("User-Agent", "mcpkit-research/1.0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fetchResult{URL: url, Error: fmt.Sprintf("fetch failed: %v", err)}
	}
	defer resp.Body.Close()

	// Read up to maxBytes+1 to detect truncation
	limited := io.LimitReader(resp.Body, int64(maxBytes+1))
	body, err := io.ReadAll(limited)
	if err != nil {
		return fetchResult{URL: url, StatusCode: resp.StatusCode, Error: fmt.Sprintf("read failed: %v", err)}
	}

	truncated := len(body) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}

	return fetchResult{
		URL:        url,
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Truncated:  truncated,
	}
}

// GitHubRelease represents a GitHub release from the API.
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

// fetchGitHubReleases fetches recent releases from a GitHub repository.
func (m *Module) fetchGitHubReleases(ctx context.Context, owner, repo string, limit int) ([]GitHubRelease, error) {
	if limit <= 0 {
		limit = 5
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d", owner, repo, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return releases, nil
}
