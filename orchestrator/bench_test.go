package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkFanOut_1Agent(b *testing.B) {
	benchmarkFanOutAgents(b, 1)
}

func BenchmarkFanOut_10Agents(b *testing.B) {
	benchmarkFanOutAgents(b, 10)
}

func BenchmarkFanOut_100Agents(b *testing.B) {
	benchmarkFanOutAgents(b, 100)
}

func BenchmarkSwarmBroadcast_10Peers(b *testing.B) {
	peers := make([]SwarmPeer, 10)
	for i := range peers {
		id := fmt.Sprintf("peer-%d", i)
		peers[i] = SwarmPeer{
			ID: id,
			Handler: func(_ context.Context, msg SwarmMessage) (SwarmMessage, error) {
				return SwarmMessage{Topic: msg.Topic, Data: msg.Data}, nil
			},
		}
	}
	swarm, err := NewSwarm(peers...)
	if err != nil {
		b.Fatalf("NewSwarm: %v", err)
	}
	msg := SwarmMessage{Topic: "bench", Data: map[string]any{"n": 1}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := swarm.Broadcast(context.Background(), msg); err != nil {
			b.Fatalf("Broadcast: %v", err)
		}
	}
}

func BenchmarkHierarchicalDelegation_10Workers(b *testing.B) {
	children := make([]DelegationNode, 10)
	for i := range children {
		children[i] = DelegationNode{
			ID: fmt.Sprintf("worker-%d", i),
			Stage: func(context.Context, StageInput) (*StageOutput, error) {
				return &StageOutput{Status: "ok", Data: map[string]any{"done": true}}, nil
			},
		}
	}
	root := DelegationNode{
		ID: "manager",
		Stage: func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok", Data: map[string]any{"plan": "bench"}}, nil
		},
		Children: children,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := HierarchicalDelegation(context.Background(), root, StageInput{}); err != nil {
			b.Fatalf("HierarchicalDelegation: %v", err)
		}
	}
}

func TestFanOutLatencyBudget_100NoopAgents(t *testing.T) {
	t.Parallel()

	stages := fanOutNoopStages(100)
	start := time.Now()
	result, err := FanOut(context.Background(), stages, StageInput{})
	if err != nil {
		t.Fatalf("FanOut: %v", err)
	}
	if result.StageCount != 100 {
		t.Fatalf("StageCount = %d, want 100", result.StageCount)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("100-agent no-op fan-out took %v, budget 500ms", elapsed)
	}
}

func benchmarkFanOutAgents(b *testing.B, agents int) {
	stages := fanOutNoopStages(agents)
	input := StageInput{Data: map[string]any{"n": 1}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := FanOut(context.Background(), stages, input); err != nil {
			b.Fatalf("FanOut: %v", err)
		}
	}
}

func fanOutNoopStages(count int) []StageFunc {
	stages := make([]StageFunc, count)
	for i := range stages {
		stages[i] = func(context.Context, StageInput) (*StageOutput, error) {
			return &StageOutput{Status: "ok"}, nil
		}
	}
	return stages
}
