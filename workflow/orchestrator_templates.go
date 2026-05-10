package workflow

import (
	"context"
	"fmt"
	"maps"

	"github.com/hairglasses-studio/mcpkit/orchestrator"
)

const (
	// OrchestratorResultKey stores an *orchestrator.OrchestratorResult.
	OrchestratorResultKey = "orchestrator_result"
	// OrchestratorPatternKey stores the pattern executed by an orchestrator template node.
	OrchestratorPatternKey = "orchestrator_pattern"
	// DynamicPatternDecisionKey stores an orchestrator.DynamicPatternDecision.
	DynamicPatternDecisionKey = "orchestrator_dynamic_decision"
	// SwarmResultKey stores an *orchestrator.SwarmResult.
	SwarmResultKey = "orchestrator_swarm_result"
	// DelegationResultKey stores an *orchestrator.DelegationResult.
	DelegationResultKey = "orchestrator_delegation_result"
)

// FanOutNodeFunc creates a workflow node that runs stages with orchestrator.FanOut.
func FanOutNodeFunc(stages []orchestrator.StageFunc, config ...orchestrator.FanOutConfig) NodeFunc {
	stageCopy := append([]orchestrator.StageFunc(nil), stages...)
	configCopy := append([]orchestrator.FanOutConfig(nil), config...)
	return func(ctx context.Context, state State) (State, error) {
		if err := validateOrchestratorStages(stageCopy); err != nil {
			return state, err
		}
		result, err := orchestrator.FanOut(ctx, stageCopy, stageInputFromState(state), configCopy...)
		return stateWithOrchestratorResult(state, "fan_out", result), err
	}
}

// PipelineNodeFunc creates a workflow node that runs stages with orchestrator.Pipeline.
func PipelineNodeFunc(stages []orchestrator.StageFunc, config ...orchestrator.PipelineConfig) NodeFunc {
	stageCopy := append([]orchestrator.StageFunc(nil), stages...)
	configCopy := append([]orchestrator.PipelineConfig(nil), config...)
	return func(ctx context.Context, state State) (State, error) {
		if err := validateOrchestratorStages(stageCopy); err != nil {
			return state, err
		}
		result, err := orchestrator.Pipeline(ctx, stageCopy, stageInputFromState(state), configCopy...)
		return stateWithOrchestratorResult(state, "pipeline", result), err
	}
}

// SelectNodeFunc creates a workflow node that runs stages with orchestrator.Select.
func SelectNodeFunc(stages []orchestrator.StageFunc, config orchestrator.SelectorConfig) NodeFunc {
	stageCopy := append([]orchestrator.StageFunc(nil), stages...)
	return func(ctx context.Context, state State) (State, error) {
		if err := validateOrchestratorStages(stageCopy); err != nil {
			return state, err
		}
		result, err := orchestrator.Select(ctx, stageCopy, stageInputFromState(state), config)
		return stateWithOrchestratorResult(state, "select", result), err
	}
}

// DynamicOrchestrationNodeFunc creates a workflow node that selects fan-out,
// pipeline, or select from state metadata using orchestrator.RunDynamicPattern.
func DynamicOrchestrationNodeFunc(stages []orchestrator.StageFunc, config orchestrator.DynamicPatternConfig) NodeFunc {
	stageCopy := append([]orchestrator.StageFunc(nil), stages...)
	return func(ctx context.Context, state State) (State, error) {
		if err := validateOrchestratorStages(stageCopy); err != nil {
			return state, err
		}
		result, decision, err := orchestrator.RunDynamicPattern(ctx, stageCopy, stageInputFromState(state), config)
		out := stateWithOrchestratorResult(state, "dynamic_orchestration", result)
		out.Data[DynamicPatternDecisionKey] = decision
		return out, err
	}
}

// SwarmBroadcastNodeFunc creates a workflow node that broadcasts current state
// to every peer in a swarm mesh.
func SwarmBroadcastNodeFunc(peers []orchestrator.SwarmPeer, topic string, config ...orchestrator.SwarmConfig) NodeFunc {
	peerCopy := append([]orchestrator.SwarmPeer(nil), peers...)
	configCopy := append([]orchestrator.SwarmConfig(nil), config...)
	if topic == "" {
		topic = "workflow"
	}
	swarm, swarmErr := orchestrator.NewSwarm(peerCopy...)
	return func(ctx context.Context, state State) (State, error) {
		if swarmErr != nil {
			return state, swarmErr
		}
		result, err := swarm.Broadcast(ctx, orchestrator.SwarmMessage{
			From:     "workflow",
			Topic:    topic,
			Data:     state.Data,
			Metadata: state.Metadata,
		}, configCopy...)
		return stateWithSwarmResult(state, "swarm_broadcast", result), err
	}
}

