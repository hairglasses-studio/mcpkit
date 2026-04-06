package health

import (
	"sync"
)

// GateStatus represents the evaluation status of an autonomy gate.
type GateStatus string

const (
	// GatePass means the gate's current value meets or exceeds the threshold.
	GatePass GateStatus = "PASS"
	// GateFail means the gate's current value does not meet the threshold.
	GateFail GateStatus = "FAIL"
	// GateUnmeasured means the gate has no measurement function configured.
	GateUnmeasured GateStatus = "UNMEASURED"
)

// AutonomyGate represents a measurable gate for L3 agent autonomy.
// Each gate has a name, a description, a threshold for passing, and functions
// to measure the current value and determine if the gate is passing.
//
// The 7 standard L3 gates are defined in [DefaultGates].
type AutonomyGate struct {
	// Name is the unique identifier for the gate (e.g., "json_retry_rate").
	Name string
	// Description is a human-readable explanation of what the gate measures.
	Description string
	// Threshold is the pass threshold (0.0-1.0). The interpretation depends on
	// the gate: for some gates the value must be below the threshold (e.g.,
	// retry rate < 5%), for others it must be above (e.g., completion rate > 95%).
	Threshold float64
	// Measure returns the current value of the gate metric (0.0-1.0).
	// A nil Measure function means the gate is unmeasured.
	Measure func() float64
	// Passing reports whether the gate is currently passing.
	// A nil Passing function means the gate is unmeasured and considered failing.
	Passing func() bool
}

// GateResult is the result of checking a single autonomy gate.
type GateResult struct {
	// Name is the gate's unique identifier.
	Name string `json:"name"`
	// Description is a human-readable explanation of what the gate measures.
	Description string `json:"description"`
	// Value is the current measured value (0.0-1.0). -1 if unmeasured.
	Value float64 `json:"value"`
	// Threshold is the pass/fail threshold.
	Threshold float64 `json:"threshold"`
	// Status is the gate's current status (PASS, FAIL, or UNMEASURED).
	Status GateStatus `json:"status"`
}

// GateRegistry tracks all L3 autonomy gates. It is safe for concurrent use.
type GateRegistry struct {
	gates map[string]*AutonomyGate
	order []string // preserves registration order
	mu    sync.RWMutex
}

// NewGateRegistry creates a new empty gate registry.
func NewGateRegistry() *GateRegistry {
	return &GateRegistry{
		gates: make(map[string]*AutonomyGate),
	}
}

// Register adds an autonomy gate to the registry. If a gate with the same
// name already exists, it is replaced.
func (r *GateRegistry) Register(gate AutonomyGate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.gates[gate.Name]; !exists {
		r.order = append(r.order, gate.Name)
	}
	g := gate // copy to avoid caller mutation
	r.gates[gate.Name] = &g
}

// Check returns the current value and passing status for the named gate.
// If the gate does not exist, it returns (-1, false).
// If the gate has no Measure function, it returns (-1, false).
func (r *GateRegistry) Check(name string) (float64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.gates[name]
	if !ok {
		return -1, false
	}
	if g.Measure == nil || g.Passing == nil {
		return -1, false
	}
	return g.Measure(), g.Passing()
}

// CheckAll returns the result of checking every registered gate, keyed by name.
func (r *GateRegistry) CheckAll() map[string]GateResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make(map[string]GateResult, len(r.gates))
	for name, g := range r.gates {
		result := GateResult{
			Name:        g.Name,
			Description: g.Description,
			Threshold:   g.Threshold,
		}
		if g.Measure == nil || g.Passing == nil {
			result.Value = -1
			result.Status = GateUnmeasured
		} else {
			result.Value = g.Measure()
			if g.Passing() {
				result.Status = GatePass
			} else {
				result.Status = GateFail
			}
		}
		results[name] = result
	}
	return results
}

// Score returns an aggregate readiness score from 0.0 to 1.0, representing the
// fraction of registered gates that are currently passing. Unmeasured gates
// count as failing. Returns 0 if no gates are registered.
func (r *GateRegistry) Score() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.gates) == 0 {
		return 0
	}
	passing := 0
	for _, g := range r.gates {
		if g.Passing != nil && g.Passing() {
			passing++
		}
	}
	return float64(passing) / float64(len(r.gates))
}

// Gates returns the names of all registered gates in registration order.
func (r *GateRegistry) Gates() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// DefaultGates returns the 7 standard L3 autonomy gates as defined in
// the L3 gate baseline. All gates are returned with nil Measure and Passing
// functions (unmeasured) -- callers must wire in their own measurement
// implementations.
//
// The 7 gates are:
//  1. json_retry_rate -- JSON parse retry rate (target: < 5%)
//  2. session_completion_rate -- Session completion without manual intervention (target: > 95%)
//  3. cost_per_task -- Average USD cost per completed task (target: < $0.50, normalized to 1.0)
//  4. crash_recovery_success -- Crash recovery without manual intervention (target: > 99%)
//  5. budget_enforcement_compliance -- Budget enforcement triggers per marathon (target: < 3, normalized)
//  6. context_compaction_success -- Context compaction success rate (target: > 90%)
//  7. fleet_health_score -- Composite fleet health score (target: > 80%)
func DefaultGates() []AutonomyGate {
	return []AutonomyGate{
		{
			Name:        "json_retry_rate",
			Description: "JSON parse retry rate: percentage of planner/agent JSON outputs that fail parsing and require retry",
			Threshold:   0.05,
		},
		{
			Name:        "session_completion_rate",
			Description: "Session completion rate: percentage of agent sessions that complete their intended task without manual intervention",
			Threshold:   0.95,
		},
		{
			Name:        "cost_per_task",
			Description: "Cost per task: average USD cost per completed autonomous task, normalized to 0.0-1.0 scale where 1.0 = $0.00 and 0.0 = $1.00+",
			Threshold:   0.50,
		},
		{
			Name:        "crash_recovery_success",
			Description: "Crash recovery success: percentage of agent crashes recovered without manual intervention",
			Threshold:   0.99,
		},
		{
			Name:        "budget_enforcement_compliance",
			Description: "Budget enforcement compliance: fraction of marathons completing within budget (< 3 enforcement triggers)",
			Threshold:   1.0,
		},
		{
			Name:        "context_compaction_success",
			Description: "Context compaction success: success rate of context window compaction events",
			Threshold:   0.90,
		},
		{
			Name:        "fleet_health_score",
			Description: "Fleet health score: composite health score across all fleet components over a 24-hour window",
			Threshold:   0.80,
		},
	}
}
