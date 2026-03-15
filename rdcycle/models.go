package rdcycle

// ModelTierConfig maps task phases to model preferences, allowing cheaper
// models for simpler tasks (e.g., verification) and more capable models
// for complex tasks (e.g., implementation).
type ModelTierConfig struct {
	// Default is the model used when no task-specific override matches.
	Default string
	// TaskOverrides maps task IDs to model names.
	TaskOverrides map[string]string
}

// Selector returns a ModelSelector function suitable for ralph.Config.
// It infers the current task from completedIDs (the next uncompleted task
// in a known sequence) and returns the appropriate model override.
func (c ModelTierConfig) Selector() func(iteration int, completedIDs []string) string {
	return func(iteration int, completedIDs []string) string {
		if len(c.TaskOverrides) == 0 {
			return c.Default
		}

		// Build a set of completed task IDs for fast lookup.
		completed := make(map[string]bool, len(completedIDs))
		for _, id := range completedIDs {
			completed[id] = true
		}

		// Check if any override task is not yet completed — that's likely the current task.
		// Standard R&D cycle task order for inference.
		taskOrder := []string{"scan", "plan", "implement", "verify", "reflect", "report", "schedule"}
		for _, taskID := range taskOrder {
			if !completed[taskID] {
				if model, ok := c.TaskOverrides[taskID]; ok {
					return model
				}
				return c.Default
			}
		}

		// All known tasks completed — fall back to default.
		return c.Default
	}
}
