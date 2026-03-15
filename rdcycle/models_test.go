package rdcycle

import "testing"

func TestModelTierConfig_Selector_Default(t *testing.T) {
	cfg := ModelTierConfig{
		Default: "claude-opus-4-6",
	}
	sel := cfg.Selector()

	model := sel(1, nil)
	if model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", model)
	}
}

func TestModelTierConfig_Selector_TaskOverride(t *testing.T) {
	cfg := ModelTierConfig{
		Default: "claude-opus-4-6",
		TaskOverrides: map[string]string{
			"verify": "claude-sonnet-4-6",
			"scan":   "claude-haiku-4-5",
		},
	}
	sel := cfg.Selector()

	// No tasks completed — first uncompleted is "scan", which has override.
	model := sel(1, nil)
	if model != "claude-haiku-4-5" {
		t.Errorf("model = %q, want claude-haiku-4-5", model)
	}

	// scan completed — next is "plan", no override → default.
	model = sel(2, []string{"scan"})
	if model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", model)
	}

	// scan+plan+implement completed — next is "verify", which has override.
	model = sel(5, []string{"scan", "plan", "implement"})
	if model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", model)
	}
}

func TestModelTierConfig_Selector_AllCompleted(t *testing.T) {
	cfg := ModelTierConfig{
		Default: "claude-opus-4-6",
		TaskOverrides: map[string]string{
			"verify": "claude-sonnet-4-6",
		},
	}
	sel := cfg.Selector()

	allDone := []string{"scan", "plan", "implement", "verify", "reflect", "report", "schedule"}
	model := sel(10, allDone)
	if model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", model)
	}
}

func TestModelTierConfig_Selector_EmptyOverrides(t *testing.T) {
	cfg := ModelTierConfig{
		Default:       "claude-sonnet-4-6",
		TaskOverrides: map[string]string{},
	}
	sel := cfg.Selector()

	model := sel(1, []string{"scan"})
	if model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", model)
	}
}
