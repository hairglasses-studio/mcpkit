package vuln

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// buildTestJSON assembles a govulncheck JSON output from individual message objects.
func buildTestJSON(msgs ...any) []byte {
	var sb strings.Builder
	for _, m := range msgs {
		b, _ := json.Marshal(m)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return []byte(sb.String())
}

func TestScannerParseOutput_NoVulns(t *testing.T) {
	s := NewScanner()

	dbTime := time.Date(2026, 4, 15, 15, 36, 31, 0, time.UTC)
	data := buildTestJSON(
		govulncheckMessage{
			Config: &govulncheckConfig{
				ScannerVersion: "v1.1.4",
				GoVersion:      "go1.26.1",
				DBLastModified: &dbTime,
			},
		},
	)

	result, err := s.parseOutput(context.Background(), data, nil)
	if err != nil {
		t.Fatalf("parseOutput: %v", err)
	}
	if len(result.Vulnerabilities) != 0 {
		t.Errorf("expected 0 vulns, got %d", len(result.Vulnerabilities))
	}
	if result.ScannerVersion != "v1.1.4" {
		t.Errorf("ScannerVersion = %q, want v1.1.4", result.ScannerVersion)
	}
	if result.GoVersion != "go1.26.1" {
		t.Errorf("GoVersion = %q, want go1.26.1", result.GoVersion)
	}
	if result.DBLastModified == nil || !result.DBLastModified.Equal(dbTime) {
		t.Errorf("DBLastModified = %v, want %v", result.DBLastModified, dbTime)
	}
	if result.Summary != "No vulnerabilities found." {
		t.Errorf("Summary = %q", result.Summary)
	}
}

func TestScannerParseOutput_WithVuln(t *testing.T) {
	s := NewScanner()

	pub := time.Date(2026, 2, 26, 18, 24, 17, 0, time.UTC)
	mod := time.Date(2026, 3, 3, 1, 29, 33, 0, time.UTC)
	data := buildTestJSON(
		govulncheckMessage{
			Config: &govulncheckConfig{
				ScannerVersion: "v1.1.4",
				GoVersion:      "go1.26.1",
			},
		},
		govulncheckMessage{
			OSV: &osvEntry{
				ID:        "GO-2026-4559",
				Summary:   "Sending certain HTTP/2 frames can cause a server to panic in golang.org/x/net",
				Details:   "Due to missing nil check, sending 0x0a-0x0f HTTP/2 frames will cause a running server to panic",
				Aliases:   []string{"CVE-2026-27141"},
				Published: pub,
				Modified:  mod,
				DatabaseSpecific: &struct {
					URL string `json:"url"`
				}{URL: "https://pkg.go.dev/vuln/GO-2026-4559"},
			},
		},
		govulncheckMessage{
			Finding: &govulncheckFinding{
				OSV:          "GO-2026-4559",
				FixedVersion: "v0.51.0",
				Trace: []govulncheckFrame{
					{Module: "golang.org/x/net", Version: "v0.50.0", Package: "golang.org/x/net/http2", Function: "ClientConn.RoundTrip"},
				},
			},
		},
	)

	result, err := s.parseOutput(context.Background(), data, nil)
	if err != nil {
		t.Fatalf("parseOutput: %v", err)
	}
	if len(result.Vulnerabilities) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(result.Vulnerabilities))
	}
	v := result.Vulnerabilities[0]
	if v.ID != "GO-2026-4559" {
		t.Errorf("ID = %q, want GO-2026-4559", v.ID)
	}
	if v.Module != "golang.org/x/net" {
		t.Errorf("Module = %q, want golang.org/x/net", v.Module)
	}
	if v.Version != "v0.50.0" {
		t.Errorf("Version = %q, want v0.50.0", v.Version)
	}
	if v.FixedVersion != "v0.51.0" {
		t.Errorf("FixedVersion = %q, want v0.51.0", v.FixedVersion)
	}
	if !v.Called {
		t.Error("Called should be true for symbol-level finding")
	}
	if v.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want HIGH (contains 'panic')", v.Severity)
	}
	if len(v.Aliases) == 0 || v.Aliases[0] != "CVE-2026-27141" {
		t.Errorf("Aliases = %v, want [CVE-2026-27141]", v.Aliases)
	}
	if v.AdvisoryURL != "https://pkg.go.dev/vuln/GO-2026-4559" {
		t.Errorf("AdvisoryURL = %q", v.AdvisoryURL)
	}
	if result.Summary == "" {
		t.Error("Summary should be non-empty")
	}
}

func TestScannerParseOutput_ModuleLevelFinding(t *testing.T) {
	// Module-level findings have no Function in the trace — Called should be false.
	s := NewScanner()
	data := buildTestJSON(
		govulncheckMessage{
			OSV: &osvEntry{
				ID:      "GO-2025-1111",
				Summary: "Test vulnerability",
			},
		},
		govulncheckMessage{
			Finding: &govulncheckFinding{
				OSV:          "GO-2025-1111",
				FixedVersion: "v1.2.0",
				Trace: []govulncheckFrame{
					// No Function → module-level, not called.
					{Module: "example.com/pkg", Version: "v1.0.0"},
				},
			},
		},
	)

	result, err := s.parseOutput(context.Background(), data, nil)
	if err != nil {
		t.Fatalf("parseOutput: %v", err)
	}
	if len(result.Vulnerabilities) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(result.Vulnerabilities))
	}
	v := result.Vulnerabilities[0]
	if v.Called {
		t.Error("Called should be false for module-level finding")
	}
	if !strings.Contains(result.Summary, "imported but not called") {
		t.Errorf("Summary should mention 'imported but not called', got %q", result.Summary)
	}
}

