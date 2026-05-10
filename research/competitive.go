package research

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// A2AMonitor describes sources used to monitor the Agent2Agent protocol.
type A2AMonitor interface {
	Sources() []A2ASource
}

// A2ASource is an A2A source monitored for protocol changes.
type A2ASource struct {
	Name              string   `json:"name"`
	URL               string   `json:"url"`
	Keywords          []string `json:"keywords"`
	VersionKeywords   []string `json:"version_keywords,omitempty"`
	AgentCardKeywords []string `json:"agent_card_keywords,omitempty"`
	BreakingKeywords  []string `json:"breaking_keywords,omitempty"`
}

type staticA2AMonitor struct {
	sources []A2ASource
}

// NewA2AMonitor creates the default A2A protocol monitor.
func NewA2AMonitor() A2AMonitor {
	return staticA2AMonitor{sources: []A2ASource{
		{
			Name: "A2A Specification",
			URL:  "https://a2a-protocol.org/latest/specification/",
			Keywords: []string{
				"agent card", "send message", "streaming", "push notification",
				"grpc", "json-rpc", "http+json", "version", "extension",
			},
			VersionKeywords:   []string{"version", "v1.0", "a2a-version"},
			AgentCardKeywords: []string{"agent card", "agent skill", "well-known", "extended agent card"},
			BreakingKeywords:  []string{"breaking change", "removed", "deprecated", "migration"},
		},
		{
			Name: "A2A Releases",
			URL:  "https://github.com/a2aproject/A2A/releases",
			Keywords: []string{
				"release", "v1.0", "deprecated", "removed", "spec",
				"agent card", "breaking", "a2a.proto",
			},
			VersionKeywords:   []string{"v1.0", "release", "version"},
			AgentCardKeywords: []string{"agent card", "agentcard"},
			BreakingKeywords:  []string{"breaking", "deprecated", "removed"},
		},
		{
			Name: "A2A Samples",
			URL:  "https://github.com/a2aproject/a2a-samples",
			Keywords: []string{
				"samples", "agentcard", "agent card", "a2a-mcp", "server",
				"client", "python", "go", "javascript",
			},
			VersionKeywords:   []string{"version", "v1.0"},
			AgentCardKeywords: []string{"agentcard", "agent card", "skills.description"},
			BreakingKeywords:  []string{"untrusted input", "security", "prompt injection"},
		},
	}}
}

func (m staticA2AMonitor) Sources() []A2ASource {
	return append([]A2ASource(nil), m.sources...)
}

// A2ATrackingInput is the input for research_a2a_tracking.
type A2ATrackingInput struct {
	Sources      []string `json:"sources,omitempty" jsonschema:"description=A2A source names to scan (default: all)"`
	MaxBodyBytes int      `json:"max_body_bytes,omitempty" jsonschema:"description=Max bytes to fetch per source (default 65536)"`
}

// A2ATrackingOutput is the output of research_a2a_tracking.
type A2ATrackingOutput struct {
	Sources           []SourceFinding `json:"sources"`
	Signals           []A2ASignal     `json:"signals"`
	VersionCandidates []string        `json:"version_candidates"`
	AgentCardSignals  int             `json:"agent_card_signals"`
	BreakingSignals   int             `json:"breaking_signals"`
	ActionItems       []string        `json:"action_items"`
	Summary           string          `json:"summary"`
}

// A2ASignal is a protocol tracking finding.
type A2ASignal struct {
	Source   string  `json:"source"`
	URL      string  `json:"url"`
	Keyword  string  `json:"keyword"`
	Snippet  string  `json:"snippet"`
	Severity string  `json:"severity"`
	Score    float64 `json:"score"`
}

func (m *Module) a2aTrackingTool() registry.ToolDefinition {
	desc := "Track A2A protocol specification, release, and agent-card sample activity. " +
		"Detects version candidates, agent-card example changes, and breaking-change signals from official A2A sources." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Scan all default A2A sources",
				Input:       map[string]any{},
				Output:      "A2A source findings, version candidates, protocol signals, and action items",
			},
		})

	return handler.TypedHandler[A2ATrackingInput, A2ATrackingOutput](
		"research_a2a_tracking",
		desc,
		m.handleA2ATracking,
	)
}

