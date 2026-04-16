package vuln

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ScannerConfig configures the govulncheck scanner.
type ScannerConfig struct {
	// Dir is the module root directory to scan. Defaults to ".".
	Dir string

	// Patterns is the list of package patterns to pass to govulncheck.
	// Defaults to []string{"./..."}.
	Patterns []string

	// GovulncheckBin is the path to the govulncheck binary.
	// Defaults to "govulncheck" (resolved via PATH).
	GovulncheckBin string

	// Timeout is the maximum time allowed for the scan.
	// Defaults to 5 minutes.
	Timeout time.Duration
}

// Scanner wraps govulncheck to produce structured vulnerability results.
type Scanner struct {
	cfg ScannerConfig
}

// NewScanner creates a Scanner with the given configuration.
// All configuration fields have sensible defaults.
func NewScanner(cfg ...ScannerConfig) *Scanner {
	s := &Scanner{}
	if len(cfg) > 0 {
		s.cfg = cfg[0]
	}
	if s.cfg.Dir == "" {
		s.cfg.Dir = "."
	}
	if len(s.cfg.Patterns) == 0 {
		s.cfg.Patterns = []string{"./..."}
	}
	if s.cfg.GovulncheckBin == "" {
		s.cfg.GovulncheckBin = "govulncheck"
	}
	if s.cfg.Timeout == 0 {
		s.cfg.Timeout = 5 * time.Minute
	}
	return s
}

// Scan runs govulncheck on the configured directory and returns structured results.
// It requires govulncheck to be installed; see golang.org/x/vuln/cmd/govulncheck.
func (s *Scanner) Scan(ctx context.Context) (ScanResult, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	args := append([]string{"-format", "json"}, s.cfg.Patterns...)
	cmd := exec.CommandContext(ctx, s.cfg.GovulncheckBin, args...)
	cmd.Dir = s.cfg.Dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// govulncheck exits non-zero when vulnerabilities are found — that is
	// expected. We treat only execution errors (binary not found, timeout,
	// etc.) as hard errors.
	runErr := cmd.Run()

	// If stdout is empty we have a real execution problem.
	if stdout.Len() == 0 {
		if runErr != nil {
			return ScanResult{}, fmt.Errorf("govulncheck failed: %w: %s", runErr, stderr.String())
		}
		return ScanResult{Dir: s.cfg.Dir}, nil
	}

	return s.parseOutput(ctx, stdout.Bytes(), runErr)
}

// govulncheckMessage matches the streaming JSON output envelope.
type govulncheckMessage struct {
	Config  *govulncheckConfig  `json:"config,omitempty"`
	Finding *govulncheckFinding `json:"finding,omitempty"`
	OSV     *osvEntry           `json:"osv,omitempty"`
}

type govulncheckConfig struct {
	ScannerVersion string     `json:"scanner_version,omitempty"`
	GoVersion      string     `json:"go_version,omitempty"`
	DBLastModified *time.Time `json:"db_last_modified,omitempty"`
}

type govulncheckFinding struct {
	OSV          string                   `json:"osv,omitempty"`
	FixedVersion string                   `json:"fixed_version,omitempty"`
	Trace        []govulncheckFrame       `json:"trace,omitempty"`
}

type govulncheckFrame struct {
	Module  string `json:"module,omitempty"`
	Version string `json:"version,omitempty"`
	Package string `json:"package,omitempty"`
	// Function is non-empty for symbol-level findings.
	Function string `json:"function,omitempty"`
}

