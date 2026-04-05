// Package skills provides context-aware lazy tool loading with skill bundles
// and trigger functions for activating tools on demand.
package skills

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TriggerFunc evaluates whether a skill should be activated given the current context.
type TriggerFunc func(ctx SkillContext) bool

// SkillContext provides contextual information for trigger evaluation.
type SkillContext struct {
	ActiveTools []string
	RecentCalls []string
	TaskHints   []string
	Tags        map[string]string
}

// Skill groups related tools with activation conditions.
type Skill struct {
	Name        string
	Description string
	Tools       []string    // tool names in the parent registry
	Trigger     TriggerFunc // when to activate (nil = manual only)
	Priority    int         // higher = loaded first when competing
}

// SkillRegistry manages skill bundles against a DynamicRegistry.
type SkillRegistry struct {
	mu       sync.RWMutex
	skills   map[string]Skill
	active   map[string]bool
	dynReg   *registry.DynamicRegistry
	allTools map[string]registry.ToolDefinition // stashed tools removed from registry
}

// NewSkillRegistry creates a SkillRegistry backed by the given DynamicRegistry.
func NewSkillRegistry(reg *registry.DynamicRegistry) *SkillRegistry {
	return &SkillRegistry{
		skills:   make(map[string]Skill),
		active:   make(map[string]bool),
		dynReg:   reg,
		allTools: make(map[string]registry.ToolDefinition),
	}
}

// Register adds a skill to the registry. The skill's tools are stashed
// (removed from the DynamicRegistry) until the skill is activated.
func (r *SkillRegistry) Register(skill Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name] = skill
	// Stash tool definitions that exist in the dynamic registry.
	for _, toolName := range skill.Tools {
		if td, ok := r.dynReg.GetTool(toolName); ok {
			r.allTools[toolName] = td
			r.dynReg.RemoveTool(toolName)
		}
	}
}

// Activate makes a skill's tools available in the DynamicRegistry.
func (r *SkillRegistry) Activate(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	skill, ok := r.skills[name]
	if !ok {
		return fmt.Errorf("skills: unknown skill %q", name)
	}
	if r.active[name] {
		return nil // already active
	}
	r.active[name] = true
	for _, toolName := range skill.Tools {
		if td, ok := r.allTools[toolName]; ok {
			r.dynReg.AddTool(td)
		}
	}
	return nil
}

// Deactivate removes a skill's tools from the DynamicRegistry.
func (r *SkillRegistry) Deactivate(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	skill, ok := r.skills[name]
	if !ok {
		return fmt.Errorf("skills: unknown skill %q", name)
	}
	if !r.active[name] {
		return nil // already inactive
	}
	delete(r.active, name)
	for _, toolName := range skill.Tools {
		// Only remove if no other active skill claims this tool.
		if !r.toolClaimedByActive(toolName) {
			r.dynReg.RemoveTool(toolName)
		}
	}
	return nil
}

// toolClaimedByActive returns true if any active skill includes the tool.
// Must be called with r.mu held.
func (r *SkillRegistry) toolClaimedByActive(toolName string) bool {
	for sn, skill := range r.skills {
		if !r.active[sn] {
			continue
		}
		if slices.Contains(skill.Tools, toolName) {
			return true
		}
	}
	return false
}

// ActiveSkills returns the names of currently active skills, sorted.
func (r *SkillRegistry) ActiveSkills() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name := range r.active {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Evaluate returns skill names whose triggers fire for the given context,
// sorted by descending priority.
func (r *SkillRegistry) Evaluate(sc SkillContext) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	type scored struct {
		name     string
		priority int
	}
	var matches []scored
	for _, skill := range r.skills {
		if skill.Trigger != nil && skill.Trigger(sc) {
			matches = append(matches, scored{skill.Name, skill.Priority})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].priority != matches[j].priority {
			return matches[i].priority > matches[j].priority
		}
		return matches[i].name < matches[j].name
	})
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.name
	}
	return names
}

// GetSkill returns a skill by name.
func (r *SkillRegistry) GetSkill(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// activeToolCount returns the number of tools currently exposed through active skills.
// Must be called with r.mu held (read or write).
func (r *SkillRegistry) activeToolCount() int {
	seen := make(map[string]bool)
	for name, skill := range r.skills {
		if !r.active[name] {
			continue
		}
		for _, tn := range skill.Tools {
			seen[tn] = true
		}
	}
	return len(seen)
}

// ActiveToolCount returns the number of unique tools currently exposed through active skills.
func (r *SkillRegistry) ActiveToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeToolCount()
}
