package fossil

import (
	"context"
	"strconv"
)

// DeadCodeOptions configures dead-code detection.
type DeadCodeOptions struct {
	IncludeTests  bool
	MinConfidence string
	MinLines      int
	Language      string
}

// DetectDeadCode runs fossil dead-code detection in JSON mode and parses findings.
func (s *Scanner) DetectDeadCode(ctx context.Context, opts DeadCodeOptions) ([]Finding, error) {
	var args []string
	if opts.IncludeTests {
		args = append(args, "--include-tests")
	}
	if opts.MinConfidence != "" {
		args = append(args, "--min-confidence", opts.MinConfidence)
	}
	if opts.MinLines > 0 {
		args = append(args, "--min-lines", strconv.Itoa(opts.MinLines))
	}
	if opts.Language != "" {
		args = append(args, "--language", opts.Language)
	}
	raw, err := s.run(ctx, "dead-code", FormatJSON, args...)
	if err != nil {
		return nil, err
	}
	report, err := ParseScanReport(raw)
	if err != nil {
		return nil, err
	}
	return report.Findings, nil
}
