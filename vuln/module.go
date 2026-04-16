package vuln

import (
	"context"
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ModuleConfig configures the vuln MCP module.
type ModuleConfig struct {
	// ScannerConfig is passed to NewScanner for vuln_scan calls.
	ScannerConfig ScannerConfig

	// OSVClientConfig is passed to NewOSVClient for vuln_osv_query calls.
	OSVClientConfig OSVClientConfig
}

// Module implements registry.ToolModule for vulnerability scanning tools.
type Module struct {
	scanner   *Scanner
	osvClient *OSVClient
}

// NewModule creates a vuln module with the given configuration.
func NewModule(cfg ...ModuleConfig) *Module {
	m := &Module{}
	if len(cfg) > 0 {
		m.scanner = NewScanner(cfg[0].ScannerConfig)
		m.osvClient = NewOSVClient(cfg[0].OSVClientConfig)
	} else {
		m.scanner = NewScanner()
		m.osvClient = NewOSVClient()
	}
	return m
}

// Name returns the module name.
func (m *Module) Name() string { return "vuln" }

// Description returns the module description.
func (m *Module) Description() string {
	return "Go module security scanning tools: govulncheck integration and OSV API queries"
}

// Tools returns all vulnerability scanning tool definitions.
func (m *Module) Tools() []registry.ToolDefinition {
	tools := []registry.ToolDefinition{
		m.scanTool(),
		m.osvQueryTool(),
	}
	for i := range tools {
		tools[i].Category = "security"
		tools[i].Timeout = 10 * time.Minute
		tools[i].IsWrite = false
		tools[i].Complexity = registry.ComplexityModerate
	}
	return tools
}

// --- Tool input/output types ---

// ScanInput is the input for the vuln_scan tool.
type ScanInput struct {
	// Dir is the Go module root directory to scan. Defaults to ".".
	Dir string `json:"dir,omitempty" jsonschema:"description=Go module root directory to scan (default: current directory)"`

	// Patterns are the package patterns passed to govulncheck. Defaults to [\"./...\"].
	Patterns []string `json:"patterns,omitempty" jsonschema:"description=Package patterns to scan (default: [./...])"`
}

// ScanOutput is the output of the vuln_scan tool.
type ScanOutput = ScanResult

// OSVQueryInput is the input for the vuln_osv_query tool.
type OSVQueryInput struct {
	// Module is the Go module path to query (e.g. "golang.org/x/net").
	Module string `json:"module" jsonschema:"required,description=Go module path to query (e.g. golang.org/x/net)"`

	// Version is the module version to query (e.g. "v0.50.0").
	// Leave empty to query all known vulnerable versions.
	Version string `json:"version,omitempty" jsonschema:"description=Module version to query (e.g. v0.50.0); omit to query all versions"`
}

// OSVQueryOutput is the output of the vuln_osv_query tool.
type OSVQueryOutput = OSVQueryResult

// --- Tool definitions ---

func (m *Module) scanTool() registry.ToolDefinition {
	desc := "Run govulncheck on a Go module directory and return structured vulnerability results. " +
		"Requires govulncheck to be installed (go install golang.org/x/vuln/cmd/govulncheck@latest). " +
		"Returns each vulnerability with its OSV ID, severity estimate, affected module, current version, " +
		"fixed version, and whether the vulnerable symbol is reachable from your code." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Scan the current directory",
				Input:       map[string]any{"dir": "."},
				Output:      "ScanResult with Vulnerabilities list and Summary",
			},
			{
				Description: "Scan a specific module path with custom patterns",
				Input: map[string]any{
					"dir":      "/path/to/mymodule",
					"patterns": []any{"./cmd/...", "./internal/..."},
				},
				Output: "ScanResult filtered to specified package patterns",
			},
		})
	return handler.TypedHandler[ScanInput, ScanOutput](
		"vuln_scan",
		desc,
		func(ctx context.Context, input ScanInput) (ScanOutput, error) {
			dir := input.Dir
			if dir == "" {
				dir = "."
			}
			cfg := m.scanner.cfg
			cfg.Dir = dir
			if len(input.Patterns) > 0 {
				cfg.Patterns = input.Patterns
			}
			s := NewScanner(cfg)
			result, err := s.Scan(ctx)
			if err != nil {
				return ScanResult{}, fmt.Errorf("scan failed: %w", err)
			}
			return result, nil
		},
	)
}

func (m *Module) osvQueryTool() registry.ToolDefinition {
	desc := "Query the OSV (Open Source Vulnerabilities) API for known vulnerabilities " +
		"affecting a specific Go module and version. Returns structured vulnerability data " +
		"including CVE aliases, severity estimate, affected version ranges, and fix version." +
		handler.FormatExamples([]handler.ToolExample{
			{
				Description: "Query vulnerabilities for golang.org/x/net v0.50.0",
				Input: map[string]any{
					"module":  "golang.org/x/net",
					"version": "v0.50.0",
				},
				Output: "OSVQueryResult with vulnerability list and summary",
			},
			{
				Description: "Query all known vulnerabilities for a module",
				Input: map[string]any{
					"module": "github.com/gin-gonic/gin",
				},
				Output: "OSVQueryResult listing all historically vulnerable versions",
			},
		})
	return handler.TypedHandler[OSVQueryInput, OSVQueryOutput](
		"vuln_osv_query",
		desc,
		func(ctx context.Context, input OSVQueryInput) (OSVQueryOutput, error) {
			if input.Module == "" {
				return OSVQueryResult{}, fmt.Errorf("module path is required")
			}
			// Strip leading "v" from version before sending to OSV API which uses semver without prefix.
			version := input.Version
			if len(version) > 0 && version[0] == 'v' {
				version = version[1:]
			}
			return m.osvClient.Query(ctx, input.Module, version)
		},
	)
}
