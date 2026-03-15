package dispatcher

// groupManager tracks concurrency limits per named group.
// It is not goroutine-safe on its own; the dispatcher's main lock must be
// held whenever any method is called.
type groupManager struct {
	limits map[string]int // maximum concurrent jobs per group (0 or absent = unlimited)
	active map[string]int // current in-flight jobs per group
}

// newGroupManager creates a groupManager with the provided limit map.
// A nil limits map is treated as no limits.
func newGroupManager(limits map[string]int) *groupManager {
	if limits == nil {
		limits = make(map[string]int)
	}
	return &groupManager{
		limits: limits,
		active: make(map[string]int),
	}
}

// canAcquire reports whether a new job for the given group may start.
// It returns true when:
//   - group is the empty string (ungrouped jobs are always allowed), or
//   - the group has no configured limit, or
//   - the group's active count is below its limit.
func (gm *groupManager) canAcquire(group string) bool {
	if group == "" {
		return true
	}
	limit, ok := gm.limits[group]
	if !ok || limit <= 0 {
		return true
	}
	return gm.active[group] < limit
}

// acquire increments the active count for group.
func (gm *groupManager) acquire(group string) {
	if group == "" {
		return
	}
	gm.active[group]++
}

// release decrements the active count for group, floored at 0.
func (gm *groupManager) release(group string) {
	if group == "" {
		return
	}
	if gm.active[group] > 0 {
		gm.active[group]--
	}
}

// snapshot returns a shallow copy of the active map for inspection.
func (gm *groupManager) snapshot() map[string]int {
	out := make(map[string]int, len(gm.active))
	for k, v := range gm.active {
		out[k] = v
	}
	return out
}
