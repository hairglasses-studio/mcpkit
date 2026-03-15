package ralph

import (
	"strings"
	"testing"
)

func TestReadyTasks_NoDeps(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Description: "first"},
		{ID: "t2", Description: "second"},
	}
	ready := ReadyTasks(tasks, map[string]bool{})
	if len(ready) != 2 {
		t.Errorf("ReadyTasks() = %v, want 2 tasks", ready)
	}
}

func TestReadyTasks_WithDeps(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Description: "first"},
		{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
		{ID: "t3", Description: "third", DependsOn: []string{"t2"}},
	}
	ready := ReadyTasks(tasks, map[string]bool{})
	if len(ready) != 1 || ready[0] != "t1" {
		t.Errorf("ReadyTasks() = %v, want [t1]", ready)
	}
}

func TestReadyTasks_AfterCompletion(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Description: "first"},
		{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
		{ID: "t3", Description: "third", DependsOn: []string{"t1", "t2"}},
	}
	completed := map[string]bool{"t1": true}
	ready := ReadyTasks(tasks, completed)
	if len(ready) != 1 || ready[0] != "t2" {
		t.Errorf("ReadyTasks() = %v, want [t2]", ready)
	}
}

func TestReadyTasks_SkipsDone(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Description: "first", Done: true},
		{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
	}
	// t1 is Done in spec, t2 depends on t1 but t1 not in completed map
	// ReadyTasks checks t.Done OR completed[t.ID], so t1 is skipped
	// For t2's dep check, completed map doesn't have t1, so t2 is blocked
	// But wait - the plan says ReadyTasks uses completed map for dep checking.
	// Let's check: t2 depends on t1, but t1 is only Done in the spec, not in completed map.
	// So t2 would be blocked. That's correct - completed map should be authoritative.
	// Actually, let's make this test use completed map too.
	completed := map[string]bool{"t1": true}
	ready := ReadyTasks(tasks, completed)
	// t1 is Done, so skipped. t2's dep t1 is in completed, so t2 is ready.
	if len(ready) != 1 || ready[0] != "t2" {
		t.Errorf("ReadyTasks() = %v, want [t2]", ready)
	}
}

func TestValidateDependencies_Valid(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Description: "first"},
		{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
		{ID: "t3", Description: "third", DependsOn: []string{"t2"}},
	}
	if err := ValidateDependencies(tasks); err != nil {
		t.Errorf("ValidateDependencies() = %v, want nil", err)
	}
}

func TestValidateDependencies_Cycle(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Description: "first", DependsOn: []string{"t2"}},
		{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
	}
	err := ValidateDependencies(tasks)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want mention of 'cycle'", err)
	}
}

func TestValidateDependencies_MissingRef(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Description: "first", DependsOn: []string{"t99"}},
	}
	err := ValidateDependencies(tasks)
	if err == nil {
		t.Fatal("expected error for missing ref")
	}
	if !strings.Contains(err.Error(), "unknown task") {
		t.Errorf("error = %q, want mention of 'unknown task'", err)
	}
}

func TestValidateSpec_WithDependencies(t *testing.T) {
	spec := Spec{
		Name: "test", Description: "test desc",
		Tasks: []Task{
			{ID: "t1", Description: "first"},
			{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
		},
	}
	if err := ValidateSpec(spec); err != nil {
		t.Errorf("ValidateSpec() = %v, want nil", err)
	}
}

func TestValidateSpec_WithCycleDependencies(t *testing.T) {
	spec := Spec{
		Name: "test", Description: "test desc",
		Tasks: []Task{
			{ID: "t1", Description: "first", DependsOn: []string{"t2"}},
			{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
		},
	}
	err := ValidateSpec(spec)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}