func (m *Module) handleA2ATracking(ctx context.Context, input A2ATrackingInput) (A2ATrackingOutput, error) {
	maxBytes := input.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	out := A2ATrackingOutput{}
	for _, src := range resolveA2ASources(input.Sources) {
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
			out.Sources = append(out.Sources, finding)
			out.ActionItems = append(out.ActionItems, fmt.Sprintf("Restore A2A monitoring source %s: %s", src.Name, fetch.Error))
			continue
		}

		finding.KeywordHits, finding.Relevance = keywordHits(fetch.Body, src.Keywords)
		out.VersionCandidates = append(out.VersionCandidates, extractVersionCandidates(fetch.Body)...)
		signals := a2aSignals(src, finding, fetch.Body)
		for _, signal := range signals {
			switch signal.Severity {
			case "agent-card":
				out.AgentCardSignals++
			case "breaking":
				out.BreakingSignals++
			}
		}
		out.Signals = append(out.Signals, signals...)
		out.Sources = append(out.Sources, finding)
	}
	out.VersionCandidates = dedupeStrings(out.VersionCandidates)
	out.ActionItems = append(out.ActionItems, a2aActionItems(out)...)
	out.ActionItems = dedupeStrings(out.ActionItems)
	out.Summary = buildA2ASummary(out)
	return out, nil
}

func resolveA2ASources(requested []string) []A2ASource {
	sources := NewA2AMonitor().Sources()
	if len(requested) == 0 {
		return sources
	}
	var out []A2ASource
	for _, name := range requested {
		name = strings.TrimSpace(name)
		for _, src := range sources {
			if strings.EqualFold(name, src.Name) || strings.Contains(strings.ToLower(src.Name), strings.ToLower(name)) {
				out = append(out, src)
				break
			}
		}
	}
	return out
}

func a2aSignals(src A2ASource, finding SourceFinding, body string) []A2ASignal {
	var signals []A2ASignal
	for _, keyword := range src.VersionKeywords {
		signals = appendA2ASignal(signals, src, finding, body, keyword, "version", 0.6)
	}
	for _, keyword := range src.AgentCardKeywords {
		signals = appendA2ASignal(signals, src, finding, body, keyword, "agent-card", 0.7)
	}
	for _, keyword := range src.BreakingKeywords {
		signals = appendA2ASignal(signals, src, finding, body, keyword, "breaking", 0.9)
	}
	return signals
}

func appendA2ASignal(signals []A2ASignal, src A2ASource, finding SourceFinding, body, keyword, severity string, minScore float64) []A2ASignal {
	if !strings.Contains(strings.ToLower(body), strings.ToLower(keyword)) {
		return signals
	}
	return append(signals, A2ASignal{
		Source:   src.Name,
		URL:      finding.URL,
		Keyword:  keyword,
		Snippet:  extractSnippet(body, keyword, 180),
		Severity: severity,
		Score:    maxFloat(finding.Relevance, minScore),
	})
}

var versionCandidatePattern = regexp.MustCompile(`(?i)\b(?:v([0-9]+(?:\.[0-9]+){1,2})|version\s+([0-9]+(?:\.[0-9]+){1,2}))\b`)

func extractVersionCandidates(body string) []string {
	matches := versionCandidatePattern.FindAllStringSubmatch(body, -1)
	versions := make([]string, 0, len(matches))
	for _, match := range matches {
		for _, group := range match[1:] {
			if group != "" {
				versions = append(versions, "v"+group)
				break
			}
		}
	}
	return dedupeStrings(versions)
}

func a2aActionItems(out A2ATrackingOutput) []string {
	var items []string
	if len(out.VersionCandidates) > 0 {
		items = append(items, fmt.Sprintf("Validate mcpkit A2A compatibility against detected A2A versions: %s.", strings.Join(out.VersionCandidates, ", ")))
	}
	if out.BreakingSignals > 0 {
		items = append(items, fmt.Sprintf("Review %d A2A breaking-change signal%s before the next bridge release.", out.BreakingSignals, pluralSuffix(out.BreakingSignals, "", "s")))
	}
	if out.AgentCardSignals > 0 {
		items = append(items, "Refresh A2A AgentCard compatibility tests against current official examples.")
	}
	return items
}

