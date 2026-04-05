//go:build !official_sdk

package eval

import (
	"strings"
	"testing"
	"time"
)

func TestExactMatch_Match(t *testing.T) {
	t.Parallel()
	s := ExactMatch().Score("hello world", false, "hello world")
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestExactMatch_NoMatch(t *testing.T) {
	t.Parallel()
	s := ExactMatch().Score("hello", false, "world")
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
}

func TestContains_Match(t *testing.T) {
	t.Parallel()
	s := Contains().Score("hello world foo", false, "world")
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestContains_NoMatch(t *testing.T) {
	t.Parallel()
	s := Contains().Score("hello", false, "world")
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
}

func TestRegex_Match(t *testing.T) {
	t.Parallel()
	s := Regex().Score("abc123def", false, `\d{3}`)
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestRegex_NoMatch(t *testing.T) {
	t.Parallel()
	s := Regex().Score("abcdef", false, `\d{3}`)
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
}

func TestRegex_InvalidPattern(t *testing.T) {
	t.Parallel()
	s := Regex().Score("anything", false, `[invalid`)
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
	if s.Reason == "" {
		t.Error("expected reason for invalid regex")
	}
}

func TestIsError_True(t *testing.T) {
	t.Parallel()
	s := IsError(true).Score("err", true, nil)
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestIsError_False(t *testing.T) {
	t.Parallel()
	s := IsError(false).Score("ok", false, nil)
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestIsError_Mismatch(t *testing.T) {
	t.Parallel()
	s := IsError(true).Score("ok", false, nil)
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
}

func TestJSONPath_Match(t *testing.T) {
	t.Parallel()
	s := JSONPath("count").Score(`{"count": 5}`, false, float64(5))
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestJSONPath_Nested(t *testing.T) {
	t.Parallel()
	s := JSONPath("data.value").Score(`{"data": {"value": "ok"}}`, false, "ok")
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestJSONPath_Missing(t *testing.T) {
	t.Parallel()
	s := JSONPath("missing").Score(`{"other": 1}`, false, 1)
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
}

func TestCustomScorer(t *testing.T) {
	t.Parallel()
	scorer := Custom("length_check", func(output string, isError bool, expected any) Score {
		maxLen, _ := expected.(float64)
		if float64(len(output)) <= maxLen {
			return Score{Value: 1.0}
		}
		return Score{Value: 0.0, Reason: "too long"}
	})

	s := scorer.Score("hi", false, float64(10))
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
	if s.Scorer != "length_check" {
		t.Errorf("scorer = %q, want length_check", s.Scorer)
	}
}

func TestSummary_PassRate(t *testing.T) {
	t.Parallel()
	s := Summary{Total: 4, Passed: 3}
	if r := s.PassRate(); r != 0.75 {
		t.Errorf("PassRate = %f, want 0.75", r)
	}
}

func TestSummary_PassRate_Empty(t *testing.T) {
	t.Parallel()
	s := Summary{}
	if r := s.PassRate(); r != 0 {
		t.Errorf("PassRate = %f, want 0", r)
	}
}

func TestResult_AverageScore(t *testing.T) {
	t.Parallel()
	r := Result{
		Scores: []Score{
			{Value: 1.0},
			{Value: 0.5},
			{Value: 0.0},
		},
	}
	if avg := r.AverageScore(); avg != 0.5 {
		t.Errorf("AverageScore = %f, want 0.5", avg)
	}
}

func TestResult_AverageScore_Empty(t *testing.T) {
	t.Parallel()
	r := Result{}
	if avg := r.AverageScore(); avg != 0 {
		t.Errorf("AverageScore = %f, want 0", avg)
	}
}

func TestNotEmpty_NonEmpty(t *testing.T) {
	t.Parallel()
	s := NotEmpty().Score("hello", false, nil)
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestNotEmpty_Empty(t *testing.T) {
	t.Parallel()
	s := NotEmpty().Score("", false, nil)
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
}

func TestErrorRate_NoError(t *testing.T) {
	t.Parallel()
	scorer := ErrorRate()
	s := scorer.ScoreResult(Result{Error: false})
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
	if s.Scorer != "error_rate" {
		t.Errorf("scorer = %q, want error_rate", s.Scorer)
	}
}

func TestErrorRate_WithError(t *testing.T) {
	t.Parallel()
	scorer := ErrorRate()
	s := scorer.ScoreResult(Result{Error: true})
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
	if s.Scorer != "error_rate" {
		t.Errorf("scorer = %q, want error_rate", s.Scorer)
	}
	if s.Reason == "" {
		t.Error("expected non-empty reason when error")
	}
	if !strings.Contains(s.Reason, "error") {
		t.Errorf("reason %q does not contain 'error'", s.Reason)
	}
}

func TestLatency_WithinLimit(t *testing.T) {
	t.Parallel()
	scorer := Latency(100 * time.Millisecond)
	s := scorer.ScoreResult(Result{Duration: 50 * time.Millisecond})
	if s.Value != 1.0 {
		t.Errorf("value = %f, want 1.0", s.Value)
	}
}

func TestLatency_ExceedsLimit(t *testing.T) {
	t.Parallel()
	scorer := Latency(100 * time.Millisecond)
	s := scorer.ScoreResult(Result{Duration: 200 * time.Millisecond})
	if s.Value != 0.0 {
		t.Errorf("value = %f, want 0.0", s.Value)
	}
}
