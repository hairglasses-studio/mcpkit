
package research

import (
	"context"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// EcosystemInput is the input for the research_ecosystem tool.
type EcosystemInput struct {
	Sources      []string `json:"sources,omitempty" jsonschema:"description=Source names to fetch (e.g. 'MCP Spec' or 'Anthropic Blog'). Defaults to all if empty."`
	MaxBodyBytes int      `json:"max_body_bytes,omitempty" jsonschema:"description=Max bytes to fetch per source (default 65536)"`
}

// EcosystemOutput is the output of the research_ecosystem tool.
type EcosystemOutput struct {
	Sources    []SourceFinding `json:"sources"`
	Highlights []Highlight     `json:"highlights"`
}

// SourceFinding holds the analysis of a single ecosystem source.
type SourceFinding struct {
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	StatusCode   int      `json:"status_code"`
	BodySize     int      `json:"body_size"`
	KeywordHits  []string `json:"keyword_hits"`
	Relevance    float64  `json:"relevance"`
	Error        string   `json:"error,omitempty"`
}

// Highlight is a notable finding extracted from ecosystem sources.
type Highlight struct {
	Source  string  `json:"source"`
	Text   string  `json:"text"`
	Score  float64 `json:"score"`
}

func (m *Module) ecosystemTool() registry.ToolDefinition {
	desc := "Fetch and analyze MCP ecosystem sources for relevant developments. " +
		"Monitors spec pages, blog posts, and release pages for keyword matches and produces relevance scores." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Scan all default sources",
				Input:       map[string]any{},
				Output:      "Findings from 5 sources with relevance scores",
			},
			{
				Description: "Check only the spec",
				Input:       map[string]any{"sources": []any{"MCP Spec"}},
				Output:      "Spec-specific findings",
			},
		})

	return handler.TypedHandler[EcosystemInput, EcosystemOutput](
		"research_ecosystem",
		desc,
		m.handleEcosystem,
	)
}

func (m *Module) handleEcosystem(ctx context.Context, input EcosystemInput) (EcosystemOutput, error) {
	out := EcosystemOutput{}
	maxBytes := input.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	sources := m.resolveSources(input.Sources)

	for _, src := range sources {
		url := m.resolveURL(src.Name, src.URL)
		result := m.fetchURL(ctx, url, maxBytes)

		finding := SourceFinding{
			Name:       src.Name,
			URL:        url,
			StatusCode: result.StatusCode,
			BodySize:   len(result.Body),
		}

		if result.Error != "" {
			finding.Error = result.Error
			out.Sources = append(out.Sources, finding)
			continue
		}

		// Keyword matching
		bodyLower := strings.ToLower(result.Body)
		totalKeywords := len(src.Keywords)
		hits := 0
		for _, kw := range src.Keywords {
			if strings.Contains(bodyLower, strings.ToLower(kw)) {
				finding.KeywordHits = append(finding.KeywordHits, kw)
				hits++
			}
		}

		if totalKeywords > 0 {
			finding.Relevance = float64(hits) / float64(totalKeywords)
		}

		// Extract highlights for high-relevance findings
		if finding.Relevance > 0.5 {
			for _, kw := range finding.KeywordHits {
				snippet := extractSnippet(result.Body, kw, 120)
				if snippet != "" {
					out.Highlights = append(out.Highlights, Highlight{
						Source: src.Name,
						Text:   snippet,
						Score:  finding.Relevance,
					})
				}
			}
		}

		out.Sources = append(out.Sources, finding)
	}

	return out, nil
}

func (m *Module) resolveSources(requested []string) []EcosystemSource {
	if len(requested) == 0 {
		return defaultEcosystemSources
	}

	var sources []EcosystemSource
	for _, name := range requested {
		for _, src := range defaultEcosystemSources {
			if strings.EqualFold(src.Name, name) {
				sources = append(sources, src)
				break
			}
		}
	}
	return sources
}

// extractSnippet finds the first occurrence of keyword and returns surrounding context.
func extractSnippet(body, keyword string, contextLen int) string {
	bodyLower := strings.ToLower(body)
	kwLower := strings.ToLower(keyword)

	idx := strings.Index(bodyLower, kwLower)
	if idx < 0 {
		return ""
	}

	start := idx - contextLen/2
	if start < 0 {
		start = 0
	}
	end := idx + len(keyword) + contextLen/2
	if end > len(body) {
		end = len(body)
	}

	snippet := strings.TrimSpace(body[start:end])
	// Clean up whitespace
	snippet = strings.Join(strings.Fields(snippet), " ")

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(body) {
		snippet = snippet + "..."
	}

	return snippet
}
