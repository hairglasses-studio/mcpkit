//go:build !official_sdk

package eval

import "time"

// Case defines a single evaluation case for a tool.
type Case struct {
	Name     string         `json:"name"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args,omitempty"`
	Expected any            `json:"expected,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
}

// Score records the result of a single scorer applied to a case.
type Score struct {
	Scorer string  `json:"scorer"`
	Value  float64 `json:"value"` // 0.0 to 1.0
	Reason string  `json:"reason,omitempty"`
}

// Result records the outcome of running a single case.
type Result struct {
	Case     Case          `json:"case"`
	Scores   []Score       `json:"scores"`
	Output   string        `json:"output"`
	Error    bool          `json:"error"`
	Duration time.Duration `json:"duration"`
	Passed   bool          `json:"passed"`
}

// AverageScore returns the mean of all scores for this result.
func (r Result) AverageScore() float64 {
	if len(r.Scores) == 0 {
		return 0
	}
	var sum float64
	for _, s := range r.Scores {
		sum += s.Value
	}
	return sum / float64(len(r.Scores))
}

// ResultScorer evaluates a full Result including duration and metadata.
type ResultScorer interface {
	Name() string
	ScoreResult(result Result) Score
}

// Suite defines a collection of evaluation cases with shared scorers.
type Suite struct {
	Name          string         `json:"name"`
	Cases         []Case         `json:"cases"`
	Scorers       []Scorer       `json:"-"`
	ResultScorers []ResultScorer `json:"-"`
	Threshold     float64        `json:"threshold"` // minimum average score to pass (default 1.0)
}

// Summary is the aggregate result of running a suite.
type Summary struct {
	Suite    string        `json:"suite"`
	Total    int           `json:"total"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	AvgScore float64       `json:"avg_score"`
	Duration time.Duration `json:"duration"`
	Results  []Result      `json:"results"`
}

// PassRate returns the fraction of cases that passed (0.0 to 1.0).
func (s Summary) PassRate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Passed) / float64(s.Total)
}
