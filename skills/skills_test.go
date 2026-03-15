package skills

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func makeTool(name string) registry.ToolDefinition {
	return registry.ToolDefinition{
		Tool: registry.Tool{Name: name},
		Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
			return registry.MakeTextResult("ok"), nil
		},
	}
}

func toolInRegistry(reg *registry.DynamicRegistry, name string) bool {
	_, ok := reg.GetTool(name)
	return ok
}

func TestRegisterStashesTools(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("tool_a"))
	reg.AddTool(makeTool("tool_b"))

	sr := NewSkillRegistry(reg)
	sr.Register(Skill{
		Name:  "skill1",
		Tools: []string{"tool_a", "tool_b"},
	})

	// Tools should be stashed (removed from the registry).
	if toolInRegistry(reg, "tool_a") {
		t.Error("tool_a should have been stashed (removed from registry)")
	}
	if toolInRegistry(reg, "tool_b") {
		t.Error("tool_b should have been stashed (removed from registry)")
	}
}

func TestActivateMakesToolsAvailable(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("tool_a"))

	sr := NewSkillRegistry(reg)
	sr.Register(Skill{
		Name:  "skill1",
		Tools: []string{"tool_a"},
	})

	if err := sr.Activate(context.Background(), "skill1"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	if !toolInRegistry(reg, "tool_a") {
		t.Error("tool_a should be present in registry after activation")
	}

	active := sr.ActiveSkills()
	if len(active) != 1 || active[0] != "skill1" {
		t.Errorf("ActiveSkills = %v, want [skill1]", active)
	}
}

func TestDeactivateRemovesTools(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("tool_a"))

	sr := NewSkillRegistry(reg)
	sr.Register(Skill{
		Name:  "skill1",
		Tools: []string{"tool_a"},
	})

	sr.Activate(context.Background(), "skill1") //nolint

	if err := sr.Deactivate("skill1"); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	if toolInRegistry(reg, "tool_a") {
		t.Error("tool_a should have been removed from registry after deactivation")
	}

	if len(sr.ActiveSkills()) != 0 {
		t.Error("no skills should be active after deactivation")
	}
}

func TestActivateIsIdempotent(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("tool_a"))

	sr := NewSkillRegistry(reg)
	sr.Register(Skill{Name: "skill1", Tools: []string{"tool_a"}})

	if err := sr.Activate(context.Background(), "skill1"); err != nil {
		t.Fatalf("first Activate: %v", err)
	}
	if err := sr.Activate(context.Background(), "skill1"); err != nil {
		t.Fatalf("second Activate (idempotent): %v", err)
	}
}

func TestDeactivateIsIdempotent(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("tool_a"))

	sr := NewSkillRegistry(reg)
	sr.Register(Skill{Name: "skill1", Tools: []string{"tool_a"}})

	sr.Activate(context.Background(), "skill1") //nolint

	if err := sr.Deactivate("skill1"); err != nil {
		t.Fatalf("first Deactivate: %v", err)
	}
	if err := sr.Deactivate("skill1"); err != nil {
		t.Fatalf("second Deactivate (idempotent): %v", err)
	}
}

func TestUnknownSkillReturnsError(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	sr := NewSkillRegistry(reg)

	if err := sr.Activate(context.Background(), "nonexistent"); err == nil {
		t.Error("Activate with unknown skill should return error")
	}
	if err := sr.Deactivate("nonexistent"); err == nil {
		t.Error("Deactivate with unknown skill should return error")
	}
}

func TestActiveSkillsSorted(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("t1"))
	reg.AddTool(makeTool("t2"))
	reg.AddTool(makeTool("t3"))

	sr := NewSkillRegistry(reg)
	sr.Register(Skill{Name: "charlie", Tools: []string{"t1"}})
	sr.Register(Skill{Name: "alpha", Tools: []string{"t2"}})
	sr.Register(Skill{Name: "bravo", Tools: []string{"t3"}})

	sr.Activate(context.Background(), "charlie") //nolint
	sr.Activate(context.Background(), "alpha")   //nolint
	sr.Activate(context.Background(), "bravo")   //nolint

	active := sr.ActiveSkills()
	if len(active) != 3 {
		t.Fatalf("want 3 active skills, got %d", len(active))
	}
	if active[0] != "alpha" || active[1] != "bravo" || active[2] != "charlie" {
		t.Errorf("ActiveSkills not sorted: %v", active)
	}
}

