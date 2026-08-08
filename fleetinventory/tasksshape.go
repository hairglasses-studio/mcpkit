package fleetinventory

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Tasks-shape correctness detection. The MCP 2026-07-28 rewrite moved Tasks
// OUT of core into an experimental extension (SEP-2663) that no shipping Go SDK
// implements. The 2025-11-25 core Tasks shape — `tasks/get|list|cancel|result`
// RPCs, mcp-go's `Task`/`TaskStatus`/`TaskSupport` types, and mcpkit's
// `registry/tasks` manager built on them — is therefore removed-from-core on a
// Modern server. Per the SDK research this is a CORRECTNESS WARNING, not a
// maturity signal: a scanner must not score "has Tasks" as positive. Severity
// is era-dependent — non-conformant on a modern-capable/dual server, a
// migration hazard on a legacy-only one (where core Tasks is still valid).
//
// Static detection reads .go source (bounded per file to the import/early
// region where these signals appear); the authoritative check is a live
// `tasks/list` probe, deferred to the live variant.

// tasksScanBytes bounds how much of each .go file is read — imports and method
// registrations live near the top, so a header read catches the signal without
// paying to read multi-thousand-line files in full.
const tasksScanBytes = 8192

// Detection is deliberately HIGH-CONFIDENCE only. "Task"/"TaskStatus"/
// "tasks/list" are ubiquitous generic identifiers (Google Tasks API, private
// task types, job queues) — matching them floods the fleet with false
// positives (verified: runmylife's tasks/list is gtasks, mesmer/secretstudios
// TaskStatus are their own types). The authoritative check is a live
// `tasks/list` probe; statically we only flag the two signals that
// unambiguously mean the MCP Tasks shape:
//   - an import of mcpkit's registry/tasks manager, or
//   - a package-qualified mcp-go Tasks type (mcp.Task{ / mcp.TaskStatus /
//     mcp.TaskSupport), the mcp-go package's own protocol types.
//
// Under-detecting is acceptable; a warning that cries wolf on every "Task" in
// the fleet is worse than one that fires only when it's really the MCP shape.
var (
	tasksImportRe = regexp.MustCompile(`hairglasses-studio/mcpkit/registry/tasks"`)
	tasksTypeRe   = regexp.MustCompile(`\bmcp\.(Task\b|Task\{|TaskStatus|TaskSupport|NewTask)`)
)

// detectTasksShape reports whether a repo statically uses the removed-from-core
// 2025-11-25 Tasks shape, with up to a few evidence lines (relpath: signal).
func detectTasksShape(dir string, files []string) (bool, []string) {
	var evidence []string
	buf := make([]byte, tasksScanBytes)
	for _, rel := range files {
		if len(evidence) >= 5 {
			break
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		// Skip this detector's own source — it embeds the import path and type
		// names as regex string literals, which would self-match.
		if strings.HasSuffix(rel, "fleetinventory/tasksshape.go") || rel == "tasksshape.go" {
			continue
		}
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		n, _ := f.Read(buf)
		f.Close()
		s := string(buf[:n])
		var hit string
		switch {
		case tasksImportRe.MatchString(s):
			hit = "imports mcpkit/registry/tasks"
		case tasksTypeRe.MatchString(s):
			hit = "uses " + strings.TrimSuffix(tasksTypeRe.FindString(s), "{") + " (mcp-go Tasks type)"
		}
		if hit != "" {
			evidence = append(evidence, rel+": "+hit)
		}
	}
	return len(evidence) > 0, evidence
}

// tasksWarning returns a severity-tagged warning string for a repo's runtime
// profile, or "" when there is no Tasks-shape usage.
func tasksWarning(rt MCPRuntime) string {
	if !rt.TasksLegacyShape {
		return ""
	}
	switch rt.SpecEra {
	case EraModernCapable, EraDual:
		return "tasks-shape NON-CONFORMANT: uses the 2025-11-25 core Tasks shape, removed from core in 2026-07-28 (no shipping Go SDK implements the replacement extension)"
	default:
		return "tasks-shape migration hazard: uses the 2025-11-25 core Tasks shape (valid today) that is removed from core in 2026-07-28"
	}
}
