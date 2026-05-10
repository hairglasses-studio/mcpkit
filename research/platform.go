package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// PlatformMonitor describes a cloud-platform activity monitor.
type PlatformMonitor interface {
	Platform() string
	Category() string
	Sources() []PlatformSource
}

// PlatformSource is a platform source monitored for MCP-relevant activity.
type PlatformSource struct {
	Name               string   `json:"name"`
	URL                string   `json:"url"`
	Keywords           []string `json:"keywords"`
	HighSignalKeywords []string `json:"high_signal_keywords,omitempty"`
	BreakingKeywords   []string `json:"breaking_keywords,omitempty"`
}

// StaticPlatformMonitor is a declarative PlatformMonitor implementation.
type StaticPlatformMonitor struct {
	platform string
	category string
	sources  []PlatformSource
}

// NewStaticPlatformMonitor creates a declarative platform monitor.
func NewStaticPlatformMonitor(platform, category string, sources []PlatformSource) StaticPlatformMonitor {
	return StaticPlatformMonitor{
		platform: platform,
		category: category,
		sources:  append([]PlatformSource(nil), sources...),
	}
}

// Platform returns the monitor platform name.
func (m StaticPlatformMonitor) Platform() string { return m.platform }

// Category returns the monitor category.
func (m StaticPlatformMonitor) Category() string { return m.category }

// Sources returns the monitor sources.
func (m StaticPlatformMonitor) Sources() []PlatformSource {
	return append([]PlatformSource(nil), m.sources...)
}

// PlatformInput is the input for the research_platform_activity tool.
type PlatformInput struct {
	Platforms    []string `json:"platforms,omitempty" jsonschema:"description=Cloud platforms to scan by name (default: all)"`
	MaxBodyBytes int      `json:"max_body_bytes,omitempty" jsonschema:"description=Max bytes to fetch per source (default 65536)"`
}

// PlatformOutput is the output of the research_platform_activity tool.
type PlatformOutput struct {
	Results     []PlatformMonitorResult `json:"results"`
	Signals     []PlatformSignal        `json:"signals"`
	ActionItems []string                `json:"action_items"`
	Summary     string                  `json:"summary"`
}

// PlatformMonitorResult summarizes one platform monitor run.
type PlatformMonitorResult struct {
	Platform  string           `json:"platform"`
	Category  string           `json:"category"`
	Sources   []SourceFinding  `json:"sources"`
	Signals   []PlatformSignal `json:"signals"`
	Relevance float64          `json:"relevance"`
	Summary   string           `json:"summary"`
	Error     string           `json:"error,omitempty"`
}

// PlatformSignal is a high-signal platform finding.
type PlatformSignal struct {
	Platform string  `json:"platform"`
	Source   string  `json:"source"`
	URL      string  `json:"url"`
	Keyword  string  `json:"keyword"`
	Snippet  string  `json:"snippet"`
	Severity string  `json:"severity"`
	Score    float64 `json:"score"`
}

func (m *Module) platformActivityTool() registry.ToolDefinition {
	desc := "Monitor cloud-platform MCP activity across Cloudflare Workers, Vercel AI SDK, and Azure AI Foundry. " +
		"Fetches official changelog and documentation sources, extracts MCP-relevant signals, and produces action items." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Scan all default cloud platforms",
				Input:       map[string]any{},
				Output:      "Platform monitor results with high-signal MCP activity and action items",
			},
			{
				Description: "Scan only Vercel AI SDK sources",
				Input:       map[string]any{"platforms": []any{"Vercel AI SDK"}},
				Output:      "Vercel MCP/AI SDK signals from official sources",
			},
		})

	return handler.TypedHandler[PlatformInput, PlatformOutput](
		"research_platform_activity",
		desc,
		m.handlePlatformActivity,
	)
}

func (m *Module) handlePlatformActivity(ctx context.Context, input PlatformInput) (PlatformOutput, error) {
	maxBytes := input.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	monitors := resolvePlatformMonitors(input.Platforms)
	out := PlatformOutput{}
	for _, monitor := range monitors {
		result := m.runPlatformMonitor(ctx, monitor, maxBytes)
		out.Results = append(out.Results, result)
		out.Signals = append(out.Signals, result.Signals...)
		out.ActionItems = append(out.ActionItems, platformActionItems(result)...)
	}
	out.ActionItems = dedupeStrings(out.ActionItems)
	out.Summary = buildPlatformSummary(out.Results, len(out.Signals))
	return out, nil
}

