package workflow

// Hooks provides optional callbacks for workflow lifecycle events.
type Hooks struct {
	OnNodeStart     func(name string, state State)
	OnNodeEnd       func(name string, state State)
	OnNodeError     func(name string, err error)
	OnCheckpoint    func(cp Checkpoint)
	OnCycleDetected func(name string, step int)
	// OnCompensationStart is called before each individual compensation function runs.
	OnCompensationStart func(nodeName string)
	// OnCompensationEnd is called after each individual compensation function runs.
	// err is nil on success.
	OnCompensationEnd func(nodeName string, err error)
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

func (h *Hooks) callCompensationStart(nodeName string) {
	if h.OnCompensationStart != nil {
		h.OnCompensationStart(nodeName)
	}
}

func (h *Hooks) callCompensationEnd(nodeName string, err error) {
	if h.OnCompensationEnd != nil {
		h.OnCompensationEnd(nodeName, err)
	}
}
