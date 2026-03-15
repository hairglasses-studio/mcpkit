package workflow

import (
	"context"
	"fmt"
	"sync"
)

// CompensateFunc reverses the effect of a completed node.
type CompensateFunc func(ctx context.Context, state State) error

// CompensationRecord tracks a completed compensable step for rollback.
type CompensationRecord struct {
	NodeName   string
	Compensate CompensateFunc
	State      State // state snapshot at completion
}

// CompensationStack is a LIFO stack of compensation records.
// On failure, compensations run in reverse order (saga pattern).
type CompensationStack struct {
	mu      sync.Mutex
	records []CompensationRecord
}

// NewCompensationStack creates an empty compensation stack.
func NewCompensationStack() *CompensationStack {
	return &CompensationStack{}
}

// Push adds a compensation record to the stack.
func (cs *CompensationStack) Push(r CompensationRecord) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.records = append(cs.records, r)
}

// Len returns the number of records.
func (cs *CompensationStack) Len() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.records)
}

// Compensate runs all compensation functions in reverse order (LIFO).
// Returns all errors encountered — compensation does not stop on first error.
func (cs *CompensationStack) Compensate(ctx context.Context) []error {
	cs.mu.Lock()
	records := make([]CompensationRecord, len(cs.records))
	copy(records, cs.records)
	cs.mu.Unlock()

	var errs []error
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		if err := r.Compensate(ctx, r.State); err != nil {
			errs = append(errs, fmt.Errorf("compensate %q: %w", r.NodeName, err))
		}
	}
	return errs
}

// AddCompensableNode adds a node that has a compensation (rollback) function.
// On success, the forward function runs and a CompensationRecord is pushed onto
// the CompensationStack stored in the context. If CompensateOnFailure is true
// on the EngineConfig and a later node fails, the engine will call
// stack.Compensate to roll back completed steps in LIFO order.
func (g *Graph) AddCompensableNode(name string, forward NodeFunc, compensate CompensateFunc, opts ...NodeOption) error {
	if compensate == nil {
		return fmt.Errorf("workflow: compensate function cannot be nil for node %q", name)
	}
	// Wrap the forward function to push compensation on success.
	wrapped := func(ctx context.Context, state State) (State, error) {
		newState, err := forward(ctx, state)
		if err != nil {
			return newState, err
		}
		// Push compensation record — the stack is stored in the context.
		stack := compensationStackFromContext(ctx)
		if stack != nil {
			stack.Push(CompensationRecord{
				NodeName:   name,
				Compensate: compensate,
				State:      newState.Clone(),
			})
		}
		return newState, nil
	}
	return g.AddNode(name, wrapped, opts...)
}

// contextKey for compensation stack.
type compensationContextKey struct{}

// withCompensationStack adds a CompensationStack to the context.
func withCompensationStack(ctx context.Context, stack *CompensationStack) context.Context {
	return context.WithValue(ctx, compensationContextKey{}, stack)
}

// compensationStackFromContext retrieves the CompensationStack from context.
func compensationStackFromContext(ctx context.Context) *CompensationStack {
	v, _ := ctx.Value(compensationContextKey{}).(*CompensationStack)
	return v
}
