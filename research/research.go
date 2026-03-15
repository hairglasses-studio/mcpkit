
package research

import (
	"net/http"
	"time"

	"github.com/hairglasses-studio/mcpkit/client"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Config configures the research module.
type Config struct {
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// GitHubToken for authenticated GitHub API requests (higher rate limits).
	GitHubToken string

	// BaselineFeatures overrides the default feature matrix.
	BaselineFeatures []FeatureEntry

	// SourceOverrides maps source names to alternative URLs (useful for testing).
	SourceOverrides map[string]string
}

// Module implements registry.ToolModule for research tools.
type Module struct {
	httpClient       *http.Client
	githubToken      string
	baselineFeatures []FeatureEntry
	sourceOverrides  map[string]string
}

// NewModule creates a research module with the given configuration.
func NewModule(cfg ...Config) *Module {
	m := &Module{
		httpClient:       client.Standard(),
		baselineFeatures: defaultFeatureMatrix,
	}

	if len(cfg) > 0 {
		c := cfg[0]
		if c.HTTPClient != nil {
			m.httpClient = c.HTTPClient
		}
		if c.GitHubToken != "" {
			m.githubToken = c.GitHubToken
		}
		if len(c.BaselineFeatures) > 0 {
			m.baselineFeatures = c.BaselineFeatures
		}
		if len(c.SourceOverrides) > 0 {
			m.sourceOverrides = c.SourceOverrides
		}
	}

	return m
}

// Name returns the module name.
func (m *Module) Name() string { return "research" }

// Description returns the module description.
func (m *Module) Description() string {
	return "MCP ecosystem monitoring and viability assessment tools"
}

// Tools returns all research tool definitions.
func (m *Module) Tools() []registry.ToolDefinition {
	tools := []registry.ToolDefinition{
		m.specTool(),
		m.sdkReleasesTool(),
		m.ecosystemTool(),
		m.assessTool(),
		m.summaryTool(),
		m.githubActivityTool(),
		m.diffAnalysisTool(),
	}

	// Apply shared metadata
	for i := range tools {
		tools[i].Category = "research"
		tools[i].Timeout = 60 * time.Second
		tools[i].IsWrite = false
		tools[i].Complexity = registry.ComplexityModerate
	}

	return tools
}

// resolveURL returns the override URL if one exists, otherwise the default.
func (m *Module) resolveURL(name, defaultURL string) string {
	if m.sourceOverrides != nil {
		if override, ok := m.sourceOverrides[name]; ok {
			return override
		}
	}
	return defaultURL
}
