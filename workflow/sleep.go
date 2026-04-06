package workflow

import (
	"context"
	"fmt"
	"time"
)

// DurableSleepKey is the state key where the DurableSleep node stores
// the wake-up time. This allows checkpoint/restore to resume correctly
// even if the process was restarted during the sleep.
const DurableSleepKey = "__durable_sleep_until__"

// DurableSleepNodeFunc creates a NodeFunc that sleeps for the given duration
// in a checkpoint-safe manner. Instead of a simple time.Sleep, it stores the
// target wake-up time in the state, enabling recovery after a restart.
//
// On resume (when the checkpoint is restored), the function checks if the
// target time has already passed. If so, it returns immediately. If not, it
// sleeps for the remaining duration.
//
// The sleep is context-aware and will return early if the context is cancelled.
func DurableSleepNodeFunc(duration time.Duration) NodeFunc {
	return func(ctx context.Context, state State) (State, error) {
		// Check if there's an existing target time from a previous checkpoint.
		var target time.Time
		if raw, ok := state.Data[DurableSleepKey]; ok {
			if ts, ok := raw.(string); ok {
				if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
					target = parsed
				}
			}
			// Also handle time.Time directly (in-process resume, no serialization).
			if t, ok := raw.(time.Time); ok {
				target = t
			}
		}

		// If no target yet, compute it and store in state.
		if target.IsZero() {
			target = time.Now().Add(duration)
			state = Set(state, DurableSleepKey, target.Format(time.RFC3339Nano))
		}

		// Calculate remaining sleep time.
		remaining := time.Until(target)
		if remaining <= 0 {
			// Already past the target — no sleep needed (resumed from checkpoint).
			return state, nil
		}

		// Sleep with context cancellation support.
		timer := time.NewTimer(remaining)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return state, fmt.Errorf("durable sleep cancelled: %w", ctx.Err())
		case <-timer.C:
			return state, nil
		}
	}
}

// AddDurableSleepNode is a convenience that adds a durable sleep node to the graph.
func (g *Graph) AddDurableSleepNode(name string, duration time.Duration, opts ...NodeOption) error {
	return g.AddNode(name, DurableSleepNodeFunc(duration), opts...)
}
