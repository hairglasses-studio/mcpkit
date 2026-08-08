package fleetinventory

import (
	"regexp"
	"strings"

	"github.com/hairglasses-studio/mcpkit/surfaceinventory"
)

// Security static analysis of MCP tool/resource/prompt surfaces, encoding the
// statically-checkable subset of the OWASP MCP Top 10 plus MCP tool-name spec
// validity. Purely static (operates on already-extracted descriptions/names);
// the live-interrogation and semantic checks are tracked in the roadmap.
//
// A fleet-wide inventory is itself the mitigation for OWASP MCP09 (Shadow MCP
// Servers) — enumerating every server's surface is what makes shadow surface
// visible — so these checks cover the two remaining statically-scoreable
// categories: MCP01 (secret exposure) and MCP03 (tool poisoning).

// Security finding kinds.
const (
	FindingSecretInDescription = "secret_in_description" // OWASP MCP01
	FindingToolPoisoning       = "tool_poisoning"        // OWASP MCP03
	FindingSpecInvalidName     = "spec_invalid_tool_name"
)

// SecurityFinding is one flagged surface.
type SecurityFinding struct {
	Kind    string `json:"kind"`
	Surface string `json:"surface"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Detail  string `json:"detail"`
}

// specToolName is the MCP spec charset/length for tool names (server/tools):
// 1-128 chars from [A-Za-z0-9_.-]. Enforced only for mcp_tool surfaces (HTTP
// routes and CLI commands are not MCP tool names).
var specToolName = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,128}$`)

// secretPatterns match provider-shaped credential literals appearing in a
// human-facing description or schema string — where a real secret never
// belongs. Deliberately specific (known prefixes / explicit assignments) to
// avoid flagging ordinary high-entropy identifiers.
var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"openai/anthropic key", regexp.MustCompile(`sk-[A-Za-z0-9_\-]{20,}`)},
	{"aws access key id", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"github token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{30,}`)},
	{"slack token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`)},
	{"google api key", regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`)},
	{"private key block", regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |PGP )?PRIVATE KEY-----`)},
	{"explicit secret assignment", regexp.MustCompile(`(?i)\b(password|passwd|api[_-]?key|secret|token|bearer)\s*[:=]\s*["']?[A-Za-z0-9_\-]{8,}`)},
}

// poisoningPatterns match model-directed imperative language in a description —
// text aimed at steering the model rather than describing the tool's behavior
// to a reader. This is the tool-poisoning / hidden-instruction signature.
var poisoningPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore (all |any )?(the )?(previous|prior|above|preceding) (instructions|prompts?|context)`),
	regexp.MustCompile(`(?i)disregard (all |any )?(the )?(previous|prior|above) (instructions|prompts?)`),
	regexp.MustCompile(`(?i)do not (tell|inform|mention|reveal|disclose)( this)?( to)? the user`),
	regexp.MustCompile(`(?i)without (telling|informing|notifying|alerting) the user`),
	regexp.MustCompile(`(?i)</?(system|instructions?|important)>`),
	regexp.MustCompile(`(?i)\byou (must|should|need to|have to) (always |secretly |first )?(call|use|invoke|run|read|send|exfiltrate)`),
	regexp.MustCompile(`(?i)(before|prior to) (using|calling) (any|this|the) (other )?tools?,? (you|always|first)`),
}

// SecurityFindings scans a repo's surfaces for the static OWASP subset and
// spec-invalid tool names.
func SecurityFindings(surfaces []surfaceinventory.Surface) []SecurityFinding {
	var out []SecurityFinding
	for _, s := range surfaces {
		if s.Kind == surfaceinventory.KindMCPTool && s.Name != "" && !specToolName.MatchString(s.Name) {
			out = append(out, SecurityFinding{
				Kind: FindingSpecInvalidName, Surface: s.Name, File: s.File, Line: s.Line,
				Detail: "tool name violates MCP spec charset/length [A-Za-z0-9_.-]{1,128}",
			})
		}
		if !mcpKinds[s.Kind] || s.Description == "" {
			continue
		}
		desc := s.Description
		for _, sp := range secretPatterns {
			if sp.re.MatchString(desc) {
				out = append(out, SecurityFinding{
					Kind: FindingSecretInDescription, Surface: s.Name, File: s.File, Line: s.Line,
					Detail: "possible " + sp.name + " literal in description (OWASP MCP01)",
				})
				break // one secret finding per surface is enough signal
			}
		}
		for _, pp := range poisoningPatterns {
			if pp.MatchString(desc) {
				out = append(out, SecurityFinding{
					Kind: FindingToolPoisoning, Surface: s.Name, File: s.File, Line: s.Line,
					Detail: "model-directed imperative in description (OWASP MCP03 tool poisoning): " + firstMatch(pp, desc),
				})
				break
			}
		}
	}
	return out
}

func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindString(s)
	m = strings.TrimSpace(m)
	if len(m) > 80 {
		m = m[:80] + "…"
	}
	return m
}

// securityScore converts findings into a 0-100 dimension. Secrets are the
// heaviest (real exposure), poisoning next, spec-invalid names lightest. nil
// when there is nothing to measure (no MCP surfaces with descriptions/names).
func securityScore(surfaces []surfaceinventory.Surface, findings []SecurityFinding) *int {
	measurable := false
	for _, s := range surfaces {
		if mcpKinds[s.Kind] {
			measurable = true
			break
		}
	}
	if !measurable {
		return nil
	}
	penalty := 0
	for _, f := range findings {
		switch f.Kind {
		case FindingSecretInDescription:
			penalty += 40
		case FindingToolPoisoning:
			penalty += 25
		case FindingSpecInvalidName:
			penalty += 8
		}
	}
	v := clamp(100 - penalty)
	return &v
}
