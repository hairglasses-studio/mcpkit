package ralph

import "fmt"

// ReadyTasks returns IDs of tasks not yet completed whose DependsOn are all satisfied.
func ReadyTasks(tasks []Task, completed map[string]bool) []string {
	var ready []string
	for _, t := range tasks {
		if t.Done || completed[t.ID] {
			continue
		}
		allMet := true
		for _, dep := range t.DependsOn {
			if !completed[dep] {
				allMet = false
				break
			}
		}
		if allMet {
			ready = append(ready, t.ID)
		}
	}
	return ready
}

// ValidateDependencies checks all DependsOn refs exist and there are no cycles.
// Uses Kahn's algorithm for cycle detection.
func ValidateDependencies(tasks []Task) error {
	ids := make(map[string]bool)
	for _, t := range tasks {
		ids[t.ID] = true
	}

	// Check all refs exist.
	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			if !ids[dep] {
				return fmt.Errorf("ralph: task %q depends on unknown task %q", t.ID, dep)
			}
		}
	}

	// Kahn's algorithm for cycle detection.
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // dep -> tasks that depend on it
	for _, t := range tasks {
		if _, ok := inDegree[t.ID]; !ok {
			inDegree[t.ID] = 0
		}
		for _, dep := range t.DependsOn {
			inDegree[t.ID]++
			dependents[dep] = append(dependents[dep], t.ID)
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, dep := range dependents[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if visited != len(tasks) {
		return fmt.Errorf("ralph: dependency cycle detected among tasks")
	}

	return nil
}
