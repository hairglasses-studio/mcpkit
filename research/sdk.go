
package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// SDKInput is the input for the research_sdk_releases tool.
type SDKInput struct {
	Repos             []string `json:"repos,omitempty" jsonschema:"description=GitHub repos to check as 'owner/repo'. Defaults to tracked repos if empty."`
	IncludePrerelease bool     `json:"include_prerelease,omitempty" jsonschema:"description=Include pre-release versions (default false)"`
	ReleaseCount      int      `json:"release_count,omitempty" jsonschema:"description=Number of recent releases to fetch per repo (default 5)"`
}

// SDKOutput is the output of the research_sdk_releases tool.
type SDKOutput struct {
	Repos          []RepoStatus `json:"repos"`
	UpgradeAdvice  []string     `json:"upgrade_advice"`
	GoModVersion   string       `json:"go_mod_version"`
	LatestUpstream string       `json:"latest_upstream,omitempty"`
}

// RepoStatus holds release information for a tracked repository.
type RepoStatus struct {
	Owner       string          `json:"owner"`
	Repo        string          `json:"repo"`
	Role        string          `json:"role"`
	Releases    []GitHubRelease `json:"releases"`
	LatestTag   string          `json:"latest_tag"`
	Error       string          `json:"error,omitempty"`
	BehindCount int             `json:"behind_count,omitempty"`
}

func (m *Module) sdkReleasesTool() registry.ToolDefinition {
	desc := "Check GitHub releases for tracked MCP SDK repositories. " +
		"Compares latest versions against mcpkit's current mcp-go dependency and provides upgrade advice." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Check all tracked repos",
				Input:       map[string]any{},
				Output:      "Release status for mcp-go, go-sdk, fastmcp, etc.",
			},
			{
				Description: "Check specific repo with pre-releases",
				Input:       map[string]any{"repos": []any{"mark3labs/mcp-go"}, "include_prerelease": true},
				Output:      "mcp-go releases including pre-release versions",
			},
		})

	return handler.TypedHandler[SDKInput, SDKOutput](
		"research_sdk_releases",
		desc,
		m.handleSDK,
	)
}

func (m *Module) handleSDK(ctx context.Context, input SDKInput) (SDKOutput, error) {
	out := SDKOutput{
		GoModVersion: mcpGoVersion,
	}

	repos := m.resolveRepos(input.Repos)
	count := input.ReleaseCount
	if count <= 0 {
		count = 5
	}

	for _, repo := range repos {
		status := RepoStatus{
			Owner: repo.Owner,
			Repo:  repo.Repo,
			Role:  repo.Role,
		}

		releases, err := m.fetchGitHubReleases(ctx, repo.Owner, repo.Repo, count)
		if err != nil {
			status.Error = err.Error()
			out.Repos = append(out.Repos, status)
			continue
		}

		// Filter pre-releases if not requested
		if !input.IncludePrerelease {
			var filtered []GitHubRelease
			for _, r := range releases {
				if !r.Prerelease && !r.Draft {
					filtered = append(filtered, r)
				}
			}
			releases = filtered
		}

		status.Releases = releases
		if len(releases) > 0 {
			status.LatestTag = releases[0].TagName
		}

		// For the foundation repo, compute version delta
		if repo.Role == "foundation" && status.LatestTag != "" {
			status.BehindCount = estimateVersionDelta(mcpGoVersion, status.LatestTag)
			if status.BehindCount > 0 {
				out.UpgradeAdvice = append(out.UpgradeAdvice,
					fmt.Sprintf("mcp-go: current %s, latest %s (%d versions behind)",
						mcpGoVersion, status.LatestTag, status.BehindCount))
				out.LatestUpstream = status.LatestTag
			}
		}

		out.Repos = append(out.Repos, status)
	}

	if len(out.UpgradeAdvice) == 0 {
		out.UpgradeAdvice = append(out.UpgradeAdvice, "All tracked dependencies are up to date")
	}

	return out, nil
}

func (m *Module) resolveRepos(requested []string) []TrackedRepo {
	if len(requested) == 0 {
		return defaultTrackedRepos
	}

	var repos []TrackedRepo
	for _, r := range requested {
		parts := strings.SplitN(r, "/", 2)
		if len(parts) != 2 {
			continue
		}
		// Check if it's a known tracked repo
		role := "custom"
		for _, tr := range defaultTrackedRepos {
			if tr.Owner == parts[0] && tr.Repo == parts[1] {
				role = tr.Role
				break
			}
		}
		repos = append(repos, TrackedRepo{Owner: parts[0], Repo: parts[1], Role: role})
	}
	return repos
}

// estimateVersionDelta gives a rough count of how many minor versions apart two semver tags are.
func estimateVersionDelta(current, latest string) int {
	// Strip 'v' prefix
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	if len(currentParts) < 2 || len(latestParts) < 2 {
		return 0
	}

	// Compare minor versions (simple heuristic)
	currentMinor := parseSimpleInt(currentParts[1])
	latestMinor := parseSimpleInt(latestParts[1])

	if currentParts[0] != latestParts[0] {
		// Major version difference
		return latestMinor + 1
	}

	delta := latestMinor - currentMinor
	if delta < 0 {
		return 0
	}
	return delta
}

func parseSimpleInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
