package handoff

import (
	"context"
	"testing"
)

func TestWrapDelegate_Single(t *testing.T) {
	var log []string

	delegate := func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
		log = append(log, "delegate:"+agent.Name)
		return &HandoffResult{Status: "completed"}, nil
	}

	mw := func(agentName string, next DelegateFunc) DelegateFunc {
		return func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
			log = append(log, "before:"+agentName)
			result, err := next(ctx, agent, req)
			log = append(log, "after:"+agentName)
			return result, err
		}
	}

	wrapped := WrapDelegate(delegate, "agent-x", mw)
	result, err := wrapped(context.Background(), AgentRef{Name: "agent-x"}, HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected 'completed', got %q", result.Status)
	}

	expected := []string{"before:agent-x", "delegate:agent-x", "after:agent-x"}
	if len(log) != len(expected) {
		t.Fatalf("expected %d log entries, got %d: %v", len(expected), len(log), log)
	}
	for i, e := range expected {
		if log[i] != e {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], e)
		}
	}
}

func TestWrapDelegate_MultipleMiddleware(t *testing.T) {
	var log []string

	delegate := func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
		log = append(log, "delegate")
		return &HandoffResult{Status: "completed"}, nil
	}

	makeMW := func(id string) DelegateMiddleware {
		return func(agentName string, next DelegateFunc) DelegateFunc {
			return func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
				log = append(log, id+":before")
				result, err := next(ctx, agent, req)
				log = append(log, id+":after")
				return result, err
			}
		}
	}

	wrapped := WrapDelegate(delegate, "a", makeMW("A"), makeMW("B"))
	_, err := wrapped(context.Background(), AgentRef{Name: "a"}, HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"A:before", "B:before", "delegate", "B:after", "A:after"}
	if len(log) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(log), log)
	}
	for i, e := range expected {
		if log[i] != e {
			t.Fatalf("log[%d] = %q, want %q", i, log[i], e)
		}
	}
}

func TestWithMiddleware_ImmutableConfig(t *testing.T) {
	var called bool
	delegate := func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
		called = true
		return &HandoffResult{Status: "completed"}, nil
	}

	original := Config{
		Delegate: delegate,
	}

	mw := func(agentName string, next DelegateFunc) DelegateFunc {
		return func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
			return next(ctx, agent, req)
		}
	}

	newCfg := original.WithMiddleware(mw)

	// Original should be unchanged — calling it should not go through middleware
	called = false
	original.Delegate(context.Background(), AgentRef{Name: "x"}, HandoffRequest{})
	if !called {
		t.Fatal("original delegate should still work")
	}

	// New config should also work
	called = false
	newCfg.Delegate(context.Background(), AgentRef{Name: "x"}, HandoffRequest{})
	if !called {
		t.Fatal("new config delegate should work")
	}
}

func TestWithMiddleware_NilDelegate(t *testing.T) {
	cfg := Config{}
	newCfg := cfg.WithMiddleware(func(agentName string, next DelegateFunc) DelegateFunc {
		return next
	})
	if newCfg.Delegate != nil {
		t.Fatal("expected nil delegate when original has no delegate")
	}
}

type handoffTenantKey struct{}

func TestWrapDelegate_ContextPropagation(t *testing.T) {
	delegate := func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
		tenant, ok := ctx.Value(handoffTenantKey{}).(string)
		if !ok || tenant != "acme" {
			t.Fatalf("expected tenant 'acme', got %q", tenant)
		}
		return &HandoffResult{Status: "completed"}, nil
	}

	mw := func(agentName string, next DelegateFunc) DelegateFunc {
		return func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
			_, ok := ctx.Value(handoffTenantKey{}).(string)
			if !ok {
				t.Fatal("tenant not in context at middleware")
			}
			return next(ctx, agent, req)
		}
	}

	wrapped := WrapDelegate(delegate, "a", mw)
	ctx := context.WithValue(context.Background(), handoffTenantKey{}, "acme")
	_, err := wrapped(ctx, AgentRef{Name: "a"}, HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWrapDelegate_ErrorPropagation(t *testing.T) {
	delegate := func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
		return nil, context.DeadlineExceeded
	}

	mw := func(agentName string, next DelegateFunc) DelegateFunc {
		return func(ctx context.Context, agent AgentRef, req HandoffRequest) (*HandoffResult, error) {
			return next(ctx, agent, req)
		}
	}

	wrapped := WrapDelegate(delegate, "a", mw)
	_, err := wrapped(context.Background(), AgentRef{Name: "a"}, HandoffRequest{})
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}
