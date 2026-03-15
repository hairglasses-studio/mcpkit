package workflow

// Hooks provides optional callbacks for workflow lifecycle events.
type Hooks struct {
	OnNodeStart     func(name string, state State)
	OnNodeEnd       func(name string, state State)
	OnNodeError     func(name string, err error)
	OnCheckpoint    func(cp Checkpoint)
	OnCycleDetected func(name string, step int)
}

func (h *Hooks) callNodeStart(name string, state State) {
	if h.OnNodeStart != nil {
		h.OnNodeStart(name, state)
	}
}

func (h *Hooks) callNodeEnd(name string, state State) {
	if h.OnNodeEnd != nil {
		h.OnNodeEnd(name, state)
	}
}

func (h *Hooks) callNodeError(name string, err error) {
	if h.OnNodeError != nil {
		h.OnNodeError(name, err)
	}
}

func (h *Hooks) callCheckpoint(cp Checkpoint) {
	if h.OnCheckpoint != nil {
		h.OnCheckpoint(cp)
	}
}

func (h *Hooks) callCycleDetected(name string, step int) {
	if h.OnCycleDetected != nil {
		h.OnCycleDetected(name, step)
	}
}
