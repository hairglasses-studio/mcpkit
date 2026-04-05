//go:build !official_sdk

package eval

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Scorer evaluates a tool's output against expected values.
type Scorer interface {
	Name() string
	Score(output string, isError bool, expected any) Score
}

// ExactMatch returns a scorer that checks if the output exactly matches the
// expected string.
func ExactMatch() Scorer {
	return &exactMatch{}
}

type exactMatch struct{}

func (s *exactMatch) Name() string { return "exact_match" }

func (s *exactMatch) Score(output string, isError bool, expected any) Score {
	want, _ := expected.(string)
	if output == want {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("got %q, want %q", output, want)}
}

// Contains returns a scorer that checks if the output contains the expected
// string.
func Contains() Scorer {
	return &containsScorer{}
}

type containsScorer struct{}

func (s *containsScorer) Name() string { return "contains" }

func (s *containsScorer) Score(output string, isError bool, expected any) Score {
	want, _ := expected.(string)
	if strings.Contains(output, want) {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("output does not contain %q", want)}
}

// Regex returns a scorer that checks if the output matches the expected
// regular expression pattern (expected should be a string).
func Regex() Scorer {
	return &regexScorer{}
}

type regexScorer struct{}

func (s *regexScorer) Name() string { return "regex" }

func (s *regexScorer) Score(output string, isError bool, expected any) Score {
	pattern, _ := expected.(string)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("invalid regex %q: %v", pattern, err)}
	}
	if re.MatchString(output) {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("output does not match pattern %q", pattern)}
}

// IsError returns a scorer that checks whether the tool result is an error.
// If want is true, the scorer passes when the result IS an error.
// If want is false, the scorer passes when the result is NOT an error.
func IsError(want bool) Scorer {
	return &isErrorScorer{want: want}
}

type isErrorScorer struct {
	want bool
}

func (s *isErrorScorer) Name() string { return "is_error" }

func (s *isErrorScorer) Score(output string, isError bool, expected any) Score {
	if isError == s.want {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("isError=%v, want %v", isError, s.want)}
}

// JSONPath returns a scorer that parses the output as JSON, navigates a
// dot-separated path, and compares the value at that path against expected.
// The path uses dot notation (e.g., "data.count" navigates {"data":{"count":5}}).
func JSONPath(path string) Scorer {
	return &jsonPathScorer{path: path}
}

type jsonPathScorer struct {
	path string
}

func (s *jsonPathScorer) Name() string { return "jsonpath" }

func (s *jsonPathScorer) Score(output string, isError bool, expected any) Score {
	var data any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("invalid JSON: %v", err)}
	}

	parts := strings.Split(s.path, ".")
	current := data
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("path %q: not an object at %q", s.path, part)}
		}
		current, ok = m[part]
		if !ok {
			return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("path %q: key %q not found", s.path, part)}
		}
	}

	// Compare: marshal both to JSON for type-agnostic comparison
	gotJSON, _ := json.Marshal(current)
	wantJSON, _ := json.Marshal(expected)
	if string(gotJSON) == string(wantJSON) {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("at %q: got %s, want %s", s.path, gotJSON, wantJSON)}
}

// NotEmpty returns a scorer that passes when the output is non-empty.
func NotEmpty() Scorer {
	return &notEmptyScorer{}
}

type notEmptyScorer struct{}

func (s *notEmptyScorer) Name() string { return "not_empty" }

func (s *notEmptyScorer) Score(output string, isError bool, expected any) Score {
	if output != "" {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: "output is empty"}
}

// Latency returns a ResultScorer that passes when the result duration is
// within the specified maximum.
func Latency(max time.Duration) ResultScorer {
	return &latencyScorer{max: max}
}

type latencyScorer struct {
	max time.Duration
}

func (s *latencyScorer) Name() string { return "latency" }

func (s *latencyScorer) ScoreResult(result Result) Score {
	if result.Duration <= s.max {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: fmt.Sprintf("duration %v exceeds max %v", result.Duration, s.max)}
}

// ErrorRate returns a ResultScorer that passes (1.0) when the tool result
// has no error, and fails (0.0) when the result indicates an error.
func ErrorRate() ResultScorer {
	return &errorRateScorer{}
}

type errorRateScorer struct{}

func (s *errorRateScorer) Name() string { return "error_rate" }

func (s *errorRateScorer) ScoreResult(result Result) Score {
	if !result.Error {
		return Score{Scorer: s.Name(), Value: 1.0}
	}
	return Score{Scorer: s.Name(), Value: 0.0, Reason: "tool returned error"}
}

// Custom returns a scorer with a user-provided scoring function.
func Custom(name string, fn func(output string, isError bool, expected any) Score) Scorer {
	return &customScorer{name: name, fn: fn}
}

type customScorer struct {
	name string
	fn   func(output string, isError bool, expected any) Score
}

func (s *customScorer) Name() string { return s.name }

func (s *customScorer) Score(output string, isError bool, expected any) Score {
	sc := s.fn(output, isError, expected)
	sc.Scorer = s.name
	return sc
}
