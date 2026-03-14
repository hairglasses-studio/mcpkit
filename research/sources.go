// Package research provides MCP tools for monitoring the MCP ecosystem
// and assessing viability for mcpkit. It fetches spec changes, SDK releases,
// ecosystem developments, and produces actionable assessments.
package research

// mcpGoVersion is the current mcp-go version used by mcpkit.
const mcpGoVersion = "v0.45.0"

// ConfidenceLevel indicates how reliable a finding is.
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"
	ConfidenceMedium ConfidenceLevel = "medium"
	ConfidenceLow    ConfidenceLevel = "low"
)

// FeatureEntry represents a tracked MCP protocol feature and its implementation status.
type FeatureEntry struct {
	ID         int             `json:"id"`
	Name       string          `json:"name"`
	SpecVer    string          `json:"spec_version"`
	Status     string          `json:"status"`
	Confidence ConfidenceLevel `json:"confidence"`
	Notes      string          `json:"notes"`
}

// TrackedRepo is a GitHub repository tracked for releases.
type TrackedRepo struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Description string `json:"description"`
	Role        string `json:"role"` // e.g., "foundation", "official-sdk", "competitor"
}

// EcosystemSource is a URL source for ecosystem monitoring.
type EcosystemSource struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Keywords []string `json:"keywords"`
}

// defaultFeatureMatrix is the baseline feature coverage from RESEARCH.md.
var defaultFeatureMatrix = []FeatureEntry{
	{1, "Tools (registration, middleware, search)", "Draft", "Implemented", ConfidenceHigh, "Core strength"},
	{2, "Tool Annotations (read-only, destructive, idempotent)", "2025-03-26", "Implemented", ConfidenceHigh, "Auto-inferred from ToolDefinition"},
	{3, "Structured Output (outputSchema + structuredContent)", "2025-11-25", "Implemented", ConfidenceHigh, "TypedHandler auto-generates outputSchema"},
	{4, "Elicitation (user input during tool execution)", "2025-11-25", "Implemented", ConfidenceHigh, "handler.ElicitForm, ElicitURL"},
	{5, "Tasks (async operations, status tracking)", "2025-11-25", "Implemented", ConfidenceMedium, "Task types in compat.go"},
	{6, "Deferred Tool Loading (lazy schema fetch)", "2025-11-25", "Implemented", ConfidenceHigh, "registry.RegisterDeferredModule"},
	{7, "OAuth 2.1 Authorization", "2025-03-26", "Partial", ConfidenceMedium, "Metadata only, missing token exchange"},
	{8, "Streamable HTTP Transport", "2025-03-26", "Delegated", ConfidenceMedium, "Handled by mcp-go"},
	{9, "Progress Reporting", "2025-11-25", "Partial", ConfidenceLow, "No dedicated middleware yet"},
	{10, "tools/list_changed Notifications", "2025-03-26", "Partial", ConfidenceMedium, "Dynamic registry exists"},
	{11, "Resources (URI-based data exposure)", "Draft", "Not implemented", ConfidenceLow, "Planned: Tier 1"},
	{12, "Prompts (reusable prompt templates)", "Draft", "Not implemented", ConfidenceLow, "Planned: Tier 1"},
	{13, "Sampling (LLM completion requests)", "Draft", "Not implemented", ConfidenceLow, "Planned: Tier 2"},
	{14, "Logging Endpoint", "Draft", "Not implemented", ConfidenceLow, "Planned: Tier 2"},
}

// defaultTrackedRepos are the GitHub repos we monitor for releases.
var defaultTrackedRepos = []TrackedRepo{
	{"mark3labs", "mcp-go", "Community Go MCP SDK (mcpkit foundation)", "foundation"},
	{"modelcontextprotocol", "go-sdk", "Official Go MCP SDK", "official-sdk"},
	{"modelcontextprotocol", "specification", "MCP specification", "spec"},
	{"jlowin", "fastmcp", "Python MCP framework", "competitor"},
	{"modelcontextprotocol", "typescript-sdk", "Official TypeScript SDK", "reference"},
}

// defaultEcosystemSources are URLs monitored for ecosystem developments.
var defaultEcosystemSources = []EcosystemSource{
	{"MCP Spec", "https://modelcontextprotocol.io/specification/2025-11-25", []string{"protocol", "transport", "tool", "resource", "prompt", "sampling"}},
	{"MCP Roadmap", "https://modelcontextprotocol.io/development/roadmap", []string{"registry", "namespace", "agent", "extension"}},
	{"Anthropic Blog", "https://www.anthropic.com/news", []string{"MCP", "model context protocol", "tool use", "agent"}},
	{"mcp-go Releases", "https://github.com/mark3labs/mcp-go/releases", []string{"release", "breaking", "feature", "fix"}},
	{"Go SDK Releases", "https://github.com/modelcontextprotocol/go-sdk/releases", []string{"release", "breaking", "feature"}},
}
