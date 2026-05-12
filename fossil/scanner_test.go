package fossil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewScannerDefaults(t *testing.T) {
	s := NewScanner()
	if s.cfg.Dir != "." {
		t.Fatalf("Dir=%q, want .", s.cfg.Dir)
	}
	if s.cfg.FossilBin != "fossil-mcp" {
		t.Fatalf("FossilBin=%q, want fossil-mcp", s.cfg.FossilBin)
	}
	if s.cfg.Timeout != 5*time.Minute {
		t.Fatalf("Timeout=%v, want 5m", s.cfg.Timeout)
	}
	if !s.cfg.NoUpdateCheck {
		t.Fatal("NoUpdateCheck should default to true")
	}
}

func TestScanSuccess(t *testing.T) {
	fake := writeFakeFossil(t)
	s := NewScanner(ScannerConfig{
		Dir:           ".",
		FossilBin:     fake,
		Timeout:       2 * time.Second,
		NoUpdateCheck: true,
	})

	out, err := s.Scan(context.Background(), FormatJSON)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "env:1") {
		t.Fatalf("expected FOSSIL_NO_UPDATE_CHECK=1 in output, got %q", text)
	}
	if !strings.Contains(text, "args:scan . --format json") {
		t.Fatalf("unexpected args output: %q", text)
	}
}

func TestScanError(t *testing.T) {
	fake := writeFakeFossil(t)
	s := NewScanner(ScannerConfig{
		Dir:           ".",
		FossilBin:     fake,
		Timeout:       2 * time.Second,
		NoUpdateCheck: true,
	})

	t.Setenv("FOSSIL_TEST_FAIL", "1")
	_, err := s.Scan(context.Background(), FormatJSON)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "simulated failure") {
		t.Fatalf("expected stderr message in error, got %v", err)
	}
}

func TestScanRejectsInvalidFormat(t *testing.T) {
	s := NewScanner()
	_, err := s.Scan(context.Background(), ScanFormat("xml"))
	if err == nil {
		t.Fatal("expected invalid format error, got nil")
	}
}

func TestScannerDefaultsPersistAcrossPartialConfig(t *testing.T) {
	s := NewScanner(ScannerConfig{Dir: "/tmp/example"})
	if s.cfg.Dir != "/tmp/example" {
		t.Fatalf("Dir=%q, want /tmp/example", s.cfg.Dir)
	}
	if !s.cfg.NoUpdateCheck {
		t.Fatal("NoUpdateCheck default should persist when partial config is provided")
	}
}