func (s *Scanner) parseOutput(_ context.Context, data []byte, _ error) (ScanResult, error) {
	result := ScanResult{
		Dir: s.cfg.Dir,
	}

	// govulncheck emits one JSON object per line.
	osvByID := make(map[string]*osvEntry)
	// Track best finding per OSV ID: prefer symbol-level (called) over module-level.
	type findingKey struct{ osvID string }
	type findingRecord struct {
		fixedVersion string
		module       string
		version      string
		called       bool
	}
	findings := make(map[findingKey]findingRecord)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	// govulncheck output can have very long lines (full symbol lists).
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg govulncheckMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip unparseable lines (progress messages, etc.)
			continue
		}

		if msg.Config != nil {
			result.ScannerVersion = msg.Config.ScannerVersion
			result.GoVersion = msg.Config.GoVersion
			result.DBLastModified = msg.Config.DBLastModified
		}

		if msg.OSV != nil {
			osvByID[msg.OSV.ID] = msg.OSV
		}

		if msg.Finding != nil {
			if msg.Finding.OSV == "" {
				continue
			}
			key := findingKey{osvID: msg.Finding.OSV}
			rec, exists := findings[key]

			// Determine module and version from the first frame.
			module, version := "", ""
			called := false
			for _, f := range msg.Finding.Trace {
				if f.Module != "" {
					module = f.Module
					version = f.Version
				}
				if f.Function != "" {
					called = true
				}
			}
			if msg.Finding.FixedVersion != "" {
				rec.fixedVersion = msg.Finding.FixedVersion
			}
			if module != "" {
				rec.module = module
				rec.version = version
			}
			// Once we see a symbol-level (called) finding, keep it.
			if !exists || called {
				rec.called = called
			}
			findings[key] = rec
		}
	}
	if err := scanner.Err(); err != nil {
		return ScanResult{}, fmt.Errorf("parsing govulncheck output: %w", err)
	}

	for osvID, rec := range findings {
		entry := osvByID[osvID.osvID]

		vuln := Vulnerability{
			ID:           osvID.osvID,
			Module:       rec.module,
			Version:      rec.version,
			FixedVersion: rec.fixedVersion,
			Called:       rec.called,
			Severity:     SeverityUnknown,
		}
		if entry != nil {
			vuln.Summary = entry.Summary
			vuln.Details = entry.Details
			vuln.Aliases = entry.Aliases
			vuln.Published = entry.Published
			vuln.Modified = entry.Modified
			for _, ref := range entry.References {
				vuln.References = append(vuln.References, Reference{Type: ref.Type, URL: ref.URL})
			}
			if entry.DatabaseSpecific != nil {
				vuln.AdvisoryURL = entry.DatabaseSpecific.URL
			}
			vuln.Severity = classifySeverity(entry)
		}
		result.Vulnerabilities = append(result.Vulnerabilities, vuln)
	}

	result.Summary = buildScanSummary(result)
	return result, nil
}

// classifySeverity applies a heuristic severity classification based on
// the presence of CVE aliases and OSV review status. The Go vuln DB does
// not include CVSS scores, so this is a best-effort estimate.
func classifySeverity(e *osvEntry) Severity {
	if e == nil {
		return SeverityUnknown
	}
	// If there is no CVE alias the impact is typically limited.
	hasCVE := false
	for _, a := range e.Aliases {
		if strings.HasPrefix(a, "CVE-") {
			hasCVE = true
			break
		}
	}
	if !hasCVE {
		return SeverityLow
	}
	// Heuristic: RCE/auth-bypass keywords in summary → High; else Medium.
	s := strings.ToLower(e.Summary + " " + e.Details)
	for _, kw := range []string{"remote code execution", "arbitrary code", "privilege escalation", "authentication bypass", "panic", "crash", "denial of service"} {
		if strings.Contains(s, kw) {
			return SeverityHigh
		}
	}
	return SeverityMedium
}

func buildScanSummary(r ScanResult) string {
	n := len(r.Vulnerabilities)
	if n == 0 {
		return "No vulnerabilities found."
	}
	called := 0
	for _, v := range r.Vulnerabilities {
		if v.Called {
			called++
		}
	}
	if called == 0 {
		return fmt.Sprintf("%d vulnerabilit%s found (imported but not called).", n, pluralSuffix(n))
	}
	return fmt.Sprintf("%d vulnerabilit%s found (%d called).", n, pluralSuffix(n), called)
}

func pluralSuffix(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