func buildA2ASummary(out A2ATrackingOutput) string {
	return fmt.Sprintf("Scanned %d A2A source%s, found %d signal%s, %d version candidate%s, and %d breaking-change signal%s.",
		len(out.Sources),
		pluralSuffix(len(out.Sources), "", "s"),
		len(out.Signals),
		pluralSuffix(len(out.Signals), "", "s"),
		len(out.VersionCandidates),
		pluralSuffix(len(out.VersionCandidates), "", "s"),
		out.BreakingSignals,
		pluralSuffix(out.BreakingSignals, "", "s"),
	)
}

// SDKCompareInput is the input for research_sdk_compare.
type SDKCompareInput struct {
	Frameworks   []string `json:"frameworks,omitempty" jsonschema:"description=Framework names to compare (default: all)"`
	MaxBodyBytes int      `json:"max_body_bytes,omitempty" jsonschema:"description=Max bytes to fetch per framework source (default 65536)"`
}

// SDKCompareOutput is the output of research_sdk_compare.
type SDKCompareOutput struct {
	Frameworks  []SDKFrameworkFinding  `json:"frameworks"`
	Matrix      []SDKFeatureComparison `json:"matrix"`
	Reports     map[string]string      `json:"reports"`
	ActionItems []string               `json:"action_items"`
	Summary     string                 `json:"summary"`
}

// SDKFrameworkFinding summarizes one framework source fetch.
type SDKFrameworkFinding struct {
	Name        string   `json:"name"`
	Ecosystem   string   `json:"ecosystem"`
	URL         string   `json:"url"`
	StatusCode  int      `json:"status_code"`
	BodySize    int      `json:"body_size"`
	KeywordHits []string `json:"keyword_hits"`
	Relevance   float64  `json:"relevance"`
	Error       string   `json:"error,omitempty"`
}

// SDKFeatureComparison is one row in the cross-framework feature matrix.
type SDKFeatureComparison struct {
	Feature  string              `json:"feature"`
	Supports []SDKFeatureSupport `json:"supports"`
}

// SDKFeatureSupport is one framework's support state for a feature.
type SDKFeatureSupport struct {
	SDK        string          `json:"sdk"`
	Status     string          `json:"status"`
	Confidence ConfidenceLevel `json:"confidence"`
	Evidence   []string        `json:"evidence,omitempty"`
}

type sdkFrameworkSource struct {
	Name      string
	Ecosystem string
	URL       string
	Keywords  []string
}

type sdkFeatureSpec struct {
	Name     string
	Keywords []string
}

var defaultSDKCompareFrameworks = []sdkFrameworkSource{
	{
		Name:      "mcp-go",
		Ecosystem: "Go",
		URL:       "https://raw.githubusercontent.com/mark3labs/mcp-go/main/README.md",
		Keywords:  []string{"tools", "resources", "prompts", "streamable", "sse", "middleware", "logging", "sampling"},
	},
	{
		Name:      "go-sdk",
		Ecosystem: "Go",
		URL:       "https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/README.md",
		Keywords:  []string{"tools", "resources", "prompts", "streamable", "sse", "client", "server", "sampling"},
	},
	{
		Name:      "FastMCP",
		Ecosystem: "Python",
		URL:       "https://raw.githubusercontent.com/jlowin/fastmcp/main/README.md",
		Keywords:  []string{"tools", "resources", "prompts", "client", "server", "authentication", "middleware", "proxy"},
	},
	{
		Name:      "TypeScript SDK",
		Ecosystem: "TypeScript",
		URL:       "https://raw.githubusercontent.com/modelcontextprotocol/typescript-sdk/main/README.md",
		Keywords:  []string{"tools", "resources", "prompts", "streamable", "stdio", "sse", "oauth", "sampling"},
	},
}