func TestScanReportParsesFindings(t *testing.T) {
	fake := writeFakeFossil(t)
	s := NewScanner(ScannerConfig{
		Dir:       ".",
		FossilBin: fake,
		Timeout:   2 * time.Second,
	})
	t.Setenv("FOSSIL_TEST_STDOUT", `[{"rule_id":"DEAD-dead function","title":"unusedOne","description":"never called","severity":"info","confidence":"low","location":{"file":"main.go","line_start":5,"line_end":5,"column_start":0,"column_end":0},"tags":[],"related_locations":[]}]`)

	report, err := s.ScanReport(context.Background())
	if err != nil {
		t.Fatalf("ScanReport() error = %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("len(Findings)=%d, want 1", len(report.Findings))
	}
	if report.Findings[0].Category() != CategoryDeadCode {
		t.Fatalf("Category=%q, want %q", report.Findings[0].Category(), CategoryDeadCode)
	}
}

func TestDetectDeadCodeParsesFindings(t *testing.T) {
	fake := writeFakeFossil(t)
	s := NewScanner(ScannerConfig{
		Dir:       ".",
		FossilBin: fake,
		Timeout:   2 * time.Second,
	})
	t.Setenv("FOSSIL_TEST_STDOUT", `[{"rule_id":"DEAD-dead function","title":"unusedOne","description":"never called","severity":"info","confidence":"certain","location":{"file":"main.go","line_start":5,"line_end":5,"column_start":0,"column_end":0},"tags":[],"related_locations":[]}]`)

	findings, err := s.DetectDeadCode(context.Background(), DeadCodeOptions{MinConfidence: "certain"})
	if err != nil {
		t.Fatalf("DetectDeadCode() error = %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("len(findings)=%d, want 1", len(findings))
	}
	if findings[0].Category() != CategoryDeadCode {
		t.Fatalf("Category=%q, want %q", findings[0].Category(), CategoryDeadCode)
	}
}

func TestDetectClonesParsesGroups(t *testing.T) {
	fake := writeFakeFossil(t)
	s := NewScanner(ScannerConfig{
		Dir:       ".",
		FossilBin: fake,
		Timeout:   2 * time.Second,
	})
	t.Setenv("FOSSIL_TEST_STDOUT", `[{"clone_type":"Type3","instances":[{"file":"main.go","start_line":5,"end_line":13,"start_byte":0,"end_byte":0,"function_name":"cloneA"},{"file":"main.go","start_line":14,"end_line":22,"start_byte":0,"end_byte":0,"function_name":"cloneB"}],"similarity":1.0,"hash":null}]`)

	groups, err := s.DetectClones(context.Background(), CloneOptions{MinLines: 3})
	if err != nil {
		t.Fatalf("DetectClones() error = %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups)=%d, want 1", len(groups))
	}
	if got := groups[0].Instances[0].FunctionName; got != "cloneA" {
		t.Fatalf("FunctionName=%q, want cloneA", got)
	}
}

func TestDetectScaffoldingParsesFindings(t *testing.T) {
	fake := writeFakeFossil(t)
	s := NewScanner(ScannerConfig{
		Dir:       ".",
		FossilBin: fake,
		Timeout:   2 * time.Second,
	})
	t.Setenv("FOSSIL_TEST_STDOUT", `[{"rule_id":"SCAFFOLD-placeholder","title":"// TODO: implement real logic","description":"Scaffolding: placeholder","severity":"info","confidence":"high","location":{"file":"main.go","line_start":4,"line_end":4,"column_start":0,"column_end":0},"tags":[],"related_locations":[]}]`)

	findings, err := s.DetectScaffolding(context.Background(), ScaffoldingOptions{IncludeTODOs: true})
	if err != nil {
		t.Fatalf("DetectScaffolding() error = %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("len(findings)=%d, want 1", len(findings))
	}
	if findings[0].Category() != CategoryScaffolding {
		t.Fatalf("Category=%q, want %q", findings[0].Category(), CategoryScaffolding)
	}
}

func TestCheckParsesFailureJSONFromStderr(t *testing.T) {
	fake := writeFakeFossil(t)
	s := NewScanner(ScannerConfig{
		Dir:       ".",
		FossilBin: fake,
		Timeout:   2 * time.Second,
	})
	t.Setenv("FOSSIL_TEST_STDERR", "banner\n{\"dead_code_count\":2,\"clone_count\":0,\"scaffolding_count\":0,\"findings\":[],\"violations\":[{\"category\":\"dead_code\",\"threshold\":0,\"actual\":2,\"message\":\"too many\"}],\"passed\":false,\"diff_scope\":null}\nError: analysis error")
	t.Setenv("FOSSIL_TEST_EXITCODE", "1")

	report, err := s.Check(context.Background(), CheckOptions{MaxDeadCode: 0})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if report.Passed {
		t.Fatal("expected failed report")
	}
	if len(report.Violations) != 1 || report.Violations[0].Category != "dead_code" {
		t.Fatalf("unexpected violations: %+v", report.Violations)
	}
}

func writeFakeFossil(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-fossil.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
if [[ "${FOSSIL_TEST_FAIL:-0}" == "1" ]]; then
  echo "simulated failure" >&2
  exit 7
fi
if [[ -n "${FOSSIL_TEST_STDERR:-}" ]]; then
  printf '%s' "${FOSSIL_TEST_STDERR}" >&2
  exit "${FOSSIL_TEST_EXITCODE:-1}"
fi
if [[ -n "${FOSSIL_TEST_STDOUT:-}" ]]; then
  printf '%s' "${FOSSIL_TEST_STDOUT}"
  exit 0
fi
echo "env:${FOSSIL_NO_UPDATE_CHECK:-0}"
echo "args:$*"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake fossil script: %v", err)
	}
	return path
}
