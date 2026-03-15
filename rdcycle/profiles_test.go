package rdcycle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersonalProfile(t *testing.T) {
	p := PersonalProfile()
	if p.Name != "personal" {
		t.Errorf("name = %q, want personal", p.Name)
	}
	if p.MaxIterations != 50 {
		t.Errorf("max_iterations = %d, want 50", p.MaxIterations)
	}
	if p.DollarBudget != 5.0 {
		t.Errorf("dollar_budget = %f, want 5.0", p.DollarBudget)
	}
	if p.DailyDollarCap != 20.0 {
		t.Errorf("daily_dollar_cap = %f, want 20.0", p.DailyDollarCap)
	}
	if p.MaxTokensPerReq != 4096 {
		t.Errorf("max_tokens_per_req = %d, want 4096", p.MaxTokensPerReq)
	}
	if len(p.ModelPricing) == 0 {
		t.Error("model_pricing should not be empty")
	}
}

func TestWorkAPIProfile(t *testing.T) {
	p := WorkAPIProfile()
	if p.Name != "work-api" {
		t.Errorf("name = %q, want work-api", p.Name)
	}
	if p.MaxIterations != 200 {
		t.Errorf("max_iterations = %d, want 200", p.MaxIterations)
	}
	if p.DollarBudget != 50.0 {
		t.Errorf("dollar_budget = %f, want 50.0", p.DollarBudget)
	}
}

func TestProfileJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	original := PersonalProfile()
	if err := SaveProfile(path, original); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	loaded, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("name = %q, want %q", loaded.Name, original.Name)
	}
	if loaded.MaxIterations != original.MaxIterations {
		t.Errorf("max_iterations = %d, want %d", loaded.MaxIterations, original.MaxIterations)
	}
	if loaded.DollarBudget != original.DollarBudget {
		t.Errorf("dollar_budget = %f, want %f", loaded.DollarBudget, original.DollarBudget)
	}
	if loaded.DailyDollarCap != original.DailyDollarCap {
		t.Errorf("daily_dollar_cap = %f, want %f", loaded.DailyDollarCap, original.DailyDollarCap)
	}
	if loaded.TokenBudget != original.TokenBudget {
		t.Errorf("token_budget = %d, want %d", loaded.TokenBudget, original.TokenBudget)
	}
	if len(loaded.ModelPricing) != len(original.ModelPricing) {
		t.Errorf("model_pricing len = %d, want %d", len(loaded.ModelPricing), len(original.ModelPricing))
	}
}

func TestLoadProfile_NotFound(t *testing.T) {
	_, err := LoadProfile("/nonexistent/profile.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadProfile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := LoadProfile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildFinOpsStack(t *testing.T) {
	p := PersonalProfile()
	tracker, cp, wt := BuildFinOpsStack(p)

	if tracker == nil {
		t.Fatal("tracker is nil")
	}
	if cp == nil {
		t.Fatal("cost policy is nil")
	}
	if wt == nil {
		t.Fatal("windowed tracker is nil")
	}

	// Verify cost policy budget matches profile
	remaining := cp.RemainingBudget()
	if remaining != p.DollarBudget {
		t.Errorf("remaining budget = %f, want %f", remaining, p.DollarBudget)
	}

	// Verify tracker starts at zero
	if tracker.Total() != 0 {
		t.Errorf("tracker total = %d, want 0", tracker.Total())
	}
}