var defaultSDKCompareFeatures = []sdkFeatureSpec{
	{Name: "Tools", Keywords: []string{"tools", "tool"}},
	{Name: "Resources", Keywords: []string{"resources", "resource"}},
	{Name: "Prompts", Keywords: []string{"prompts", "prompt"}},
	{Name: "Streamable HTTP", Keywords: []string{"streamable http", "streamable"}},
	{Name: "SSE Transport", Keywords: []string{"sse", "server-sent events"}},
	{Name: "OAuth", Keywords: []string{"oauth", "authorization"}},
	{Name: "Sampling", Keywords: []string{"sampling", "create message"}},
	{Name: "Elicitation", Keywords: []string{"elicitation", "elicit"}},
	{Name: "Middleware", Keywords: []string{"middleware", "interceptor"}},
	{Name: "Structured Output", Keywords: []string{"structured output", "output schema", "structuredcontent"}},
}

func (m *Module) sdkCompareTool() registry.ToolDefinition {
	desc := "Compare MCP SDK feature coverage across mcp-go, official Go SDK, FastMCP, and the TypeScript SDK. " +
		"Fetches official repository documentation, builds a cross-framework feature matrix, and emits template reports." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Compare all default SDKs",
				Input:       map[string]any{},
				Output:      "Feature matrix, competitive reports, and parity action items",
			},
		})

	return handler.TypedHandler[SDKCompareInput, SDKCompareOutput](
		"research_sdk_compare",
		desc,
		m.handleSDKCompare,
	)
}

func (m *Module) handleSDKCompare(ctx context.Context, input SDKCompareInput) (SDKCompareOutput, error) {
	maxBytes := input.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	frameworks := resolveSDKCompareFrameworks(input.Frameworks)
	out := SDKCompareOutput{}
	bodies := map[string]string{}
	for _, framework := range frameworks {
		url := m.resolveURL(framework.Name, framework.URL)
		fetch := m.fetchURL(ctx, url, maxBytes)
		finding := SDKFrameworkFinding{
			Name:       framework.Name,
			Ecosystem:  framework.Ecosystem,
			URL:        url,
			StatusCode: fetch.StatusCode,
			BodySize:   len(fetch.Body),
		}
		if fetch.Error != "" {
			finding.Error = fetch.Error
			out.ActionItems = append(out.ActionItems, fmt.Sprintf("Restore SDK comparison source %s: %s", framework.Name, fetch.Error))
			out.Frameworks = append(out.Frameworks, finding)
			continue
		}
		finding.KeywordHits, finding.Relevance = keywordHits(fetch.Body, framework.Keywords)
		bodies[framework.Name] = fetch.Body
		out.Frameworks = append(out.Frameworks, finding)
	}

	for _, feature := range defaultSDKCompareFeatures {
		row := SDKFeatureComparison{Feature: feature.Name}
		for _, framework := range frameworks {
			body, ok := bodies[framework.Name]
			if !ok {
				row.Supports = append(row.Supports, SDKFeatureSupport{SDK: framework.Name, Status: "unknown", Confidence: ConfidenceLow})
				continue
			}
			row.Supports = append(row.Supports, sdkFeatureSupport(framework.Name, feature, body))
		}
		out.Matrix = append(out.Matrix, row)
	}

	out.ActionItems = append(out.ActionItems, sdkCompareActionItems(out.Matrix)...)
	out.ActionItems = dedupeStrings(out.ActionItems)
	out.Summary = buildSDKCompareSummary(out)
	out.Reports = buildSDKCompareReports(out)
	return out, nil
}

func resolveSDKCompareFrameworks(requested []string) []sdkFrameworkSource {
	if len(requested) == 0 {
		return defaultSDKCompareFrameworks
	}
	var frameworks []sdkFrameworkSource
	for _, name := range requested {
		name = strings.TrimSpace(name)
		for _, framework := range defaultSDKCompareFrameworks {
			if strings.EqualFold(name, framework.Name) || strings.Contains(strings.ToLower(framework.Name), strings.ToLower(name)) {
				frameworks = append(frameworks, framework)
				break
			}
		}
	}
	return frameworks
}

func sdkFeatureSupport(sdk string, feature sdkFeatureSpec, body string) SDKFeatureSupport {
	hits, _ := keywordHits(body, feature.Keywords)
	status := "missing"
	confidence := ConfidenceLow
	switch {
	case len(hits) >= 2:
		status = "implemented"
		confidence = ConfidenceHigh
	case len(hits) == 1:
		status = "partial"
		confidence = ConfidenceMedium
	}
	evidence := make([]string, 0, min(len(hits), 2))
	for _, hit := range hits {
		if len(evidence) >= 2 {
			break
		}
		if snippet := extractSnippet(body, hit, 140); snippet != "" {
			evidence = append(evidence, snippet)
		}
	}
	return SDKFeatureSupport{
		SDK:        sdk,
		Status:     status,
		Confidence: confidence,
		Evidence:   evidence,
	}
}

