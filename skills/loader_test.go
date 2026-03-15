package skills

import (
	"context"
	"fmt"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// makeToolsForSkill creates n tools named "<prefix>_tool_0" ... "<prefix>_tool_n-1"
// and adds them to the registry, returning the tool names.
func makeToolsForSkill(reg *registry.DynamicRegistry, prefix string, n int) []string {
	names := make([]string, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("%s_tool_%d", prefix, i)
		reg.AddTool(makeTool(name))
		names[i] = name
	}
	return names
}

func TestLoaderDefaultConfig(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	sr := NewSkillRegistry(reg)
	loader := NewContextLoader(sr)

	cfg := loader.Config()
	if cfg.MaxActiveTools != 20 {
		t.Errorf("default MaxActiveTools = %d, want 20", cfg.MaxActiveTools)
	}
}

func TestLoaderMaxActiveToolsCap(t *testing.T) {
	reg := registry.NewDynamicRegistry()

	// Register 3 skills with 10 tools each.
	skillA := makeToolsForSkill(reg, "a", 10)
	skillB := makeToolsForSkill(reg, "b", 10)
	skillC := makeToolsForSkill(reg, "c", 10)

	sr := NewSkillRegistry(reg)
	alwaysTrue := func(sc SkillContext) bool { return true }

	sr.Register(Skill{Name: "skill_a", Tools: skillA, Trigger: alwaysTrue, Priority: 30})
	sr.Register(Skill{Name: "skill_b", Tools: skillB, Trigger: alwaysTrue, Priority: 20})
	sr.Register(Skill{Name: "skill_c", Tools: skillC, Trigger: alwaysTrue, Priority: 10})

	// Cap at 15 tools — only skill_a (10 tools) fits fully; skill_b (10 more) would
	// push to 20 which is >= 15, so only skill_a should be activated.
	loader := NewContextLoader(sr, LoaderConfig{MaxActiveTools: 15})

	if err := loader.Update(context.Background(), SkillContext{}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	active := sr.ActiveSkills()
	// skill_a (priority 30) gets 10 tools (< 15), skill_b would add 10 more (20 total >= 15).
	if len(active) != 1 || active[0] != "skill_a" {
		t.Errorf("active skills = %v, want [skill_a]", active)
	}

	count := sr.ActiveToolCount()
	if count > 15 {
		t.Errorf("active tool count = %d, exceeds cap of 15", count)
	}
}

func TestLoaderAutoDeactivate(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("tool_x"))
	reg.AddTool(makeTool("tool_y"))

	sr := NewSkillRegistry(reg)

	fire := true
	sr.Register(Skill{
		Name:    "conditional",
		Tools:   []string{"tool_x"},
		Trigger: func(sc SkillContext) bool { return fire },
	})
	sr.Register(Skill{
		Name:    "always_on",
		Tools:   []string{"tool_y"},
		Trigger: func(sc SkillContext) bool { return true },
	})

	loader := NewContextLoader(sr, LoaderConfig{
		MaxActiveTools: 20,
		AutoDeactivate: true,
	})

	// First update: both triggers fire.
	if err := loader.Update(context.Background(), SkillContext{}); err != nil {
		t.Fatalf("first Update: %v", err)
	}

	active := sr.ActiveSkills()
	if len(active) != 2 {
		t.Errorf("after first update: active = %v, want 2 skills", active)
	}

	// Stop the conditional trigger.
	fire = false

	// Second update: conditional should be auto-deactivated.
	if err := loader.Update(context.Background(), SkillContext{}); err != nil {
		t.Fatalf("second Update: %v", err)
	}

	active = sr.ActiveSkills()
	if len(active) != 1 || active[0] != "always_on" {
		t.Errorf("after second update: active = %v, want [always_on]", active)
	}
}

func TestLoaderNoAutoDeactivateByDefault(t *testing.T) {
	reg := registry.NewDynamicRegistry()
	reg.AddTool(makeTool("tool_z"))

	sr := NewSkillRegistry(reg)

	fire := true
	sr.Register(Skill{
		Name:    "toggled",
		Tools:   []string{"tool_z"},
		Trigger: func(sc SkillContext) bool { return fire },
	})

	// AutoDeactivate is false (default).
	loader := NewContextLoader(sr)

	loader.Update(context.Background(), SkillContext{}) //nolint

	if len(sr.ActiveSkills()) != 1 {
		t.Fatal("expected skill active after first update")
	}

	// Turn trigger off; without AutoDeactivate the skill should stay active.
	fire = false
	loader.Update(context.Background(), SkillContext{}) //nolint

	active := sr.ActiveSkills()
	if len(active) != 1 || active[0] != "toggled" {
		t.Errorf("without AutoDeactivate, skill should stay active; got %v", active)
	}
}
