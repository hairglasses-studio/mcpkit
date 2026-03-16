package rdcycle

import (
	"context"
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// perpetualStartInput is the input for the rdcycle_perpetual_start tool.
type perpetualStartInput struct {
	MaxCycles      int     `json:"max_cycles,omitempty" jsonschema:"description=Maximum cycles to run (0 = unlimited)"`
	AlarmPerCycle  float64 `json:"alarm_per_cycle,omitempty" jsonschema:"description=Max dollar cost per cycle before alarm (default: 5.0)"`
	ImproveCadence int     `json:"improve_cadence,omitempty" jsonschema:"description=Run meta-improvement every N cycles (default: 5)"`
}

// perpetualStartOutput is the output of the rdcycle_perpetual_start tool.
type perpetualStartOutput struct {
	Started bool   `json:"started"`
	Message string `json:"message"`
}

// perpetualStopInput is the input for the rdcycle_perpetual_stop tool.
type perpetualStopInput struct{}

// perpetualStopOutput is the output of the rdcycle_perpetual_stop tool.
type perpetualStopOutput struct {
	Stopped bool   `json:"stopped"`
	Message string `json:"message"`
}

// perpetualStatusInput is the input for the rdcycle_perpetual_status tool.
type perpetualStatusInput struct{}

// perpetualStatusOutput is the output of the rdcycle_perpetual_status tool.
type perpetualStatusOutput struct {
	Running      bool    `json:"running"`
	CycleNum     int     `json:"cycle_num"`
	BreakerState string  `json:"breaker_state"`
	TotalCost    float64 `json:"total_cost"`
	Message      string  `json:"message"`
}

// orchestratorState holds the running orchestrator and its cancel function.
type orchestratorState struct {
	orchestrator *Orchestrator
	cancel       context.CancelFunc
	breaker      *CircuitBreaker
	governor     *CostVelocityGovernor
}

func (m *Module) perpetualStartTool() registry.ToolDefinition {
	td := handler.TypedHandler[perpetualStartInput, perpetualStartOutput](
		"rdcycle_perpetual_start",
		"Start the perpetual R&D cycle orchestrator. It will autonomously scan, plan, "+
			"synthesize specs, run ralph loops, record notes, and repeat. "+
			"Uses circuit breaker and cost velocity governor for safety. "+
			"Requires a RalphStarter to be wired (see SetRalphStarter).",
		m.handlePerpetualStart,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	td.IsWrite = true
	return td
}

func (m *Module) perpetualStopTool() registry.ToolDefinition {
	td := handler.TypedHandler[perpetualStopInput, perpetualStopOutput](
		"rdcycle_perpetual_stop",
		"Stop the perpetual R&D cycle orchestrator gracefully after the current cycle completes.",
		m.handlePerpetualStop,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	return td
}

func (m *Module) perpetualStatusTool() registry.ToolDefinition {
	td := handler.TypedHandler[perpetualStatusInput, perpetualStatusOutput](
		"rdcycle_perpetual_status",
		"Get the status of the perpetual R&D cycle orchestrator: cycle count, breaker state, and total cost.",
		m.handlePerpetualStatus,
	)
	td.Category = "rdcycle"
	td.Timeout = 30 * time.Second
	td.Complexity = registry.ComplexitySimple
	return td
}

func (m *Module) handlePerpetualStart(_ context.Context, input perpetualStartInput) (perpetualStartOutput, error) {
	m.orchMu.Lock()
	defer m.orchMu.Unlock()

	if m.orchState != nil && m.orchState.orchestrator.Running() {
		return perpetualStartOutput{
			Started: false,
			Message: "perpetual orchestrator is already running",
		}, nil
	}

	if m.ralphStarter == nil {
		return perpetualStartOutput{}, fmt.Errorf("no RalphStarter configured; wire one via SetRalphStarter before starting")
	}

	alarmPerCycle := input.AlarmPerCycle
	if alarmPerCycle <= 0 {
		alarmPerCycle = 5.0
	}

	breaker := NewCircuitBreaker(3, 30*time.Minute)
	governor := NewCostVelocityGovernor(5, alarmPerCycle, 3)

	orch := NewOrchestrator(m, OrchestratorConfig{
		ArtifactStore:  m.store,
		Breaker:        breaker,
		Governor:       governor,
		MaxCycles:      input.MaxCycles,
		ImproveCadence: input.ImproveCadence,
		RalphStarter:   m.ralphStarter,
	})

	ctx, cancel := context.WithCancel(context.Background())
	m.orchState = &orchestratorState{
		orchestrator: orch,
		cancel:       cancel,
		breaker:      breaker,
		governor:     governor,
	}

	go func() {
		_ = orch.Run(ctx)
	}()

	return perpetualStartOutput{
		Started: true,
		Message: fmt.Sprintf("perpetual orchestrator started (max_cycles=%d, alarm=$%.2f)", input.MaxCycles, alarmPerCycle),
	}, nil
}

func (m *Module) handlePerpetualStop(_ context.Context, _ perpetualStopInput) (perpetualStopOutput, error) {
	m.orchMu.Lock()
	defer m.orchMu.Unlock()

	if m.orchState == nil || !m.orchState.orchestrator.Running() {
		return perpetualStopOutput{
			Stopped: false,
			Message: "no perpetual orchestrator is running",
		}, nil
	}

	m.orchState.orchestrator.Stop()
	m.orchState.cancel()

	return perpetualStopOutput{
		Stopped: true,
		Message: "stop signal sent; orchestrator will finish current cycle and halt",
	}, nil
}

func (m *Module) handlePerpetualStatus(_ context.Context, _ perpetualStatusInput) (perpetualStatusOutput, error) {
	m.orchMu.Lock()
	defer m.orchMu.Unlock()

	if m.orchState == nil {
		return perpetualStatusOutput{
			Running: false,
			Message: "no orchestrator has been started",
		}, nil
	}

	orch := m.orchState.orchestrator
	breaker := m.orchState.breaker
	governor := m.orchState.governor

	breakerState := "n/a"
	if breaker != nil {
		breakerState = breaker.State().String()
	}

	totalCost := 0.0
	if governor != nil {
		totalCost = governor.TotalCost()
	}

	return perpetualStatusOutput{
		Running:      orch.Running(),
		CycleNum:     orch.CycleNum(),
		BreakerState: breakerState,
		TotalCost:    totalCost,
		Message:      fmt.Sprintf("cycle %d, breaker=%s, cost=$%.2f", orch.CycleNum(), breakerState, totalCost),
	}, nil
}