func sdkCompareActionItems(matrix []SDKFeatureComparison) []string {
	var items []string
	for _, row := range matrix {
		var implemented []string
		var missing []string
		var mcpGoStatus string
		for _, support := range row.Supports {
			if support.SDK == "mcp-go" {
				mcpGoStatus = support.Status
			}
			if support.Status == "implemented" {
				implemented = append(implemented, support.SDK)
			}
			if support.Status == "missing" || support.Status == "unknown" {
				missing = append(missing, support.SDK)
			}
		}
		if mcpGoStatus != "" && mcpGoStatus != "implemented" && len(implemented) > 0 {
			items = append(items, fmt.Sprintf("Evaluate mcp-go parity for %s; implemented in %s.", row.Feature, strings.Join(implemented, ", ")))
			continue
		}
		if len(implemented) > 0 && len(missing) > 0 {
			items = append(items, fmt.Sprintf("Track SDK parity for %s; missing or unknown in %s.", row.Feature, strings.Join(missing, ", ")))
		}
	}
	return items
}

func buildSDKCompareSummary(out SDKCompareOutput) string {
	implemented, partial, missing := sdkCompareStatusCounts(out.Matrix)
	return fmt.Sprintf("Compared %d SDK framework%s across %d feature%s: %d implemented, %d partial, %d missing/unknown cells.",
		len(out.Frameworks),
		pluralSuffix(len(out.Frameworks), "", "s"),
		len(out.Matrix),
		pluralSuffix(len(out.Matrix), "", "s"),
		implemented,
		partial,
		missing,
	)
}

func sdkCompareStatusCounts(matrix []SDKFeatureComparison) (implemented, partial, missing int) {
	for _, row := range matrix {
		for _, support := range row.Supports {
			switch support.Status {
			case "implemented":
				implemented++
			case "partial":
				partial++
			default:
				missing++
			}
		}
	}
	return implemented, partial, missing
}

func buildSDKCompareReports(out SDKCompareOutput) map[string]string {
	var competitive strings.Builder
	competitive.WriteString("# SDK Competitive Summary\n\n")
	competitive.WriteString(out.Summary)
	competitive.WriteString("\n\n")
	for _, row := range out.Matrix {
		fmt.Fprintf(&competitive, "## %s\n", row.Feature)
		for _, support := range row.Supports {
			fmt.Fprintf(&competitive, "- %s: %s (%s)\n", support.SDK, support.Status, support.Confidence)
		}
		competitive.WriteString("\n")
	}

	var gaps strings.Builder
	gaps.WriteString("# SDK Gap Summary\n\n")
	if len(out.ActionItems) == 0 {
		gaps.WriteString("No SDK parity gaps detected.\n")
	} else {
		for _, item := range out.ActionItems {
			fmt.Fprintf(&gaps, "- %s\n", item)
		}
	}

	return map[string]string{
		"competitive_summary": competitive.String(),
		"gap_summary":         gaps.String(),
	}
}

// CompetitiveDashboardInput is the input for research_competitive_dashboard.
type CompetitiveDashboardInput struct {
	A2AFindings        *A2ATrackingOutput `json:"a2a_findings,omitempty" jsonschema:"description=Output from research_a2a_tracking"`
	SDKCompareFindings *SDKCompareOutput  `json:"sdk_compare_findings,omitempty" jsonschema:"description=Output from research_sdk_compare"`
}

// CompetitiveDashboardOutput is structured dashboard data for monitoring.
type CompetitiveDashboardOutput struct {
	GeneratedAt   string                    `json:"generated_at"`
	A2ASignals    int                       `json:"a2a_signals"`
	SDKFrameworks int                       `json:"sdk_frameworks"`
	SDKFeatures   int                       `json:"sdk_features"`
	Rows          []CompetitiveDashboardRow `json:"rows"`
	ActionItems   []string                  `json:"action_items"`
	Reports       map[string]string         `json:"reports"`
	Summary       string                    `json:"summary"`
}

