package ab

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func echoRunner(_ context.Context, prompt string) (string, error) {
	return prompt, nil
}

func lengthScorer(output string) float64 {
	l := float64(len(output))
	if l > 100 {
		return 1.0
	}
	return l / 100.0
}

func TestVariant_Render(t *testing.T) {
	tests := []struct {
		name    string
		v       Variant
		data    map[string]any
		want    string
		wantErr bool
	}{
		{
			name: "simple",
			v:    Variant{Name: "a", Template: "Hello {{.Name}}"},
			data: map[string]any{"Name": "World"},
			want: "Hello World",
		},
		{
			name: "no data",
			v:    Variant{Name: "a", Template: "static prompt"},
			data: nil,
			want: "static prompt",
		},
		{
			name:    "empty template",
			v:       Variant{Name: "a", Template: ""},
			data:    nil,
			wantErr: true,
		},
		{
			name:    "invalid template",
			v:       Variant{Name: "a", Template: "{{.Broken"},
			data:    nil,
			wantErr: true,
		},
		{
			name: "missing field renders no value",
			v:    Variant{Name: "a", Template: "Hello {{.Missing}}"},
			data: map[string]any{"Name": "x"},
			want: "Hello <no value>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.v.Render(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestABTest_Validate(t *testing.T) {
	validTest := ABTest{
		Name:     "test",
		VariantA: Variant{Name: "a", Template: "A"},
		VariantB: Variant{Name: "b", Template: "B"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	tests := []struct {
		name    string
		modify  func(ABTest) ABTest
		wantErr bool
	}{
		{"valid", func(t ABTest) ABTest { return t }, false},
		{"no name", func(t ABTest) ABTest { t.Name = ""; return t }, true},
		{"no variant a name", func(t ABTest) ABTest { t.VariantA.Name = ""; return t }, true},
		{"no variant a template", func(t ABTest) ABTest { t.VariantA.Template = ""; return t }, true},
		{"no variant b name", func(t ABTest) ABTest { t.VariantB.Name = ""; return t }, true},
		{"no variant b template", func(t ABTest) ABTest { t.VariantB.Template = ""; return t }, true},
		{"no scorer", func(t ABTest) ABTest { t.Scorer = nil; return t }, true},
		{"no runner", func(t ABTest) ABTest { t.Runner = nil; return t }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test := tt.modify(validTest)
			err := test.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRun_Basic(t *testing.T) {
	test := ABTest{
		Name:     "length-test",
		VariantA: Variant{Name: "short", Template: "hi"},
		VariantB: Variant{Name: "long", Template: "hello this is a much longer prompt template"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	result, err := Run(context.Background(), test, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Name != "length-test" {
		t.Errorf("Name = %q, want 'length-test'", result.Name)
	}
	if result.A.Name != "short" {
		t.Errorf("A.Name = %q, want 'short'", result.A.Name)
	}
	if result.B.Name != "long" {
		t.Errorf("B.Name = %q, want 'long'", result.B.Name)
	}

	// B should win (longer output)
	if result.Winner != "long" {
		t.Errorf("Winner = %q, want 'long'", result.Winner)
	}
	if result.B.Score <= result.A.Score {
		t.Errorf("B.Score (%f) should be > A.Score (%f)", result.B.Score, result.A.Score)
	}
	if result.ScoreDelta >= 0 {
		t.Errorf("ScoreDelta = %f, should be negative (B wins)", result.ScoreDelta)
	}
}

func TestRun_Tie(t *testing.T) {
	test := ABTest{
		Name:     "tie-test",
		VariantA: Variant{Name: "a", Template: "same"},
		VariantB: Variant{Name: "b", Template: "same"},
		Scorer:   func(output string) float64 { return 0.5 },
		Runner:   echoRunner,
	}

	result, err := Run(context.Background(), test, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Winner != "" {
		t.Errorf("Winner = %q, want empty for tie", result.Winner)
	}
	if result.ScoreDelta != 0 {
		t.Errorf("ScoreDelta = %f, want 0 for tie", result.ScoreDelta)
	}
}

func TestRun_WithTemplateData(t *testing.T) {
	test := ABTest{
		Name:     "template-test",
		VariantA: Variant{Name: "formal", Template: "Dear {{.Name}}, please review."},
		VariantB: Variant{Name: "casual", Template: "Hey {{.Name}}!"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	input := map[string]any{"Name": "Alice"}
	result, err := Run(context.Background(), test, input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !strings.Contains(result.A.Prompt, "Alice") {
		t.Errorf("A.Prompt = %q, should contain 'Alice'", result.A.Prompt)
	}
	if !strings.Contains(result.B.Prompt, "Alice") {
		t.Errorf("B.Prompt = %q, should contain 'Alice'", result.B.Prompt)
	}

	// Formal is longer
	if result.Winner != "formal" {
		t.Errorf("Winner = %q, want 'formal'", result.Winner)
	}
}

func TestRun_ValidationError(t *testing.T) {
	test := ABTest{} // invalid, no name
	_, err := Run(context.Background(), test, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRun_RunnerError(t *testing.T) {
	failRunner := func(ctx context.Context, prompt string) (string, error) {
		return "", errors.New("runner failed")
	}

	test := ABTest{
		Name:     "fail-test",
		VariantA: Variant{Name: "a", Template: "prompt A"},
		VariantB: Variant{Name: "b", Template: "prompt B"},
		Scorer:   lengthScorer,
		Runner:   failRunner,
	}

	result, err := Run(context.Background(), test, nil)
	if err != nil {
		t.Fatalf("Run() should not return error for runner failures, got: %v", err)
	}

	if result.A.Error == "" {
		t.Error("A.Error should be set")
	}
	if result.B.Error == "" {
		t.Error("B.Error should be set")
	}
	if result.A.Score != 0 || result.B.Score != 0 {
		t.Error("scores should be 0 on runner error")
	}
}

func TestRun_TemplateError(t *testing.T) {
	test := ABTest{
		Name:     "template-err",
		VariantA: Variant{Name: "a", Template: "{{.Broken"},
		VariantB: Variant{Name: "b", Template: "valid"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	result, err := Run(context.Background(), test, map[string]any{})
	if err != nil {
		t.Fatalf("Run() should not return error for template failures, got: %v", err)
	}

	if result.A.Error == "" {
		t.Error("A.Error should be set for template parse failure")
	}
	if result.B.Error != "" {
		t.Errorf("B.Error should not be set, got: %s", result.B.Error)
	}
}

func TestRun_VariantAWins(t *testing.T) {
	test := ABTest{
		Name:     "a-wins",
		VariantA: Variant{Name: "a", Template: "this is a longer prompt that should score higher"},
		VariantB: Variant{Name: "b", Template: "short"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	result, err := Run(context.Background(), test, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Winner != "a" {
		t.Errorf("Winner = %q, want 'a'", result.Winner)
	}
	if result.ScoreDelta <= 0 {
		t.Errorf("ScoreDelta = %f, should be positive (A wins)", result.ScoreDelta)
	}
}

func TestRunN_Basic(t *testing.T) {
	callCount := 0
	runner := func(ctx context.Context, prompt string) (string, error) {
		callCount++
		return prompt, nil
	}

	test := ABTest{
		Name:     "runN",
		VariantA: Variant{Name: "a", Template: "short"},
		VariantB: Variant{Name: "b", Template: "this is much longer than the other variant template"},
		Scorer:   lengthScorer,
		Runner:   runner,
	}

	agg, err := RunN(context.Background(), test, nil, 5)
	if err != nil {
		t.Fatalf("RunN() error = %v", err)
	}

	if agg.Name != "runN" {
		t.Errorf("Name = %q, want 'runN'", agg.Name)
	}
	if agg.Runs != 5 {
		t.Errorf("Runs = %d, want 5", agg.Runs)
	}
	if len(agg.Results) != 5 {
		t.Errorf("Results count = %d, want 5", len(agg.Results))
	}
	if callCount != 10 { // 2 variants * 5 runs
		t.Errorf("callCount = %d, want 10", callCount)
	}

	// B should always win (longer)
	if agg.Winner != "b" {
		t.Errorf("Winner = %q, want 'b'", agg.Winner)
	}
	if agg.WinsB != 5 {
		t.Errorf("WinsB = %d, want 5", agg.WinsB)
	}
	if agg.WinsA != 0 {
		t.Errorf("WinsA = %d, want 0", agg.WinsA)
	}
	if agg.Ties != 0 {
		t.Errorf("Ties = %d, want 0", agg.Ties)
	}
	if agg.AvgScoreB <= agg.AvgScoreA {
		t.Errorf("AvgScoreB (%f) should be > AvgScoreA (%f)", agg.AvgScoreB, agg.AvgScoreA)
	}
	// Confidence should be high (consistent winner)
	if agg.Confidence < 0.5 {
		t.Errorf("Confidence = %f, expected >= 0.5 for consistent results", agg.Confidence)
	}
}

func TestRunN_InvalidN(t *testing.T) {
	test := ABTest{
		Name:     "test",
		VariantA: Variant{Name: "a", Template: "A"},
		VariantB: Variant{Name: "b", Template: "B"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	_, err := RunN(context.Background(), test, nil, 0)
	if err == nil {
		t.Fatal("expected error for n=0")
	}

	_, err = RunN(context.Background(), test, nil, -1)
	if err == nil {
		t.Fatal("expected error for n=-1")
	}
}

func TestRunN_ValidationError(t *testing.T) {
	_, err := RunN(context.Background(), ABTest{}, nil, 5)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRunN_AllTies(t *testing.T) {
	test := ABTest{
		Name:     "tie-test",
		VariantA: Variant{Name: "a", Template: "same"},
		VariantB: Variant{Name: "b", Template: "same"},
		Scorer:   func(output string) float64 { return 0.5 },
		Runner:   echoRunner,
	}

	agg, err := RunN(context.Background(), test, nil, 3)
	if err != nil {
		t.Fatalf("RunN() error = %v", err)
	}

	if agg.Winner != "" {
		t.Errorf("Winner = %q, want empty for all ties", agg.Winner)
	}
	if agg.Ties != 3 {
		t.Errorf("Ties = %d, want 3", agg.Ties)
	}
	if agg.Confidence != 0 {
		t.Errorf("Confidence = %f, want 0 for all ties", agg.Confidence)
	}
}

func TestRunN_SingleRun(t *testing.T) {
	test := ABTest{
		Name:     "single",
		VariantA: Variant{Name: "a", Template: "alpha"},
		VariantB: Variant{Name: "b", Template: "beta beta beta"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	agg, err := RunN(context.Background(), test, nil, 1)
	if err != nil {
		t.Fatalf("RunN() error = %v", err)
	}

	if agg.Runs != 1 {
		t.Errorf("Runs = %d, want 1", agg.Runs)
	}
	if len(agg.Results) != 1 {
		t.Errorf("Results = %d, want 1", len(agg.Results))
	}
}

func TestRunN_Confidence_HighVariance(t *testing.T) {
	// Alternating scorer: A scores high on odd calls, B scores high on even
	callNum := 0
	alternatingRunner := func(ctx context.Context, prompt string) (string, error) {
		callNum++
		if callNum%2 == 1 {
			return strings.Repeat("x", 50), nil // medium score
		}
		return strings.Repeat("x", 80), nil // high score
	}

	test := ABTest{
		Name:     "variance",
		VariantA: Variant{Name: "a", Template: "prompt A"},
		VariantB: Variant{Name: "b", Template: "prompt B"},
		Scorer:   lengthScorer,
		Runner:   alternatingRunner,
	}

	agg, err := RunN(context.Background(), test, nil, 4)
	if err != nil {
		t.Fatalf("RunN() error = %v", err)
	}

	// Should have some results
	if agg.Runs != 4 {
		t.Errorf("Runs = %d, want 4", agg.Runs)
	}
}

func TestRunVariant_Duration(t *testing.T) {
	test := ABTest{
		Name:     "duration",
		VariantA: Variant{Name: "a", Template: "test"},
		VariantB: Variant{Name: "b", Template: "test"},
		Scorer:   lengthScorer,
		Runner:   echoRunner,
	}

	result, err := Run(context.Background(), test, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Duration should be non-negative
	if result.A.Duration < 0 {
		t.Errorf("A.Duration = %v, should be >= 0", result.A.Duration)
	}
	if result.B.Duration < 0 {
		t.Errorf("B.Duration = %v, should be >= 0", result.B.Duration)
	}
}

func ExampleRun() {
	test := ABTest{
		Name:     "greeting-style",
		VariantA: Variant{Name: "formal", Template: "Dear {{.Name}}, please provide feedback."},
		VariantB: Variant{Name: "casual", Template: "Hey {{.Name}}!"},
		Scorer:   func(output string) float64 { return float64(len(output)) / 50.0 },
		Runner: func(ctx context.Context, prompt string) (string, error) {
			return prompt, nil
		},
	}

	result, err := Run(context.Background(), test, map[string]any{"Name": "World"})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	fmt.Printf("winner: %s\n", result.Winner)
	// Output: winner: formal
}

func ExampleRunN() {
	test := ABTest{
		Name:     "prompt-length",
		VariantA: Variant{Name: "verbose", Template: "Please explain in detail"},
		VariantB: Variant{Name: "concise", Template: "Explain"},
		Scorer:   func(output string) float64 { return float64(len(output)) / 30.0 },
		Runner: func(ctx context.Context, prompt string) (string, error) {
			return prompt, nil
		},
	}

	agg, err := RunN(context.Background(), test, nil, 3)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	fmt.Printf("winner after %d runs: %s (confidence: %.2f)\n", agg.Runs, agg.Winner, agg.Confidence)
	// Output: winner after 3 runs: verbose (confidence: 1.00)
}
