package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitInterceptor_BasicAllow(t *testing.T) {
	t.Parallel()

	r := NewRateLimitInterceptor(RateLimitConfig{
		DefaultRate:  10.0,
		DefaultBurst: 3,
	})

	// First 3 requests should succeed (burst).
	for i := 0; i < 3; i++ {
		if err := r.Allow("http://agent.example.com"); err != nil {
			t.Errorf("request %d: unexpected error: %v", i+1, err)
		}
	}

	// Fourth request should be rate limited.
	if err := r.Allow("http://agent.example.com"); err == nil {
		t.Error("expected rate limit error on 4th request")
	}
}

func TestRateLimitInterceptor_DifferentAgents(t *testing.T) {
	t.Parallel()

	r := NewRateLimitInterceptor(RateLimitConfig{
		DefaultRate:  10.0,
		DefaultBurst: 1,
	})

	// Each agent gets its own bucket.
	if err := r.Allow("http://agent-a.example.com"); err != nil {
		t.Errorf("agent-a first request: %v", err)
	}
	if err := r.Allow("http://agent-b.example.com"); err != nil {
		t.Errorf("agent-b first request: %v", err)
	}

	// Both should now be rate limited.
	if err := r.Allow("http://agent-a.example.com"); err == nil {
		t.Error("expected rate limit for agent-a")
	}
	if err := r.Allow("http://agent-b.example.com"); err == nil {
		t.Error("expected rate limit for agent-b")
	}
}

func TestRateLimitInterceptor_SetAgentRate(t *testing.T) {
	t.Parallel()

	r := NewRateLimitInterceptor(RateLimitConfig{
		DefaultRate:  10.0,
		DefaultBurst: 1,
	})

	// Set a custom rate with higher burst for a specific agent.
	r.SetAgentRate("http://fast-agent.example.com", 100.0, 5)

	// Should allow 5 requests.
	for i := 0; i < 5; i++ {
		if err := r.Allow("http://fast-agent.example.com"); err != nil {
			t.Errorf("request %d: unexpected error: %v", i+1, err)
		}
	}
	if err := r.Allow("http://fast-agent.example.com"); err == nil {
		t.Error("expected rate limit on 6th request")
	}
}

func TestRateLimitInterceptor_DefaultsApplied(t *testing.T) {
	t.Parallel()

	// Zero config should use defaults.
	r := NewRateLimitInterceptor(RateLimitConfig{})

	// Default burst is 20, so 20 requests should succeed.
	for i := 0; i < 20; i++ {
		if err := r.Allow("http://default.example.com"); err != nil {
			t.Errorf("request %d: unexpected error: %v", i+1, err)
		}
	}
}

func TestRateLimitedClient_SendTask(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := Task{ID: "t1", State: TaskCompleted}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	inner := NewClient(ts.URL)
	limiter := NewRateLimitInterceptor(RateLimitConfig{
		DefaultRate:  10.0,
		DefaultBurst: 1,
	})
	rc := NewRateLimitedClient(inner, limiter)

	// First request succeeds.
	task, err := rc.SendTask(context.Background(), TaskSendParams{ID: "t1"})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}
	if task.State != TaskCompleted {
		t.Errorf("State = %q, want completed", task.State)
	}

	// Second request should be rate limited.
	_, err = rc.SendTask(context.Background(), TaskSendParams{ID: "t2"})
	if err == nil {
		t.Error("expected rate limit error on second SendTask")
	}
}

func TestRateLimitedClient_GetTask(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := Task{ID: "t1", State: TaskWorking}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	inner := NewClient(ts.URL)
	limiter := NewRateLimitInterceptor(RateLimitConfig{
		DefaultRate:  10.0,
		DefaultBurst: 1,
	})
	rc := NewRateLimitedClient(inner, limiter)

	task, err := rc.GetTask(context.Background(), "t1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.State != TaskWorking {
		t.Errorf("State = %q, want working", task.State)
	}

	// Second request should be rate limited.
	_, err = rc.GetTask(context.Background(), "t1")
	if err == nil {
		t.Error("expected rate limit error on second GetTask")
	}
}