// CompetitiveDashboardRow is one monitoring dashboard row.
type CompetitiveDashboardRow struct {
	Area     string  `json:"area"`
	Subject  string  `json:"subject"`
	Status   string  `json:"status"`
	Severity string  `json:"severity"`
	Details  string  `json:"details"`
	Score    float64 `json:"score"`
}

func (m *Module) competitiveDashboardTool() registry.ToolDefinition {
	desc := "Build dashboard-ready competitive research data from A2A tracking and SDK comparison outputs. " +
		"Returns normalized rows, action items, and markdown report templates for monitoring dashboards." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Generate a dashboard from prior research outputs",
				Input:       map[string]any{"a2a_findings": map[string]any{"summary": "A2A scan"}, "sdk_compare_findings": map[string]any{"summary": "SDK scan"}},
				Output:      "Dashboard rows, action items, and template reports",
			},
		})

	return handler.TypedHandler[CompetitiveDashboardInput, CompetitiveDashboardOutput](
		"research_competitive_dashboard",
		desc,
		m.handleCompetitiveDashboard,
	)
}

func (m *Module) handleCompetitiveDashboard(_ context.Context, input CompetitiveDashboardInput) (CompetitiveDashboardOutput, error) {
	out := CompetitiveDashboardOutput{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Reports:     map[string]string{},
	}
	if input.A2AFindings != nil {
		out.A2ASignals = len(input.A2AFindings.Signals)
		out.ActionItems = append(out.ActionItems, input.A2AFindings.ActionItems...)
		for _, signal := range input.A2AFindings.Signals {
			out.Rows = append(out.Rows, CompetitiveDashboardRow{
				Area:     "a2a",
				Subject:  signal.Source,
				Status:   signal.Keyword,
				Severity: signal.Severity,
				Details:  signal.Snippet,
				Score:    signal.Score,
			})
		}
		out.Reports["a2a_tracking"] = buildA2AReport(*input.A2AFindings)
	}
	if input.SDKCompareFindings != nil {
		out.SDKFrameworks = len(input.SDKCompareFindings.Frameworks)
		out.SDKFeatures = len(input.SDKCompareFindings.Matrix)
		out.ActionItems = append(out.ActionItems, input.SDKCompareFindings.ActionItems...)
		for _, row := range input.SDKCompareFindings.Matrix {
			for _, support := range row.Supports {
				out.Rows = append(out.Rows, CompetitiveDashboardRow{
					Area:     "sdk",
					Subject:  support.SDK + " / " + row.Feature,
					Status:   support.Status,
					Severity: string(support.Confidence),
					Details:  strings.Join(support.Evidence, " "),
					Score:    sdkStatusScore(support.Status),
				})
			}
		}
		for name, report := range input.SDKCompareFindings.Reports {
			out.Reports[name] = report
		}
	}
	out.ActionItems = dedupeStrings(out.ActionItems)
	out.Summary = fmt.Sprintf("Competitive dashboard contains %d row%s, %d A2A signal%s, and %d SDK feature%s.",
		len(out.Rows),
		pluralSuffix(len(out.Rows), "", "s"),
		out.A2ASignals,
		pluralSuffix(out.A2ASignals, "", "s"),
		out.SDKFeatures,
		pluralSuffix(out.SDKFeatures, "", "s"),
	)
	return out, nil
}

func buildA2AReport(out A2ATrackingOutput) string {
	var b strings.Builder
	b.WriteString("# A2A Tracking Summary\n\n")
	b.WriteString(out.Summary)
	b.WriteString("\n\n")
	if len(out.VersionCandidates) > 0 {
		fmt.Fprintf(&b, "Detected versions: %s\n\n", strings.Join(out.VersionCandidates, ", "))
	}
	for _, signal := range out.Signals {
		fmt.Fprintf(&b, "- [%s] %s from %s: %s\n", signal.Severity, signal.Keyword, signal.Source, signal.Snippet)
	}
	return b.String()
}

func sdkStatusScore(status string) float64 {
	switch status {
	case "implemented":
		return 1
	case "partial":
		return 0.5
	default:
		return 0
	}
}