func TestScannerParseOutput_MultipleVulns(t *testing.T) {
	s := NewScanner()
	data := buildTestJSON(
		govulncheckMessage{
			OSV: &osvEntry{ID: "GO-2025-0001", Summary: "First vuln"},
		},
		govulncheckMessage{
			OSV: &osvEntry{ID: "GO-2025-0002", Summary: "Second vuln"},
		},
		govulncheckMessage{
			Finding: &govulncheckFinding{
				OSV:   "GO-2025-0001",
				Trace: []govulncheckFrame{{Module: "pkg/a", Version: "v1.0.0", Function: "Foo"}},
			},
		},
		govulncheckMessage{
			Finding: &govulncheckFinding{
				OSV:   "GO-2025-0002",
				Trace: []govulncheckFrame{{Module: "pkg/b", Version: "v2.0.0"}},
			},
		},
	)

	result, err := s.parseOutput(context.Background(), data, nil)
	if err != nil {
		t.Fatalf("parseOutput: %v", err)
	}
	if len(result.Vulnerabilities) != 2 {
		t.Fatalf("expected 2 vulns, got %d", len(result.Vulnerabilities))
	}
}

func TestScannerParseOutput_SkipsNonJSON(t *testing.T) {
	s := NewScanner()
	// Mix valid JSON with junk lines — should parse gracefully.
	data := []byte(`{"config":{"scanner_version":"v1.1.4"}}
not-json-at-all
{"finding":{"osv":"GO-2025-9999","fixed_version":"v1.0.0","trace":[{"module":"x/y","version":"v0.5.0"}]}}
{"osv":{"id":"GO-2025-9999","summary":"test"}}
`)
	result, err := s.parseOutput(context.Background(), data, nil)
	if err != nil {
		t.Fatalf("parseOutput: %v", err)
	}
	if len(result.Vulnerabilities) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(result.Vulnerabilities))
	}
}

func TestClassifySeverity_NilEntry(t *testing.T) {
	if classifySeverity(nil) != SeverityUnknown {
		t.Error("expected UNKNOWN for nil entry")
	}
}

func TestClassifySeverity_NoCVE(t *testing.T) {
	e := &osvEntry{ID: "GO-2025-0001", Aliases: []string{"GHSA-xxxx-xxxx"}}
	if classifySeverity(e) != SeverityLow {
		t.Errorf("expected LOW for GHSA-only alias, got %s", classifySeverity(e))
	}
}

func TestClassifySeverity_CVEWithPanic(t *testing.T) {
	e := &osvEntry{
		Aliases: []string{"CVE-2025-1234"},
		Summary: "A panic in the HTTP handler",
	}
	if classifySeverity(e) != SeverityHigh {
		t.Errorf("expected HIGH for panic keyword, got %s", classifySeverity(e))
	}
}

func TestClassifySeverity_CVENoKeyword(t *testing.T) {
	e := &osvEntry{
		Aliases: []string{"CVE-2025-1234"},
		Summary: "Minor information disclosure",
	}
	if classifySeverity(e) != SeverityMedium {
		t.Errorf("expected MEDIUM, got %s", classifySeverity(e))
	}
}

func TestNewScanner_Defaults(t *testing.T) {
	s := NewScanner()
	if s.cfg.Dir != "." {
		t.Errorf("Dir = %q, want .", s.cfg.Dir)
	}
	if len(s.cfg.Patterns) != 1 || s.cfg.Patterns[0] != "./..." {
		t.Errorf("Patterns = %v, want [./...]", s.cfg.Patterns)
	}
	if s.cfg.GovulncheckBin != "govulncheck" {
		t.Errorf("GovulncheckBin = %q, want govulncheck", s.cfg.GovulncheckBin)
	}
	if s.cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", s.cfg.Timeout)
	}
}

func TestScanSummary_Plurals(t *testing.T) {
	tests := []struct {
		name    string
		result  ScanResult
		wantSub string
	}{
		{
			name:    "zero",
			result:  ScanResult{},
			wantSub: "No vulnerabilities found.",
		},
		{
			name: "one called",
			result: ScanResult{
				Vulnerabilities: []Vulnerability{{ID: "A", Called: true}},
			},
			wantSub: "1 vulnerability found (1 called).",
		},
		{
			name: "two uncalled",
			result: ScanResult{
				Vulnerabilities: []Vulnerability{{ID: "A"}, {ID: "B"}},
			},
			wantSub: "imported but not called",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildScanSummary(tt.result)
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("buildScanSummary() = %q, want substring %q", got, tt.wantSub)
			}
		})
	}
}
