package fossil

import (
	"context"
	"encoding/json"
	"strings"
)

// FindingCategory is the normalized downstream contract for fossil findings.
type FindingCategory string

const (
	CategoryUnknown      FindingCategory = "unknown"
	CategoryDeadCode     FindingCategory = "dead_code"
	CategoryClone        FindingCategory = "clone"
	CategoryScaffolding  FindingCategory = "scaffolding"
	CategoryBlastRadius  FindingCategory = "blast_radius"
)

// Location points to a finding location in source.
type Location struct {
	File        string `json:"file"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
	ColumnStart int    `json:"column_start"`
	ColumnEnd   int    `json:"column_end"`
}

// RelatedLocation points to additional files/locations relevant to a finding.
type RelatedLocation struct {
	File      string `json:"file"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
}

// Finding is the canonical contract for fossil JSON findings.
type Finding struct {
	RuleID           string            `json:"rule_id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Severity         string            `json:"severity"`
	Confidence       string            `json:"confidence"`
	Location         Location          `json:"location"`
	CodeSnippet      *string           `json:"code_snippet"`
	FixSuggestion    *string           `json:"fix_suggestion"`
	CWE              *string           `json:"cwe"`
	Tags             []string          `json:"tags"`
	RelatedLocations []RelatedLocation `json:"related_locations"`
	FixText          *string           `json:"fix_text"`
}

// Category maps fossil rule IDs into stable downstream categories.
func (f Finding) Category() FindingCategory {
	ruleID := strings.ToUpper(f.RuleID)
	switch {
	case strings.HasPrefix(ruleID, "DEAD-"):
		return CategoryDeadCode
	case strings.HasPrefix(ruleID, "CLONE-"):
		return CategoryClone
	case strings.HasPrefix(ruleID, "SCAFFOLD-"), strings.HasPrefix(ruleID, "TEMP-"):
		return CategoryScaffolding
	case strings.Contains(ruleID, "BLAST"):
		return CategoryBlastRadius
	default:
		return CategoryUnknown
	}
}

// ScanReport is the typed JSON contract for `fossil-mcp scan --format json`.
type ScanReport struct {
	Findings []Finding `json:"findings"`
}

// FindingsByCategory returns only findings from the requested category.
func (r ScanReport) FindingsByCategory(category FindingCategory) []Finding {
	var out []Finding
	for _, finding := range r.Findings {
		if finding.Category() == category {
			out = append(out, finding)
		}
	}
	return out
}

// ParseScanReport decodes fossil JSON output into the canonical contract.
func ParseScanReport(data []byte) (ScanReport, error) {
	var findings []Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return ScanReport{}, err
	}
	return ScanReport{Findings: findings}, nil
}

// ScanReport runs fossil scan in JSON mode and parses the result.
func (s *Scanner) ScanReport(ctx context.Context) (ScanReport, error) {
	raw, err := s.Scan(ctx, FormatJSON)
	if err != nil {
		return ScanReport{}, err
	}
	return ParseScanReport(raw)
}
