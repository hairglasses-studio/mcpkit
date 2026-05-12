package fossil

import (
	"context"
)

// ScaffoldingOptions configures scaffolding detection.
type ScaffoldingOptions struct {
	IncludeTODOs bool
}

// DetectScaffolding runs fossil scaffolding detection in JSON mode and parses findings.
func (s *Scanner) DetectScaffolding(ctx context.Context, opts ScaffoldingOptions) ([]Finding, error) {
	var args []string
	if opts.IncludeTODOs {
		args = append(args, "--include-todos")
	}
	raw, err := s.run(ctx, "scaffolding", FormatJSON, args...)
	if err != nil {
		return nil, err
	}
	report, err := ParseScanReport(raw)
	if err != nil {
		return nil, err
	}
	return report.Findings, nil
}
