//go:build !official_sdk

package eval

import (
	"strings"
	"testing"
)

func TestLoadSuiteJSON_Valid(t *testing.T) {
	t.Parallel()
	input := `{
		"name": "test-suite",
		"cases": [
			{"name": "c1", "tool": "echo", "expected": "hello"},
			{"name": "c2", "tool": "echo", "expected": "world"}
		],
		"threshold": 0.8
	}`
	suite, err := LoadSuiteJSON(strings.NewReader(input), []Scorer{Contains()})
	if err != nil {
		t.Fatalf("LoadSuiteJSON: %v", err)
	}
	if suite.Name != "test-suite" {
		t.Errorf("name = %q, want test-suite", suite.Name)
	}
	if len(suite.Cases) != 2 {
		t.Errorf("cases = %d, want 2", len(suite.Cases))
	}
	if suite.Threshold != 0.8 {
		t.Errorf("threshold = %f, want 0.8", suite.Threshold)
	}
	if len(suite.Scorers) != 1 {
		t.Errorf("scorers = %d, want 1", len(suite.Scorers))
	}
}

func TestLoadSuiteJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := LoadSuiteJSON(strings.NewReader("{invalid"), nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadSuiteJSON_EmptyCases(t *testing.T) {
	t.Parallel()
	suite, err := LoadSuiteJSON(strings.NewReader(`{"name":"empty","cases":[]}`), nil)
	if err != nil {
		t.Fatalf("LoadSuiteJSON: %v", err)
	}
	if len(suite.Cases) != 0 {
		t.Errorf("cases = %d, want 0", len(suite.Cases))
	}
}
