package fossil

import (
	"context"
	"encoding/json"
	"strconv"
)

// CloneInstance is one location in a clone group.
type CloneInstance struct {
	File         string `json:"file"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	StartByte    int    `json:"start_byte"`
	EndByte      int    `json:"end_byte"`
	FunctionName string `json:"function_name"`
}

// CloneGroup is the canonical contract for fossil clone detection output.
type CloneGroup struct {
	CloneType  string          `json:"clone_type"`
	Instances  []CloneInstance `json:"instances"`
	Similarity float64         `json:"similarity"`
	Hash       *string         `json:"hash"`
}

// CloneOptions configures clone detection.
type CloneOptions struct {
	MinLines int
}

// ParseCloneGroups decodes `fossil-mcp clones --format json` output.
func ParseCloneGroups(data []byte) ([]CloneGroup, error) {
	var groups []CloneGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// DetectClones runs fossil clone detection in JSON mode and parses the result.
func (s *Scanner) DetectClones(ctx context.Context, opts CloneOptions) ([]CloneGroup, error) {
	var args []string
	if opts.MinLines > 0 {
		args = append(args, "--min-lines", strconv.Itoa(opts.MinLines))
	}
	raw, err := s.run(ctx, "clones", FormatJSON, args...)
	if err != nil {
		return nil, err
	}
	return ParseCloneGroups(raw)
}
