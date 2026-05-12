package fossil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ScanFormat is the output format for fossil-mcp scan.
type ScanFormat string

const (
	FormatText  ScanFormat = "text"
	FormatJSON  ScanFormat = "json"
	FormatSARIF ScanFormat = "sarif"
)

// ScannerConfig configures fossil-mcp scan execution.
type ScannerConfig struct {
	// Dir is the path to analyze. Defaults to ".".
	Dir string
	// FossilBin is the fossil binary path. Defaults to "fossil-mcp".
	FossilBin string
	// Timeout is the maximum duration per scan. Defaults to 5 minutes.
	Timeout time.Duration
	// NoUpdateCheck sets FOSSIL_NO_UPDATE_CHECK=1 for command execution.
	// Defaults to true.
	NoUpdateCheck bool
}

// Scanner runs fossil-mcp scans and returns raw output.
type Scanner struct {
	cfg    ScannerConfig
	runCmd func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// NewScanner creates a Scanner with sensible defaults.
func NewScanner(cfg ...ScannerConfig) *Scanner {
	s := &Scanner{
		cfg: ScannerConfig{
			Dir:           ".",
			FossilBin:     "fossil-mcp",
			Timeout:       5 * time.Minute,
			NoUpdateCheck: true,
		},
		runCmd: exec.CommandContext,
	}
	if len(cfg) > 0 {
		s.cfg = cfg[0]
		if s.cfg.Dir == "" {
			s.cfg.Dir = "."
		}
		if s.cfg.FossilBin == "" {
			s.cfg.FossilBin = "fossil-mcp"
		}
		if s.cfg.Timeout == 0 {
			s.cfg.Timeout = 5 * time.Minute
		}
	}
	return s
}

// Scan executes "fossil-mcp scan <dir> --format <format>" and returns stdout.
func (s *Scanner) Scan(ctx context.Context, format ScanFormat) ([]byte, error) {
	if !validFormat(format) {
		return nil, fmt.Errorf("invalid fossil scan format %q", format)
	}

	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	args := []string{"scan", s.cfg.Dir, "--format", string(format)}
	cmd := s.runCmd(ctx, s.cfg.FossilBin, args...)
	if s.cfg.NoUpdateCheck {
		cmd.Env = append(os.Environ(), "FOSSIL_NO_UPDATE_CHECK=1")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("fossil scan failed: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("fossil scan failed: %w", err)
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("fossil scan produced empty output")
	}

	return stdout.Bytes(), nil
}

func validFormat(format ScanFormat) bool {
	switch format {
	case FormatText, FormatJSON, FormatSARIF:
		return true
	default:
		return false
	}
}
