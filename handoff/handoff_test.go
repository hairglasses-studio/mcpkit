package handoff

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
)

// mockDelegate returns a DelegateFunc that responds with the given status.
func mockDelegate(status string) DelegateFunc {
	return func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
		return &HandoffResult{
			AgentName:  agent.Name,
			Status:     status,
			Result:     "done: " + req.TaskDescription,
			Iterations: 3,
		}, nil
	}
}

func TestRegisterAndUnregister(t *testing.T) {
	m := NewHandoffManager()

	if err := m.Register(AgentRef{Name: "alpha", Skills: []string{"search"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	agents := m.ListAgents()
	if len(agents) != 1 || agents[0].Name != "alpha" {
		t.Fatalf("expected 1 agent named alpha, got %v", agents)
	}

	if removed := m.Unregister("alpha"); !removed {
		t.Error("expected Unregister to return true for existing agent")
	}
	if len(m.ListAgents()) != 0 {
		t.Error("expected no agents after unregister")
	}
}

func TestRegisterEmptyName(t *testing.T) {
	m := NewHandoffManager()
	err := m.Register(AgentRef{Name: ""})
	if err == nil {
		t.Fatal("expected error for empty agent name")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	m := NewHandoffManager()
	if err := m.Register(AgentRef{Name: "beta"}); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	err := m.Register(AgentRef{Name: "beta"})
	if err == nil {
		t.Fatal("expected error when registering duplicate agent name")
	}
}

func TestUnregisterNonExistent(t *testing.T) {
	m := NewHandoffManager()
	if m.Unregister("nobody") {
		t.Error("expected false when unregistering a nonexistent agent")
	}
}

func TestListAgentsSorted(t *testing.T) {
	m := NewHandoffManager()
	_ = m.Register(AgentRef{Name: "charlie"})
	_ = m.Register(AgentRef{Name: "alice"})
	_ = m.Register(AgentRef{Name: "bob"})

	agents := m.ListAgents()
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}
	if agents[0].Name != "alice" || agents[1].Name != "bob" || agents[2].Name != "charlie" {
		t.Errorf("agents not sorted: %v", agents)
	}
}

func TestFindAgentBySkill(t *testing.T) {
	m := NewHandoffManager()
	_ = m.Register(AgentRef{Name: "coder", Skills: []string{"go", "python"}})
	_ = m.Register(AgentRef{Name: "writer", Skills: []string{"docs", "markdown"}})

	a, ok := m.FindAgent("python")
	if !ok {
		t.Fatal("expected to find agent with python skill")
	}
	if a.Name != "coder" {
		t.Errorf("expected coder, got %s", a.Name)
	}
}

func TestFindAgentSkillNotFound(t *testing.T) {
	m := NewHandoffManager()
	_ = m.Register(AgentRef{Name: "coder", Skills: []string{"go"}})

	_, ok := m.FindAgent("rust")
	if ok {
		t.Error("expected FindAgent to return false for unknown skill")
	}
}

func TestDelegateSuccess(t *testing.T) {
	m := NewHandoffManager(Config{
		DefaultTimeout:     5 * time.Second,
		MaxDelegationDepth: 5,
		Delegate:           mockDelegate("completed"),
	})
	_ = m.Register(AgentRef{Name: "worker"})

	result, err := m.Delegate(context.Background(), "worker", HandoffRequest{
		TaskDescription: "do the thing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected status completed, got %s", result.Status)
	}
	if result.AgentName != "worker" {
		t.Errorf("expected agent name worker, got %s", result.AgentName)
	}
	if result.Result != "done: do the thing" {
		t.Errorf("unexpected result content: %s", result.Result)
	}
	if result.Duration <= 0 {
		t.Error("expected Duration to be set")
	}
}

func TestDelegateUnknownAgent(t *testing.T) {
	m := NewHandoffManager(Config{
		DefaultTimeout: 5 * time.Second,
		Delegate:       mockDelegate("completed"),
	})

	_, err := m.Delegate(context.Background(), "ghost", HandoffRequest{TaskDescription: "task"})
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestDelegateNoDelegateFunc(t *testing.T) {
	m := NewHandoffManager() // no Delegate configured
	_ = m.Register(AgentRef{Name: "worker"})

	_, err := m.Delegate(context.Background(), "worker", HandoffRequest{TaskDescription: "task"})
	if !errors.Is(err, ErrNoDelegateFunc) {
		t.Errorf("expected ErrNoDelegateFunc, got %v", err)
	}
}

func TestDelegateMaxDepthEnforced(t *testing.T) {
	m := NewHandoffManager(Config{
		DefaultTimeout:     5 * time.Second,
		MaxDelegationDepth: 3,
		Delegate:           mockDelegate("completed"),
	})
	_ = m.Register(AgentRef{Name: "worker"})

	// Simulate context already at max depth.
	ctx := WithDelegationDepth(context.Background(), 3)
	_, err := m.Delegate(ctx, "worker", HandoffRequest{TaskDescription: "task"})
	if !errors.Is(err, ErrDelegationDepth) {
		t.Errorf("expected ErrDelegationDepth, got %v", err)
	}
}

func TestDelegateDepthIncrementsInContext(t *testing.T) {
	var capturedDepth int
	m := NewHandoffManager(Config{
		DefaultTimeout:     5 * time.Second,
		MaxDelegationDepth: 5,
		Delegate: func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
			capturedDepth = DelegationDepth(ctx)
			return &HandoffResult{Status: "completed", Result: "ok"}, nil
		},
	})
	_ = m.Register(AgentRef{Name: "worker"})

	ctx := WithDelegationDepth(context.Background(), 1)
	_, err := m.Delegate(ctx, "worker", HandoffRequest{TaskDescription: "task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedDepth != 2 {
		t.Errorf("expected depth 2 inside delegate, got %d", capturedDepth)
	}
}

func TestDelegateCostTracking(t *testing.T) {
	tracker := finops.NewTracker()
	m := NewHandoffManager(Config{
		DefaultTimeout:     5 * time.Second,
		MaxDelegationDepth: 5,
		CostTracker:        tracker,
		Delegate: func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
			return &HandoffResult{
				Status:     "completed",
				Result:     "ok",
				TokensUsed: 100,
			}, nil
		},
	})
	_ = m.Register(AgentRef{Name: "pricey"})

	_, err := m.Delegate(context.Background(), "pricey", HandoffRequest{TaskDescription: "expensive task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker.Total() == 0 {
		t.Error("expected cost tracker to have recorded tokens")
	}
}

func TestDelegationDepthContextHelpers(t *testing.T) {
	ctx := context.Background()
	if DelegationDepth(ctx) != 0 {
		t.Error("expected depth 0 on plain context")
	}

	ctx = WithDelegationDepth(ctx, 4)
	if DelegationDepth(ctx) != 4 {
		t.Errorf("expected depth 4, got %d", DelegationDepth(ctx))
	}

	ctx2 := WithDelegationDepth(ctx, 7)
	if DelegationDepth(ctx2) != 7 {
		t.Errorf("expected depth 7, got %d", DelegationDepth(ctx2))
	}
	// Original context should be unchanged.
	if DelegationDepth(ctx) != 4 {
		t.Errorf("original context depth mutated, got %d", DelegationDepth(ctx))
	}
}

func TestNewHandoffManagerDefaults(t *testing.T) {
	m := NewHandoffManager()
	if m.config.MaxDelegationDepth != 5 {
		t.Errorf("expected default MaxDelegationDepth 5, got %d", m.config.MaxDelegationDepth)
	}
	if m.config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", m.config.DefaultTimeout)
	}
}

func TestNewHandoffManagerZeroValuesGetDefaults(t *testing.T) {
	m := NewHandoffManager(Config{
		MaxDelegationDepth: 0,
		DefaultTimeout:     0,
	})
	if m.config.MaxDelegationDepth != 5 {
		t.Errorf("expected default MaxDelegationDepth 5, got %d", m.config.MaxDelegationDepth)
	}
	if m.config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", m.config.DefaultTimeout)
	}
}
