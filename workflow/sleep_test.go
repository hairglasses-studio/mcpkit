package workflow

import (
	"context"
	"testing"
	"time"
)

func TestDurableSleepNodeFunc_ShortSleep(t *testing.T) {
	fn := DurableSleepNodeFunc(10 * time.Millisecond)
	state := NewState()

	start := time.Now()
	result, err := fn(context.Background(), state)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < 10*time.Millisecond {
		t.Errorf("sleep was too short: %v", elapsed)
	}
	// Check that the target was stored in state.
	if _, ok := result.Data[DurableSleepKey]; !ok {
		t.Error("expected DurableSleepKey to be set in state")
	}
}

func TestDurableSleepNodeFunc_ContextCancelled(t *testing.T) {
	fn := DurableSleepNodeFunc(10 * time.Second) // long sleep
	state := NewState()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := fn(ctx, state)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestDurableSleepNodeFunc_ResumeFromCheckpoint(t *testing.T) {
	// Simulate a checkpoint where the target time has already passed.
	state := NewState()
	pastTarget := time.Now().Add(-1 * time.Second)
	state = Set(state, DurableSleepKey, pastTarget.Format(time.RFC3339Nano))

	fn := DurableSleepNodeFunc(10 * time.Second) // would be long, but target is past

	start := time.Now()
	_, err := fn(context.Background(), state)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("resume should be instant, took %v", elapsed)
	}
}

func TestDurableSleepNodeFunc_ResumeWithTimeValue(t *testing.T) {
	// Store time.Time directly (in-process resume, no serialization).
	state := NewState()
	pastTarget := time.Now().Add(-1 * time.Second)
	state.Data[DurableSleepKey] = pastTarget

	fn := DurableSleepNodeFunc(10 * time.Second)

	start := time.Now()
	_, err := fn(context.Background(), state)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("resume should be instant, took %v", elapsed)
	}
}

func TestAddDurableSleepNode(t *testing.T) {
	g := NewGraph()
	err := g.AddDurableSleepNode("wait", 5*time.Second)
	if err != nil {
		t.Fatalf("AddDurableSleepNode: %v", err)
	}

	// Verify the node was added.
	if _, ok := g.nodes["wait"]; !ok {
		t.Error("expected 'wait' node in graph")
	}
}