func TestOverlappingToolsStayOnPartialDeactivate(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("shared_tool"))

	sr := NewSkillRegistry(reg)
	// Two skills share the same tool.
	sr.Register(Skill{Name: "skill_a", Tools: []string{"shared_tool"}})
	// shared_tool is now stashed. Register second skill referencing same tool name.
	sr.Register(Skill{Name: "skill_b", Tools: []string{"shared_tool"}})

	sr.Activate(context.Background(), "skill_a") //nolint
	sr.Activate(context.Background(), "skill_b") //nolint

	// Deactivate only skill_a; skill_b still claims shared_tool.
	if err := sr.Deactivate("skill_a"); err != nil {
		t.Fatalf("Deactivate skill_a: %v", err)
	}

	if !toolInRegistry(reg, "shared_tool") {
		t.Error("shared_tool should remain in registry because skill_b is still active")
	}

	// Now deactivate skill_b; tool should be removed.
	if err := sr.Deactivate("skill_b"); err != nil {
		t.Fatalf("Deactivate skill_b: %v", err)
	}

	if toolInRegistry(reg, "shared_tool") {
		t.Error("shared_tool should be removed after both skills are deactivated")
	}
}

func TestEvaluatePriorityOrdering(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("t1"))
	reg.AddTool(makeTool("t2"))
	reg.AddTool(makeTool("t3"))

	sr := NewSkillRegistry(reg)
	alwaysTrue := func(sc SkillContext) bool { return true }

	sr.Register(Skill{Name: "low", Tools: []string{"t1"}, Trigger: alwaysTrue, Priority: 1})
	sr.Register(Skill{Name: "high", Tools: []string{"t2"}, Trigger: alwaysTrue, Priority: 10})
	sr.Register(Skill{Name: "mid", Tools: []string{"t3"}, Trigger: alwaysTrue, Priority: 5})

	results := sr.Evaluate(SkillContext{})
	if len(results) != 3 {
		t.Fatalf("expected 3 evaluated skills, got %d", len(results))
	}
	if results[0] != "high" {
		t.Errorf("first result should be 'high' (priority 10), got %q", results[0])
	}
	if results[1] != "mid" {
		t.Errorf("second result should be 'mid' (priority 5), got %q", results[1])
	}
	if results[2] != "low" {
		t.Errorf("third result should be 'low' (priority 1), got %q", results[2])
	}
}

func TestEvaluateNilTriggerNotReturned(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("t1"))
	reg.AddTool(makeTool("t2"))

	sr := NewSkillRegistry(reg)
	// nil trigger = manual only, should never appear in Evaluate
	sr.Register(Skill{Name: "manual", Tools: []string{"t1"}, Trigger: nil})
	sr.Register(Skill{
		Name:    "auto",
		Tools:   []string{"t2"},
		Trigger: func(sc SkillContext) bool { return true },
	})

	results := sr.Evaluate(SkillContext{})
	if len(results) != 1 || results[0] != "auto" {
		t.Errorf("Evaluate should return only 'auto', got %v", results)
	}
}

func TestGetSkill(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("t1"))

	sr := NewSkillRegistry(reg)
	sr.Register(Skill{Name: "myskill", Tools: []string{"t1"}, Priority: 7})

	s, ok := sr.GetSkill("myskill")
	if !ok {
		t.Fatal("GetSkill returned false for registered skill")
	}
	if s.Name != "myskill" || s.Priority != 7 {
		t.Errorf("GetSkill returned unexpected skill: %+v", s)
	}

	_, ok = sr.GetSkill("missing")
	if ok {
		t.Error("GetSkill should return false for missing skill")
	}
}
