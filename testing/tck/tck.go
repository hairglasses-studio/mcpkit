package tck

import (
	"strings"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// CheckResult records the outcome of a single conformance check.
type CheckResult struct {
	Category string
	Name     string
	Passed   bool
	Message  string
	Duration time.Duration
}

// Check is a named conformance check function.
// It receives the registry and returns a CheckResult.
type Check struct {
	Category string
	Name     string
	Fn       func(reg *registry.ToolRegistry) CheckResult
}

// Suite holds the registry under test and accumulated results.
type Suite struct {
	Registry *registry.ToolRegistry
	Results  []CheckResult
	checks   []Check
}

// NewSuite creates a TCK suite targeting the given registry.
// All built-in checks are registered automatically.
func NewSuite(reg *registry.ToolRegistry) *Suite {
	s := &Suite{
		Registry: reg,
	}
	s.checks = append(s.checks, toolChecks()...)
	s.checks = append(s.checks, lifecycleChecks()...)
	return s
}

// AddCheck registers a custom check with the suite.
func (s *Suite) AddCheck(c Check) {
	s.checks = append(s.checks, c)
}

// Run executes all checks as sub-tests and fails the test on any failure.
func (s *Suite) Run(t *testing.T) {
	t.Helper()
	s.Results = nil
	for _, c := range s.checks {
		c := c
		t.Run(c.Category+"/"+c.Name, func(t *testing.T) {
			start := time.Now()
			result := c.Fn(s.Registry)
			result.Duration = time.Since(start)
			result.Category = c.Category
			result.Name = c.Name
			s.Results = append(s.Results, result)
			if !result.Passed {
				t.Errorf("TCK FAIL: %s", result.Message)
			}
		})
	}
}

// RunCategory executes only checks matching the given category.
func (s *Suite) RunCategory(t *testing.T, category string) {
	t.Helper()
	s.Results = nil
	for _, c := range s.checks {
		if c.Category != category {
			continue
		}
		c := c
		t.Run(c.Category+"/"+c.Name, func(t *testing.T) {
			start := time.Now()
			result := c.Fn(s.Registry)
			result.Duration = time.Since(start)
			result.Category = c.Category
			result.Name = c.Name
			s.Results = append(s.Results, result)
			if !result.Passed {
				t.Errorf("TCK FAIL: %s", result.Message)
			}
		})
	}
}

// Summary returns a human-readable summary of the results.
func (s *Suite) Summary() string {
	var b strings.Builder
	passed, failed := 0, 0
	for _, r := range s.Results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}
	b.WriteString("TCK Summary: ")
	b.WriteString(itoa(passed))
	b.WriteString(" passed, ")
	b.WriteString(itoa(failed))
	b.WriteString(" failed, ")
	b.WriteString(itoa(passed+failed))
	b.WriteString(" total\n")
	for _, r := range s.Results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		b.WriteString("  [")
		b.WriteString(status)
		b.WriteString("] ")
		b.WriteString(r.Category)
		b.WriteString("/")
		b.WriteString(r.Name)
		if !r.Passed {
			b.WriteString(": ")
			b.WriteString(r.Message)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// itoa is a minimal int-to-string for avoiding strconv import.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