func (m *Module) runPlatformMonitor(ctx context.Context, monitor PlatformMonitor, maxBytes int) PlatformMonitorResult {
	result := PlatformMonitorResult{
		Platform: monitor.Platform(),
		Category: monitor.Category(),
	}

	var relevanceTotal float64
	var successes int
	var errors []string

	for _, src := range monitor.Sources() {
		url := m.resolveURL(src.Name, src.URL)
		fetch := m.fetchURL(ctx, url, maxBytes)
		finding := SourceFinding{
			Name:       src.Name,
			URL:        url,
			StatusCode: fetch.StatusCode,
			BodySize:   len(fetch.Body),
		}
		if fetch.Error != "" {
			finding.Error = fetch.Error
			errors = append(errors, fmt.Sprintf("%s: %s", src.Name, fetch.Error))
			result.Sources = append(result.Sources, finding)
			continue
		}

		finding.KeywordHits, finding.Relevance = keywordHits(fetch.Body, src.Keywords)
		relevanceTotal += finding.Relevance
		successes++
		result.Signals = append(result.Signals, platformSignals(result.Platform, src, finding, fetch.Body)...)
		result.Sources = append(result.Sources, finding)
	}

	if successes > 0 {
		result.Relevance = relevanceTotal / float64(successes)
	}
	if successes == 0 && len(errors) > 0 {
		result.Error = strings.Join(errors, "; ")
	}
	result.Summary = fmt.Sprintf("%s: %.0f%% average relevance across %d source%s with %d signal%s.",
		result.Platform,
		result.Relevance*100,
		len(result.Sources),
		pluralSuffix(len(result.Sources), "", "s"),
		len(result.Signals),
		pluralSuffix(len(result.Signals), "", "s"),
	)
	return result
}

func keywordHits(body string, keywords []string) ([]string, float64) {
	bodyLower := strings.ToLower(body)
	hits := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		if strings.Contains(bodyLower, strings.ToLower(keyword)) {
			hits = append(hits, keyword)
		}
	}
	if len(keywords) == 0 {
		return hits, 0
	}
	return hits, float64(len(hits)) / float64(len(keywords))
}

func platformSignals(platform string, src PlatformSource, finding SourceFinding, body string) []PlatformSignal {
	var signals []PlatformSignal
	for _, keyword := range src.HighSignalKeywords {
		if !containsKeyword(finding.KeywordHits, keyword) && !strings.Contains(strings.ToLower(body), strings.ToLower(keyword)) {
			continue
		}
		signals = append(signals, PlatformSignal{
			Platform: platform,
			Source:   src.Name,
			URL:      finding.URL,
			Keyword:  keyword,
			Snippet:  extractSnippet(body, keyword, 180),
			Severity: "watch",
			Score:    maxFloat(finding.Relevance, 0.5),
		})
	}
	for _, keyword := range src.BreakingKeywords {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(keyword)) {
			continue
		}
		signals = append(signals, PlatformSignal{
			Platform: platform,
			Source:   src.Name,
			URL:      finding.URL,
			Keyword:  keyword,
			Snippet:  extractSnippet(body, keyword, 180),
			Severity: "breaking",
			Score:    maxFloat(finding.Relevance, 0.8),
		})
	}
	return signals
}

func platformActionItems(result PlatformMonitorResult) []string {
	if result.Error != "" {
		return []string{fmt.Sprintf("Restore %s platform monitoring: %s", result.Platform, result.Error)}
	}
	var items []string
	for _, signal := range result.Signals {
		switch signal.Severity {
		case "breaking":
			items = append(items, fmt.Sprintf("Review %s breaking-change signal from %s: %s", result.Platform, signal.Source, signal.Keyword))
		default:
			items = append(items, fmt.Sprintf("Assess %s MCP platform signal from %s: %s", result.Platform, signal.Source, signal.Keyword))
		}
	}
	if len(items) == 0 && result.Relevance >= 0.5 {
		items = append(items, fmt.Sprintf("Review %s platform updates for roadmap impact.", result.Platform))
	}
	return dedupeStrings(items)
}

func buildPlatformSummary(results []PlatformMonitorResult, signalCount int) string {
	if len(results) == 0 {
		return "No cloud platform monitors matched the request."
	}
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Platform)
	}
	return fmt.Sprintf("Scanned %d cloud platform%s (%s) and found %d high-signal item%s.",
		len(results),
		pluralSuffix(len(results), "", "s"),
		strings.Join(names, ", "),
		signalCount,
		pluralSuffix(signalCount, "", "s"),
	)
}

