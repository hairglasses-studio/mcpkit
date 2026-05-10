package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/orchestrator"
)

func TestFanOutTemplateRunsStagesAndStoresResult(t *testing.T) {
	stages := []orchestrator.StageFunc{
		func(context.Context, orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"alpha": "ready"}}, nil
		},
		func(context.Context, orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"beta": "ready"}}, nil
		},
	}

	result := runTemplate(t, FanOutTemplate(stages), "fan-out-template", NewState())

	if result.Status != RunStatusCompleted {
		t.Fatalf("Status = %s, want completed: %s", result.Status, result.Error)
	}
	orchestration, ok := Get[*orchestrator.OrchestratorResult](result.FinalState, OrchestratorResultKey)
	if !ok {
		t.Fatal("expected orchestrator result in final state")
	}
	if orchestration.Pattern != "fan-out" || orchestration.StageCount != 2 {
		t.Fatalf("orchestration = %#v, want fan-out with 2 stages", orchestration)
	}
	if got, _ := Get[string](result.FinalState, "alpha"); got != "ready" {
		t.Fatalf("alpha = %q, want ready", got)
	}
	if got, _ := Get[string](result.FinalState, "beta"); got != "ready" {
		t.Fatalf("beta = %q, want ready", got)
	}
	assertStagesCompleted(t, result.FinalState, []string{"fan_out"})
}

func TestPipelineTemplatePassesPreviousOutput(t *testing.T) {
	stages := []orchestrator.StageFunc{
		func(context.Context, orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"plan": "draft"}}, nil
		},
		func(_ context.Context, input orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			if input.Previous == nil {
				t.Fatal("expected previous output")
			}
			if input.Data["plan"] != "draft" {
				t.Fatalf("plan = %v, want draft", input.Data["plan"])
			}
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"reviewed": true}}, nil
		},
	}

	result := runTemplate(t, PipelineTemplate(stages), "pipeline-template", NewState())

	if result.Status != RunStatusCompleted {
		t.Fatalf("Status = %s, want completed: %s", result.Status, result.Error)
	}
	orchestration, ok := Get[*orchestrator.OrchestratorResult](result.FinalState, OrchestratorResultKey)
	if !ok {
		t.Fatal("expected orchestrator result in final state")
	}
	if orchestration.Pattern != "pipeline" || orchestration.StageCount != 2 {
		t.Fatalf("orchestration = %#v, want pipeline with 2 stages", orchestration)
	}
	if got, _ := Get[bool](result.FinalState, "reviewed"); !got {
		t.Fatal("expected reviewed=true in final state")
	}
}

func TestSelectTemplateMarksBestOutput(t *testing.T) {
	stages := []orchestrator.StageFunc{
		func(context.Context, orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"score": 1}}, nil
		},
		func(context.Context, orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"score": 9}}, nil
		},
	}
	config := orchestrator.SelectorConfig{
		Scorer: func(output *orchestrator.StageOutput) float64 {
			score, _ := output.Data["score"].(int)
			return float64(score)
		},
	}

	result := runTemplate(t, SelectTemplate(stages, config), "select-template", NewState())

	if result.Status != RunStatusCompleted {
		t.Fatalf("Status = %s, want completed: %s", result.Status, result.Error)
	}
	orchestration, ok := Get[*orchestrator.OrchestratorResult](result.FinalState, OrchestratorResultKey)
	if !ok {
		t.Fatal("expected orchestrator result in final state")
	}
	if orchestration.Pattern != "select" {
		t.Fatalf("Pattern = %q, want select", orchestration.Pattern)
	}
	if got := orchestration.Outputs[1].Metadata["selected"]; got != "true" {
		t.Fatalf("selected metadata = %q, want true", got)
	}
}

func TestSwarmBroadcastTemplateRoutesState(t *testing.T) {
	peers := []orchestrator.SwarmPeer{
		{
			ID: "researcher",
			Handler: func(_ context.Context, message orchestrator.SwarmMessage) (orchestrator.SwarmMessage, error) {
				if message.Topic != "coordination" {
					t.Fatalf("Topic = %q, want coordination", message.Topic)
				}
				if message.Data["task"] != "map" {
					t.Fatalf("task = %v, want map", message.Data["task"])
				}
				return orchestrator.SwarmMessage{Topic: message.Topic, Data: map[string]any{"role": "researcher"}}, nil
			},
		},
		{
			ID: "writer",
			Handler: func(_ context.Context, message orchestrator.SwarmMessage) (orchestrator.SwarmMessage, error) {
				return orchestrator.SwarmMessage{Topic: message.Topic, Data: map[string]any{"role": "writer"}}, nil
			},
		},
	}
	initial := Set(NewState(), "task", "map")

	result := runTemplate(t, SwarmBroadcastTemplate(peers, "coordination"), "swarm-template", initial)

	if result.Status != RunStatusCompleted {
		t.Fatalf("Status = %s, want completed: %s", result.Status, result.Error)
	}
	swarm, ok := Get[*orchestrator.SwarmResult](result.FinalState, SwarmResultKey)
	if !ok {
		t.Fatal("expected swarm result in final state")
	}
	if swarm.Pattern != "swarm-broadcast" || swarm.PeerCount != 2 || len(swarm.Responses) != 2 {
		t.Fatalf("swarm = %#v, want broadcast with 2 responses", swarm)
	}
	if got, _ := Get[string](result.FinalState, OrchestratorPatternKey); got != "swarm-broadcast" {
		t.Fatalf("orchestrator pattern = %q, want swarm-broadcast", got)
	}
}

