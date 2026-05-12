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
echo "env:${FOSSIL_NO_UPDATE_CHECK:-0}"
echo "args:$*"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake fossil script: %v", err)
	}
	return path
}
