package fleetinventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ParityMetrics is the clean-model core of the legacy agent-parity audit:
// per-repo provider-surface counts derived from a single FileIndex, plus the
// violations the fleet baseline cares about.
type ParityMetrics struct {
	// Instruction surfaces.
	AgentsMD         int `json:"agents_md_count"`
	AgentsOverrideMD int `json:"agents_override_md_count"`
	ClaudeMD         int `json:"claude_md_count"`

	// Retired provider mirrors (must be zero fleet-wide).
	GeminiMD            int `json:"gemini_md_count"`
	CopilotInstructions int `json:"copilot_instructions_count"`
	ClineRules          int `json:"clinerules_count"`

	// Skills.
	SkillSurfaceManifests int `json:"skill_surface_manifest_count"`
	CanonicalSkills       int `json:"canonical_skill_count"`
	GeneratedClaudeSkills int `json:"generated_claude_skill_count"`
	GeneratedPluginSkills int `json:"generated_plugin_skill_count"`

	// MCP configuration sources.
	MCPJSON            int `json:"mcp_json_count"`
	MCPServerCount     int `json:"repo_mcp_server_count"`
	CodexConfig        int `json:"codex_config_count"`
	CodexLocalProfiles int `json:"codex_project_local_profile_config_count"`
	WellKnownMCP       int `json:"well_known_mcp_count"`

	// Provider agents/hooks/markers.
	CodexAgents   int  `json:"codex_agent_count"`
	ClaudeAgents  int  `json:"claude_agent_count"`
	AgentsHooks   int  `json:"agents_hooks_count"`
	HasRoadmap    bool `json:"has_roadmap"`
	HasRalph      bool `json:"has_ralph"`
	RootClaudeSet bool `json:"root_claude_settings"`

	// Violations found while deriving the metrics.
	Violations []string `json:"violations,omitempty"`
}

// CollectParity derives ParityMetrics from a FileIndex. The only file it
// opens is .mcp.json (to count configured servers and detect [profiles.] in
// .codex/config.toml); everything else is path arithmetic on the index.
func CollectParity(idx *FileIndex) ParityMetrics {
	m := ParityMetrics{
		AgentsMD:         idx.CountBase("AGENTS.md"),
		AgentsOverrideMD: idx.CountBase("AGENTS.override.md"),
		ClaudeMD:         idx.CountBase("CLAUDE.md"),

		GeminiMD:            idx.CountBase("GEMINI.md"),
		CopilotInstructions: idx.CountSuffix(".github/copilot-instructions.md"),
		ClineRules:          idx.CountBase(".clinerules"),

		SkillSurfaceManifests: idx.CountSuffix(".agents/skills/surface.yaml"),
		MCPJSON:               idx.CountBase(".mcp.json"),
		CodexConfig:           idx.CountSuffix(".codex/config.toml"),
		WellKnownMCP:          idx.CountSuffix(".well-known/mcp.json"),

		HasRoadmap:    idx.CountBase("ROADMAP.md") > 0,
		RootClaudeSet: idx.Has(".claude/settings.json"),
	}
	for _, p := range idx.Paths {
		switch {
		case strings.Contains(p, ".agents/skills/") && strings.HasSuffix(p, "/SKILL.md"):
			m.CanonicalSkills++
		case strings.Contains(p, ".claude/skills/") && strings.HasSuffix(p, "/SKILL.md"):
			m.GeneratedClaudeSkills++
		case strings.Contains(p, "plugins/") && strings.HasSuffix(p, "/SKILL.md"):
			m.GeneratedPluginSkills++
		case strings.Contains(p, ".codex/agents/") && strings.HasSuffix(p, ".toml"):
			m.CodexAgents++
		case strings.Contains(p, ".agents/agents/") && strings.HasSuffix(p, "/agent.md"):
			m.ClaudeAgents++
		case strings.HasSuffix(p, ".agents/hooks.json"):
			m.AgentsHooks++
		case strings.HasPrefix(p, ".ralph/"):
			m.HasRalph = true
		}
	}

	m.MCPServerCount = countMCPServers(filepath.Join(idx.Dir, ".mcp.json"))
	m.CodexLocalProfiles = countCodexLocalProfiles(idx)

	if m.GeminiMD > 0 {
		m.Violations = append(m.Violations, "retired mirror present: GEMINI.md")
	}
	if m.CopilotInstructions > 0 {
		m.Violations = append(m.Violations, "retired mirror present: .github/copilot-instructions.md")
	}
	if m.ClineRules > 0 {
		m.Violations = append(m.Violations, "retired mirror present: .clinerules")
	}
	if m.CodexLocalProfiles > 0 {
		m.Violations = append(m.Violations, "repo-local .codex/config.toml defines [profiles.*]")
	}
	if m.AgentsMD == 0 && len(idx.Paths) > 0 {
		m.Violations = append(m.Violations, "missing AGENTS.md")
	}
	return m
}

func countMCPServers(mcpJSONPath string) int {
	raw, err := os.ReadFile(mcpJSONPath)
	if err != nil {
		return 0
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 0
	}
	return len(doc.MCPServers)
}

// countCodexLocalProfiles counts repo-local .codex/config.toml files that
// define [profiles.*] tables (unsupported; a fleet-baseline violation).
func countCodexLocalProfiles(idx *FileIndex) int {
	n := 0
	for _, p := range idx.Paths {
		if !strings.HasSuffix(p, ".codex/config.toml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(idx.Dir, filepath.FromSlash(p)))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "[profiles.") {
				n++
				break
			}
		}
	}
	return n
}