func pluralSuffix(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func resolvePlatformMonitors(requested []string) []PlatformMonitor {
	defaults := []PlatformMonitor{
		NewCloudflareWorkersMonitor(),
		NewVercelAIMonitor(),
		NewAzureFoundryMonitor(),
	}
	if len(requested) == 0 {
		return defaults
	}

	var monitors []PlatformMonitor
	for _, name := range requested {
		name = strings.TrimSpace(name)
		for _, monitor := range defaults {
			if platformMatches(name, monitor.Platform()) {
				monitors = append(monitors, monitor)
				break
			}
		}
	}
	return monitors
}

func platformMatches(requested, platform string) bool {
	if strings.EqualFold(requested, platform) {
		return true
	}
	req := strings.ToLower(requested)
	plat := strings.ToLower(platform)
	return req != "" && (strings.Contains(plat, req) || strings.Contains(req, plat))
}

func containsKeyword(haystack []string, needle string) bool {
	for _, item := range haystack {
		if strings.EqualFold(item, needle) {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// NewCloudflareWorkersMonitor monitors Cloudflare Workers and Agents MCP activity.
func NewCloudflareWorkersMonitor() PlatformMonitor {
	return NewStaticPlatformMonitor("Cloudflare Workers", "cloud-platform", []PlatformSource{
		{
			Name: "Cloudflare MCP Blog",
			URL:  "https://blog.cloudflare.com/tag/mcp/",
			Keywords: []string{
				"mcp", "model context protocol", "agents", "cloudflare workers",
				"remote mcp", "code mode", "ai gateway", "workers ai",
			},
			HighSignalKeywords: []string{"mcp", "remote mcp", "code mode", "ai gateway"},
			BreakingKeywords:   []string{"breaking", "deprecated", "migration", "requires"},
		},
		{
			Name: "Cloudflare Workers Changelog",
			URL:  "https://developers.cloudflare.com/changelog/product/workers/",
			Keywords: []string{
				"mcp", "model context protocol", "agents sdk", "workers", "durable object",
				"oauth", "rpc transport", "schema converter", "waitForMcpConnections",
			},
			HighSignalKeywords: []string{"mcp", "agents sdk", "waitForMcpConnections", "rpc transport"},
			BreakingKeywords:   []string{"breaking", "deprecated", "no longer", "requires"},
		},
		{
			Name: "Cloudflare Workers Runtime Changelog",
			URL:  "https://developers.cloudflare.com/workers/platform/changelog/index.md",
			Keywords: []string{
				"workers", "runtime", "websocket", "compatibility", "durable object",
				"stream", "fetch", "module", "breaking",
			},
			HighSignalKeywords: []string{"websocket", "stream", "durable object"},
			BreakingKeywords:   []string{"breaking", "deprecated", "compatibility flag", "no longer"},
		},
	})
}

// NewVercelAIMonitor monitors Vercel AI SDK and MCP hosting activity.
func NewVercelAIMonitor() PlatformMonitor {
	return NewStaticPlatformMonitor("Vercel AI SDK", "cloud-platform", []PlatformSource{
		{
			Name: "Vercel Changelog",
			URL:  "https://vercel.com/changelog",
			Keywords: []string{
				"mcp", "model context protocol", "ai sdk", "ai gateway", "oauth",
				"streamable http", "adapter", "tools", "resources", "prompts",
			},
			HighSignalKeywords: []string{"mcp", "ai sdk", "ai gateway", "oauth"},
			BreakingKeywords:   []string{"breaking", "deprecated", "migration", "removed"},
		},
		{
			Name: "AI SDK MCP Docs",
			URL:  "https://ai-sdk.dev/docs/ai-sdk-core/mcp-tools",
			Keywords: []string{
				"mcp", "createMCPClient", "streamablehttpclienttransport", "stdio",
				"resources", "prompts", "elicitation", "oauth", "tools",
			},
			HighSignalKeywords: []string{"createMCPClient", "resources", "prompts", "elicitation", "oauth"},
			BreakingKeywords:   []string{"does not support", "experimental", "deprecated"},
		},
		{
			Name: "Vercel AI SDK Releases",
			URL:  "https://github.com/vercel/ai/releases",
			Keywords: []string{
				"ai sdk", "mcp", "release", "patch changes", "breaking", "provider",
				"tool", "stream", "gateway",
			},
			HighSignalKeywords: []string{"mcp", "breaking", "gateway"},
			BreakingKeywords:   []string{"breaking", "major", "removed"},
		},
	})
}

// NewAzureFoundryMonitor monitors Microsoft Foundry MCP activity.
func NewAzureFoundryMonitor() PlatformMonitor {
	return NewStaticPlatformMonitor("Azure AI Foundry", "cloud-platform", []PlatformSource{
		{
			Name: "Azure Foundry MCP Get Started",
			URL:  "https://learn.microsoft.com/en-us/azure/ai-foundry/mcp/get-started?view=foundry",
			Keywords: []string{
				"foundry mcp server", "model context protocol", "mcp.ai.azure.com",
				"entra", "preview", "tools", "authentication", "agent mode",
			},
			HighSignalKeywords: []string{"foundry mcp server", "mcp.ai.azure.com", "preview", "entra"},
			BreakingKeywords:   []string{"preview", "not recommend", "constrained capabilities"},
		},
		{
			Name: "Azure Foundry MCP Server Registration",
			URL:  "https://learn.microsoft.com/en-us/azure/ai-foundry/mcp/build-your-own-mcp-server?view=foundry",
			Keywords: []string{
				"model context protocol", "azure api center", "tool catalog", "remote mcp",
				"agent service", "organizational", "authentication", "oauth",
			},
			HighSignalKeywords: []string{"azure api center", "tool catalog", "remote mcp", "agent service"},
			BreakingKeywords:   []string{"currently no", "preview", "breaking"},
		},
		{
			Name: "Azure AI Foundry Agent What's New",
			URL:  "https://learn.microsoft.com/en-us/azure/ai-services/agents/whats-new?azure-portal=true",
			Keywords: []string{
				"model context protocol", "mcp tool", "remote", "agent service",
				"preview", "tools", "foundry", "availability",
			},
			HighSignalKeywords: []string{"model context protocol", "mcp tool", "agent service"},
			BreakingKeywords:   []string{"breaking", "deprecated", "removed"},
		},
	})
}
