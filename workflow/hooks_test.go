package workflow

import (
	"errors"
	"testing"
	"time"
)

// TestHooks_NilCallbacks verifies that calling all internal dispatch methods on
// a zero-value Hooks never panics, even though every function field is nil.
func TestHooks_NilCallbacks(t *testing.T) {
	t.Parallel()
	var h Hooks
	state := NewState()
	cp := Checkpoint{RunID: "r1", State: state, SavedAt: time.Now()}

	// None of these must panic.
	h.callNodeStart("node-a", state)
	h.callNodeEnd("node-a", state)
	h.callNodeError("node-a", errors.New("boom"))
	h.callCheckpoint(cp)
	h.callCycleDetected("node-a", 3)
}

// TestHooks_OnNodeStart_Fires verifies the OnNodeStart callback is invoked with
// the correct node name and state.
func TestHooks_OnNodeStart_Fires(t *testing.T) {
	t.Parallel()

	var gotName string
	var gotStep int

	h := Hooks{
		OnNodeStart: func(name string, s State) {
			gotName = name
			gotStep = s.Step
		},
	}

	state := NewState()
	state.Step = 5
	h.callNodeStart("alpha", state)

	if gotName != "alpha" {
		t.Errorf("OnNodeStart name = %q; want %q", gotName, "alpha")
	}
	if gotStep != 5 {
		t.Errorf("OnNodeStart state.Step = %d; want 5", gotStep)
	}
}

// TestHooks_OnNodeEnd_Fires verifies the OnNodeEnd callback is invoked with the
// correct node name and state.
func TestHooks_OnNodeEnd_Fires(t *testing.T) {
	t.Parallel()

	var gotName string
	var gotKey any

	h := Hooks{
		OnNodeEnd: func(name string, s State) {
			gotName = name
			gotKey = s.Data["result"]
		},
	}

	state := NewState()
	state.Data["result"] = "done"
	h.callNodeEnd("beta", state)

	if gotName != "beta" {
		t.Errorf("OnNodeEnd name = %q; want %q", gotName, "beta")
	}
	if gotKey != "done" {
		t.Errorf("OnNodeEnd state.Data[result] = %v; want %q", gotKey, "done")
	}
}

// TestHooks_OnNodeError_Fires verifies the OnNodeError callback receives the
// correct node name and error value.
func TestHooks_OnNodeError_Fires(t *testing.T) {
	t.Parallel()

	var gotName string
	var gotErr error

	h := Hooks{
		OnNodeError: func(name string, err error) {
			gotName = name
			gotErr = err
		},
	}

	sentinel := errors.New("something went wrong")
	h.callNodeError("gamma", sentinel)

	if gotName != "gamma" {
		t.Errorf("OnNodeError name = %q; want %q", gotName, "gamma")
	}
	if !errors.Is(gotErr, sentinel) {
		t.Errorf("OnNodeError err = %v; want sentinel error", gotErr)
	}
}

// TestHooks_OnCheckpoint_Fires verifies the OnCheckpoint callback receives the
// checkpoint with intact fields.
func TestHooks_OnCheckpoint_Fires(t *testing.T) {
	t.Parallel()

	var gotCP Checkpoint

	h := Hooks{
		OnCheckpoint: func(cp Checkpoint) {
			gotCP = cp
		},
	}

	now := time.Now().UTC()
	cp := Checkpoint{
		RunID:       "cp-run",
		CurrentNode: "node-z",
		Step:        11,
		State:       NewState(),
		SavedAt:     now,
	}
	h.callCheckpoint(cp)

	if gotCP.RunID != "cp-run" {
		t.Errorf("OnCheckpoint RunID = %q; want %q", gotCP.RunID, "cp-run")
	}
	if gotCP.CurrentNode != "node-z" {
		t.Errorf("OnCheckpoint CurrentNode = %q; want %q", gotCP.CurrentNode, "node-z")
	}
	if gotCP.Step != 11 {
		t.Errorf("OnCheckpoint Step = %d; want 11", gotCP.Step)
	}
}

// TestHooks_OnCycleDetected_Fires verifies the OnCycleDetected callback receives
// the correct node name and step count.
func TestHooks_OnCycleDetected_Fires(t *testing.T) {
	t.Parallel()

	var gotName string
	var gotStep int

	h := Hooks{
		OnCycleDetected: func(name string, step int) {
			gotName = name
			gotStep = step
		},
	}

	h.callCycleDetected("looping-node", 42)

	if gotName != "looping-node" {
		t.Errorf("OnCycleDetected name = %q; want %q", gotName, "looping-node")
	}
	if gotStep != 42 {
		t.Errorf("OnCycleDetected step = %d; want 42", gotStep)
	}
}

// TestHooks_CallbackNotFiredWhenNil verifies that when only some callbacks are
// set, the absent ones remain no-ops and do not prevent the present ones from
// firing.
func TestHooks_CallbackNotFiredWhenNil(t *testing.T) {
	t.Parallel()

	fired := false
	h := Hooks{
		// Only OnNodeEnd is set; all others are nil.
		OnNodeEnd: func(_ string, _ State) {
			fired = true
		},
	}

	state := NewState()
	cp := Checkpoint{RunID: "x", State: state, SavedAt: time.Now()}

	// These must not panic with nil handlers.
	h.callNodeStart("n", state)
	h.callNodeError("n", errors.New("e"))
	h.callCheckpoint(cp)
	h.callCycleDetected("n", 1)

	// The one that is set must fire.
	h.callNodeEnd("n", state)

	if !fired {
		t.Error("OnNodeEnd was not called")
	}
}

// TestHooks_AllCallbacksFire verifies that a fully populated Hooks struct fires
// every callback exactly once.
func TestHooks_AllCallbacksFire(t *testing.T) {
	t.Parallel()

	counts := map[string]int{}

	state := NewState()
	cp := Checkpoint{RunID: "all", State: state, SavedAt: time.Now()}

	h := Hooks{
		OnNodeStart:     func(_ string, _ State) { counts["start"]++ },
		OnNodeEnd:       func(_ string, _ State) { counts["end"]++ },
		OnNodeError:     func(_ string, _ error) { counts["error"]++ },
		OnCheckpoint:    func(_ Checkpoint) { counts["checkpoint"]++ },
		OnCycleDetected: func(_ string, _ int) { counts["cycle"]++ },
	}

	h.callNodeStart("n", state)
	h.callNodeEnd("n", state)
	h.callNodeError("n", errors.New("e"))
	h.callCheckpoint(cp)
	h.callCycleDetected("n", 0)

	for _, key := range []string{"start", "end", "error", "checkpoint", "cycle"} {
		if counts[key] != 1 {
			t.Errorf("callback %q fired %d time(s); want 1", key, counts[key])
		}
	}
}
