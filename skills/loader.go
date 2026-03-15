package skills

import (
	"context"
	"sync"
)

// LoaderConfig configures the ContextLoader.
type LoaderConfig struct {
	MaxActiveTools int  // global cap on tools from skills (default 20)
	AutoDeactivate bool // deactivate skills whose triggers no longer fire
}

// ContextLoader auto-manages skill activation based on context evaluation.
type ContextLoader struct {
	mu       sync.Mutex
	skillReg *SkillRegistry
	config   LoaderConfig
}

// NewContextLoader creates a ContextLoader for the given SkillRegistry.
func NewContextLoader(skills *SkillRegistry, config ...LoaderConfig) *ContextLoader {
	cfg := LoaderConfig{MaxActiveTools: 20}
	if len(config) > 0 {
		cfg = config[0]
		if cfg.MaxActiveTools <= 0 {
			cfg.MaxActiveTools = 20
		}
	}
	return &ContextLoader{
		skillReg: skills,
		config:   cfg,
	}
}

// Update evaluates all skill triggers and activates/deactivates as needed.
// It respects MaxActiveTools by activating highest-priority skills first.
func (l *ContextLoader) Update(ctx context.Context, sc SkillContext) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	triggered := l.skillReg.Evaluate(sc)

	// Auto-deactivate skills whose triggers no longer fire.
	if l.config.AutoDeactivate {
		triggeredSet := make(map[string]bool, len(triggered))
		for _, name := range triggered {
			triggeredSet[name] = true
		}
		for _, name := range l.skillReg.ActiveSkills() {
			skill, ok := l.skillReg.GetSkill(name)
			if !ok {
				continue
			}
			// Only auto-deactivate skills that have triggers.
			if skill.Trigger != nil && !triggeredSet[name] {
				if err := l.skillReg.Deactivate(name); err != nil {
					return err
				}
			}
		}
	}

	// Activate triggered skills, respecting MaxActiveTools.
	for _, name := range triggered {
		current := l.skillReg.ActiveToolCount()
		if current >= l.config.MaxActiveTools {
			break
		}
		// Count how many new tools this skill would add.
		skill, ok := l.skillReg.GetSkill(name)
		if !ok {
			continue
		}
		newTools := 0
		for _, toolName := range skill.Tools {
			if _, stashed := l.skillReg.allTools[toolName]; stashed {
				newTools++
			}
		}
		if current+newTools > l.config.MaxActiveTools {
			// Skip this skill; it would push us over the cap.
			continue
		}
		if err := l.skillReg.Activate(ctx, name); err != nil {
			return err
		}
	}

	return nil
}

// Config returns the current loader configuration.
func (l *ContextLoader) Config() LoaderConfig {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.config
}
