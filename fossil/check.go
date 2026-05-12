package fossil

import (
	"context"
	"encoding/json"
	"strconv"
)

// CheckOptions configures fossil CI-style threshold checks.
type CheckOptions struct {
	Diff              string
	MaxDeadCode       int
	MaxClones         int
	MaxScaffolding    int
	MinConfidence     string
	FailOnScaffolding bool
}

// ThresholdViolation is one failed fossil check threshold.
type ThresholdViolation struct {
	Category  string `json:"category"`
	Threshold int    `json:"threshold"`
	Actual    int    `json:"actual"`
	Message   string `json:"message"`
}

// CheckReport is the typed JSON contract for `fossil-mcp check --format json`.
type CheckReport struct {
	DeadCodeCount    int                  `json:"dead_code_count"`
	CloneCount       int                  `json:"clone_count"`
	ScaffoldingCount int                  `json:"scaffolding_count"`
	Findings         []Finding            `json:"findings"`
	Violations       []ThresholdViolation `json:"violations"`
	Passed           bool                 `json:"passed"`
	DiffScope        any                  `json:"diff_scope"`
}

// ParseCheckReport decodes fossil check JSON output.
func ParseCheckReport(data []byte) (CheckReport, error) {
	var report CheckReport
	if err := json.Unmarshal(data, &report); err != nil {
		return CheckReport{}, err
	}
	return report, nil
}

// Check runs fossil check in JSON mode and parses the result.
//
// fossil-mcp exits 1 when thresholds are violated; that is expected behavior.
func (s *Scanner) Check(ctx context.Context, opts CheckOptions) (CheckReport, error) {
	var args []string
	if opts.Diff != "" {
		args = append(args, "--diff", opts.Diff)
	}
	if opts.MaxDeadCode >= 0 {
		args = append(args, "--max-dead-code", strconv.Itoa(opts.MaxDeadCode))
	}
	if opts.MaxClones >= 0 {
		args = append(args, "--max-clones", strconv.Itoa(opts.MaxClones))
	}
	if opts.MaxScaffolding >= 0 {
		args = append(args, "--max-scaffolding", strconv.Itoa(opts.MaxScaffolding))
	}
	if opts.MinConfidence != "" {
		args = append(args, "--min-confidence", opts.MinConfidence)
	}
	if opts.FailOnScaffolding {
		args = append(args, "--fail-on-scaffolding")
	}

	raw, err := s.runCheck(ctx, args...)
	if err != nil {
		return CheckReport{}, err
	}
	return ParseCheckReport(raw)
}