// HierarchicalDelegationNodeFunc creates a workflow node that runs a manager /
// worker tree with orchestrator.HierarchicalDelegation.
func HierarchicalDelegationNodeFunc(root orchestrator.DelegationNode, config ...orchestrator.DelegationConfig) NodeFunc {
	configCopy := append([]orchestrator.DelegationConfig(nil), config...)
	return func(ctx context.Context, state State) (State, error) {
		result, err := orchestrator.HierarchicalDelegation(ctx, root, stageInputFromState(state), configCopy...)
		return stateWithDelegationResult(state, "hierarchical_delegation", result), err
	}
}

// FanOutTemplate creates a single-node graph for parallel agent execution.
func FanOutTemplate(stages []orchestrator.StageFunc, config ...orchestrator.FanOutConfig) *Graph {
	return singleNodeTemplate("fan_out", FanOutNodeFunc(stages, config...))
}

// PipelineTemplate creates a single-node graph for sequential agent execution.
func PipelineTemplate(stages []orchestrator.StageFunc, config ...orchestrator.PipelineConfig) *Graph {
	return singleNodeTemplate("pipeline", PipelineNodeFunc(stages, config...))
}

// SelectTemplate creates a single-node graph for best-of-N agent execution.
func SelectTemplate(stages []orchestrator.StageFunc, config orchestrator.SelectorConfig) *Graph {
	return singleNodeTemplate("select", SelectNodeFunc(stages, config))
}

// DynamicOrchestrationTemplate creates a single-node graph that chooses the
// orchestration pattern at runtime from state metadata.
func DynamicOrchestrationTemplate(stages []orchestrator.StageFunc, config orchestrator.DynamicPatternConfig) *Graph {
	return singleNodeTemplate("dynamic_orchestration", DynamicOrchestrationNodeFunc(stages, config))
}

// SwarmBroadcastTemplate creates a single-node graph for peer-to-peer swarm fan-out.
func SwarmBroadcastTemplate(peers []orchestrator.SwarmPeer, topic string, config ...orchestrator.SwarmConfig) *Graph {
	return singleNodeTemplate("swarm_broadcast", SwarmBroadcastNodeFunc(peers, topic, config...))
}

// HierarchicalDelegationTemplate creates a single-node graph for manager /
// worker delegation trees.
func HierarchicalDelegationTemplate(root orchestrator.DelegationNode, config ...orchestrator.DelegationConfig) *Graph {
	return singleNodeTemplate("hierarchical_delegation", HierarchicalDelegationNodeFunc(root, config...))
}

func singleNodeTemplate(name string, fn NodeFunc) *Graph {
	g := NewGraph()
	_ = g.AddNode(name, fn)
	_ = g.AddEdge(name, EndNode)
	_ = g.SetStart(name)
	return g
}

func stageInputFromState(state State) orchestrator.StageInput {
	return orchestrator.StageInput{
		Data:     state.Data,
		Metadata: state.Metadata,
	}
}

func stateWithOrchestratorResult(state State, stageName string, result *orchestrator.OrchestratorResult) State {
	out := state.Clone()
	if result != nil {
		mergeStageOutputs(&out, result.Outputs)
		out.Data[OrchestratorResultKey] = result
		out.Data[OrchestratorPatternKey] = result.Pattern
	}
	return markTemplateStage(out, state, stageName)
}

func stateWithSwarmResult(state State, stageName string, result *orchestrator.SwarmResult) State {
	out := state.Clone()
	if result != nil {
		out.Data[SwarmResultKey] = result
		out.Data[OrchestratorPatternKey] = result.Pattern
	}
	return markTemplateStage(out, state, stageName)
}

func stateWithDelegationResult(state State, stageName string, result *orchestrator.DelegationResult) State {
	out := state.Clone()
	if result != nil {
		if result.Output != nil {
			maps.Copy(out.Data, result.Output.Data)
			maps.Copy(out.Metadata, result.Output.Metadata)
		}
		out.Data[DelegationResultKey] = result
		out.Data[OrchestratorPatternKey] = "hierarchical-delegation"
	}
	return markTemplateStage(out, state, stageName)
}

func mergeStageOutputs(state *State, outputs []*orchestrator.StageOutput) {
	for _, output := range outputs {
		if output == nil {
			continue
		}
		maps.Copy(state.Data, output.Data)
		maps.Copy(state.Metadata, output.Metadata)
	}
}

func markTemplateStage(out State, original State, stageName string) State {
	stages := []string{}
	if existing, ok := Get[[]string](original, "stages_completed"); ok {
		stages = append(stages, existing...)
	}
	stages = append(stages, stageName)
	out.Data["stages_completed"] = stages
	return out
}

func validateOrchestratorStages(stages []orchestrator.StageFunc) error {
	for i, stage := range stages {
		if stage == nil {
			return fmt.Errorf("workflow: orchestrator stage %d cannot be nil", i)
		}
	}
	return nil
}
