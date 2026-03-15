//go:build !official_sdk

package dispatcher

import "testing"

func TestNewGroupManager_NilLimits(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(nil)
	if gm.limits == nil {
		t.Error("expected non-nil limits map after nil input")
	}
	if gm.active == nil {
		t.Error("expected non-nil active map")
	}
}

func TestNewGroupManager_WithLimits(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"a": 2, "b": 1})
	if gm.limits["a"] != 2 {
		t.Errorf("expected limit 2 for group a, got %d", gm.limits["a"])
	}
	if gm.limits["b"] != 1 {
		t.Errorf("expected limit 1 for group b, got %d", gm.limits["b"])
	}
}

func TestCanAcquire_EmptyGroup(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"g": 1})
	// Empty string is always allowed.
	if !gm.canAcquire("") {
		t.Error("expected empty group to always be acquirable")
	}
}

func TestCanAcquire_NoLimit(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(nil)
	// Group with no limit — always allowed.
	if !gm.canAcquire("unknown") {
		t.Error("expected group with no configured limit to be acquirable")
	}
}

func TestCanAcquire_ZeroLimit(t *testing.T) {
	t.Parallel()
	// A zero limit means unlimited.
	gm := newGroupManager(map[string]int{"g": 0})
	if !gm.canAcquire("g") {
		t.Error("expected zero limit to be treated as unlimited")
	}
}

func TestCanAcquire_NegativeLimit(t *testing.T) {
	t.Parallel()
	// Negative limit means unlimited.
	gm := newGroupManager(map[string]int{"g": -1})
	if !gm.canAcquire("g") {
		t.Error("expected negative limit to be treated as unlimited")
	}
}

func TestCanAcquire_AtLimit(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"g": 1})
	gm.acquire("g")
	if gm.canAcquire("g") {
		t.Error("expected group at limit to be non-acquirable")
	}
}

func TestCanAcquire_BelowLimit(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"g": 3})
	gm.acquire("g")
	gm.acquire("g")
	// active == 2, limit == 3: still acquirable.
	if !gm.canAcquire("g") {
		t.Error("expected group below limit to be acquirable")
	}
}

func TestAcquire_EmptyGroupNoOp(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(nil)
	gm.acquire("")
	if gm.active[""] != 0 {
		t.Errorf("expected empty group to not increment active, got %d", gm.active[""])
	}
}

func TestRelease_DecrementsActive(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"g": 2})
	gm.acquire("g")
	gm.acquire("g")
	gm.release("g")
	if gm.active["g"] != 1 {
		t.Errorf("expected active 1 after release, got %d", gm.active["g"])
	}
}

func TestRelease_FloorsAtZero(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(nil)
	// Release without acquire — must not go negative.
	gm.release("g")
	if gm.active["g"] != 0 {
		t.Errorf("expected active to floor at 0, got %d", gm.active["g"])
	}
}

func TestRelease_EmptyGroupNoOp(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(nil)
	gm.release("") // must not panic
	if gm.active[""] != 0 {
		t.Error("expected empty group release to be a no-op")
	}
}

func TestSnapshot_IsCopy(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"g": 5})
	gm.acquire("g")
	gm.acquire("g")

	snap := gm.snapshot()
	if snap["g"] != 2 {
		t.Errorf("expected snapshot active 2, got %d", snap["g"])
	}

	// Mutate snapshot — should not affect original.
	snap["g"] = 99
	if gm.active["g"] != 2 {
		t.Error("snapshot mutation affected original active map")
	}
}

func TestSnapshot_Empty(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(nil)
	snap := gm.snapshot()
	if len(snap) != 0 {
		t.Errorf("expected empty snapshot, got %v", snap)
	}
}

func TestAcquireReleaseCycle(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"g": 1})

	// Acquire fills the limit.
	gm.acquire("g")
	if gm.canAcquire("g") {
		t.Error("expected group to be at limit after acquire")
	}

	// Release makes it available again.
	gm.release("g")
	if !gm.canAcquire("g") {
		t.Error("expected group to be acquirable after release")
	}
}

func TestMultipleGroups_Independent(t *testing.T) {
	t.Parallel()
	gm := newGroupManager(map[string]int{"a": 1, "b": 2})
	gm.acquire("a") // a is now at limit

	// b is independent — still acquirable.
	if !gm.canAcquire("b") {
		t.Error("expected group b to be acquirable independent of group a")
	}
	// a is at limit.
	if gm.canAcquire("a") {
		t.Error("expected group a to be non-acquirable at limit")
	}
}
