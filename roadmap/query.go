package roadmap

// NextPhase returns the first phase whose status is not complete, or nil if all phases are complete.
func NextPhase(rm *Roadmap) *Phase {
	for i := range rm.Phases {
		if rm.Phases[i].Status != PhaseStatusComplete {
			return &rm.Phases[i]
		}
	}
	return nil
}

// GapAnalysis returns all work items with status "planned" across all phases and tiers.
func GapAnalysis(rm *Roadmap) []WorkItem {
	var gaps []WorkItem
	for _, phase := range rm.Phases {
		for _, item := range phase.Items {
			if item.Status == ItemStatusPlanned {
				gaps = append(gaps, item)
			}
		}
	}
	for _, tier := range rm.Tiers {
		for _, item := range tier.Items {
			if item.Status == ItemStatusPlanned {
				gaps = append(gaps, item)
			}
		}
	}
	return gaps
}

// ReadyItems returns items within the given phase whose DependsOn items are all complete.
// An item with no dependencies is always considered ready (unless it is already complete).
func ReadyItems(phase *Phase) []WorkItem {
	if phase == nil {
		return nil
	}

	// Build a set of complete item IDs within this phase.
	complete := make(map[string]bool)
	for _, item := range phase.Items {
		if item.Status == ItemStatusComplete {
			complete[item.ID] = true
		}
	}

	var ready []WorkItem
	for _, item := range phase.Items {
		if item.Status == ItemStatusComplete {
			continue
		}
		allDone := true
		for _, dep := range item.DependsOn {
			if !complete[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, item)
		}
	}
	return ready
}

// PhaseByID returns the phase with the given ID, or nil if not found.
func PhaseByID(rm *Roadmap, id string) *Phase {
	for i := range rm.Phases {
		if rm.Phases[i].ID == id {
			return &rm.Phases[i]
		}
	}
	return nil
}