func TestHierarchicalDelegationTemplateMergesAggregatedOutput(t *testing.T) {
	root := orchestrator.DelegationNode{
		ID: "manager",
		Stage: func(context.Context, orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"plan": "ready"}}, nil
		},
		Children: []orchestrator.DelegationNode{
			{
				ID: "worker",
				Stage: func(_ context.Context, input orchestrator.StageInput) (*orchestrator.StageOutput, error) {
					if input.Previous == nil {
						t.Fatal("expected manager output as previous")
					}
					if input.Data["plan"] != "ready" {
						t.Fatalf("plan = %v, want ready", input.Data["plan"])
					}
					return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"worker": "done"}}, nil
				},
			},
		},
		Aggregator: func(self *orchestrator.StageOutput, children []*orchestrator.DelegationResult) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{
				Status: "ok",
				Data: map[string]any{
					"plan":    self.Data["plan"],
					"workers": len(children),
				},
			}, nil
		},
	}

	result := runTemplate(t, HierarchicalDelegationTemplate(root), "delegation-template", NewState())

	if result.Status != RunStatusCompleted {
		t.Fatalf("Status = %s, want completed: %s", result.Status, result.Error)
	}
	delegation, ok := Get[*orchestrator.DelegationResult](result.FinalState, DelegationResultKey)
	if !ok {
		t.Fatal("expected delegation result in final state")
	}
	if delegation.ID != "manager" || delegation.ChildCount != 1 {
		t.Fatalf("delegation = %#v, want manager with one child", delegation)
	}
	if got, _ := Get[int](result.FinalState, "workers"); got != 1 {
		t.Fatalf("workers = %d, want 1", got)
	}
}

func TestDynamicOrchestrationTemplateStoresDecision(t *testing.T) {
	stages := []orchestrator.StageFunc{
		func(context.Context, orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"first": true}}, nil
		},
		func(_ context.Context, input orchestrator.StageInput) (*orchestrator.StageOutput, error) {
			if input.Previous == nil {
				t.Fatal("expected pipeline previous output")
			}
			return &orchestrator.StageOutput{Status: "ok", Data: map[string]any{"second": true}}, nil
		},
	}
	initial := NewState()
	initial.Metadata["orchestrator.ordered"] = "true"

	result := runTemplate(t, DynamicOrchestrationTemplate(stages, orchestrator.DynamicPatternConfig{}), "dynamic-template", initial)

	if result.Status != RunStatusCompleted {
		t.Fatalf("Status = %s, want completed: %s", result.Status, result.Error)
	}
	decision, ok := Get[orchestrator.DynamicPatternDecision](result.FinalState, DynamicPatternDecisionKey)
	if !ok {
		t.Fatal("expected dynamic pattern decision in final state")
	}
	if decision.Pattern != orchestrator.DynamicPatternPipeline {
		t.Fatalf("decision = %#v, want pipeline", decision)
	}
	orchestration, _ := Get[*orchestrator.OrchestratorResult](result.FinalState, OrchestratorResultKey)
	if orchestration.Pattern != "pipeline" {
		t.Fatalf("orchestration pattern = %q, want pipeline", orchestration.Pattern)
	}
}

func TestOrchestratorTemplateRejectsNilStage(t *testing.T) {
	result := runTemplate(t, FanOutTemplate([]orchestrator.StageFunc{nil}), "nil-stage-template", NewState())

	if result.Status != RunStatusFailed {
		t.Fatalf("Status = %s, want failed", result.Status)
	}
	if !strings.Contains(result.Error, "orchestrator stage 0 cannot be nil") {
		t.Fatalf("Error = %q, want nil stage validation", result.Error)
	}
}

func runTemplate(t *testing.T, g *Graph, runID string, initial State) *RunResult {
	t.Helper()
	e := newEngine(t, g)
	result, err := e.Run(context.Background(), runID, initial)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result
}

func assertStagesCompleted(t *testing.T, state State, want []string) {
	t.Helper()
	got, ok := Get[[]string](state, "stages_completed")
	if !ok {
		t.Fatal("expected stages_completed in state")
	}
	if len(got) != len(want) {
		t.Fatalf("stages_completed = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stages_completed = %v, want %v", got, want)
		}
	}
}
