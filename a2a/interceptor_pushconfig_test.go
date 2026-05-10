package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Tests in this file exercise the push-notification-config delegation paths
// on both RateLimitedClient and AuthenticatedClient that staticcheck coverage
// flagged at 0% as part of v0.6.0's coverage push.

// pushConfigServer returns an httptest server that responds to every push-
// config method with a canned successful JSON-RPC envelope. The body is
// intentionally generic; tests verify the wrapper delegates correctly, not
// the exact server logic.
func pushConfigServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "CreateTaskPushNotificationConfig",
			"GetTaskPushNotificationConfig":
			cfg := PushNotificationConfig{TaskID: "t1", ID: "cfg-1", URL: "https://example.com/webhook"}
			cfgJSON, _ := json.Marshal(cfg)
			_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: cfgJSON})
		case "ListTaskPushNotificationConfigs":
			listResp := ListTaskPushNotificationConfigsResponse{
				Configs: []PushNotificationConfig{{TaskID: "t1", ID: "cfg-1"}},
			}
			respJSON, _ := json.Marshal(listResp)
			_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: respJSON})
		case "DeleteTaskPushNotificationConfig":
			_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)})
		case "tasks/cancel":
			task := Task{ID: "t1", State: TaskCanceled}
			taskJSON, _ := json.Marshal(task)
			_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: taskJSON})
		default:
			http.Error(w, "unknown method", http.StatusNotImplemented)
		}
	}))
}

func TestRateLimitedClient_PushConfigDelegation(t *testing.T) {
	t.Parallel()

	ts := pushConfigServer()
	defer ts.Close()

	inner := NewClient(ts.URL)
	limiter := NewRateLimitInterceptor(RateLimitConfig{DefaultRate: 100.0, DefaultBurst: 10})
	rc := NewRateLimitedClient(inner, limiter)
	ctx := context.Background()

	t.Run("CancelTask", func(t *testing.T) {
		task, err := rc.CancelTask(ctx, "t1")
		if err != nil {
			t.Fatalf("CancelTask: %v", err)
		}
		if task.State != TaskCanceled {
			t.Errorf("State = %q, want canceled", task.State)
		}
	})

	t.Run("CreateTaskPushNotificationConfig", func(t *testing.T) {
		cfg, err := rc.CreateTaskPushNotificationConfig(ctx, PushNotificationConfig{TaskID: "t1", URL: "https://example.com/hook"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if cfg.ID == "" {
			t.Error("expected non-empty config ID in response")
		}
	})

	t.Run("GetTaskPushNotificationConfig", func(t *testing.T) {
		cfg, err := rc.GetTaskPushNotificationConfig(ctx, "t1", "cfg-1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if cfg.ID == "" {
			t.Error("expected non-empty config ID")
		}
	})

	t.Run("ListTaskPushNotificationConfigs", func(t *testing.T) {
		resp, err := rc.ListTaskPushNotificationConfigs(ctx, "t1")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(resp.Configs) == 0 {
			t.Error("expected at least one config in list")
		}
	})

	t.Run("DeleteTaskPushNotificationConfig", func(t *testing.T) {
		if err := rc.DeleteTaskPushNotificationConfig(ctx, "t1", "cfg-1"); err != nil {
			t.Errorf("Delete: %v", err)
		}
	})
}

func TestRateLimitedClient_PushConfigRateLimited(t *testing.T) {
	t.Parallel()

	ts := pushConfigServer()
	defer ts.Close()

	inner := NewClient(ts.URL)
	// Very tight rate limit: 0.1/sec means one token every 10s, so back-to-
	// back calls within microseconds must hit the limiter on the SECOND call.
	limiter := NewRateLimitInterceptor(RateLimitConfig{DefaultRate: 0.1, DefaultBurst: 1})
	rc := NewRateLimitedClient(inner, limiter)
	ctx := context.Background()

	// First call burns the burst budget.
	if _, err := rc.CreateTaskPushNotificationConfig(ctx, PushNotificationConfig{TaskID: "t1"}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Second call should be rate-limited (the wrapper checks Allow before
	// delegating to inner).
	if _, err := rc.GetTaskPushNotificationConfig(ctx, "t1", "cfg-1"); err == nil {
		t.Error("expected rate-limit error on second call within burst window")
	}
}

func TestTracingClient_AllMethods(t *testing.T) {
	t.Parallel()

	ts := pushConfigServer()
	defer ts.Close()

	// Dedicated server for the GetAgentCard test path.
	cardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent.json" {
			card := AgentCard{
				Name:        "test-agent",
				Description: "fixture for tracing tests",
				URL:         "http://example.com",
				Version:     "0.1.0",
			}
			_ = json.NewEncoder(w).Encode(card)
			return
		}
		http.NotFound(w, r)
	}))
	defer cardServer.Close()

	// Tests exercise each TracingClient method end-to-end (uses default
	// no-op tracer when OTel isn't configured, but the wrapper code paths
	// still execute and count toward coverage).
	t.Run("delegation_paths", func(t *testing.T) {
		inner := NewClient(ts.URL)
		tc := NewTracingClient(inner)
		ctx := context.Background()

		// SendTask
		if _, err := tc.SendTask(ctx, TaskSendParams{ID: "t1"}); err != nil {
			// SendTask response from pushConfigServer is "unknown method" for
			// "tasks/send" — we don't care about the result, just that the
			// wrapper executed. Allow error.
			_ = err
		}
		// GetTask — same: wrapper executes, server returns "unknown method".
		_, _ = tc.GetTask(ctx, "t1")
		// CancelTask — supported by the server.
		if _, err := tc.CancelTask(ctx, "t1"); err != nil {
			t.Errorf("CancelTask via tracing: %v", err)
		}
		// Push-config methods — all supported.
		if _, err := tc.CreateTaskPushNotificationConfig(ctx, PushNotificationConfig{TaskID: "t1"}); err != nil {
			t.Errorf("Create via tracing: %v", err)
		}
		if _, err := tc.GetTaskPushNotificationConfig(ctx, "t1", "cfg-1"); err != nil {
			t.Errorf("Get via tracing: %v", err)
		}
		if _, err := tc.ListTaskPushNotificationConfigs(ctx, "t1"); err != nil {
			t.Errorf("List via tracing: %v", err)
		}
		if err := tc.DeleteTaskPushNotificationConfig(ctx, "t1", "cfg-1"); err != nil {
			t.Errorf("Delete via tracing: %v", err)
		}
	})

	t.Run("GetAgentCard", func(t *testing.T) {
		inner := NewClient(cardServer.URL)
		tc := NewTracingClient(inner)
		card, err := tc.GetAgentCard(context.Background())
		if err != nil {
			t.Fatalf("GetAgentCard: %v", err)
		}
		if card.Name != "test-agent" {
			t.Errorf("Name = %q, want test-agent", card.Name)
		}
	})
}

func TestAuthenticatedClient_PushConfigDelegation(t *testing.T) {
	t.Parallel()

	ts := pushConfigServer()
	defer ts.Close()

	inner := NewClient(ts.URL)
	interceptor := NewAuthInterceptor()
	interceptor.SetCredential(ts.URL, Credential{Type: CredentialBearer, Value: "tok-12345"})
	ac := NewAuthenticatedClient(inner, interceptor)
	ctx := context.Background()

	t.Run("CancelTask", func(t *testing.T) {
		task, err := ac.CancelTask(ctx, "t1")
		if err != nil {
			t.Fatalf("CancelTask: %v", err)
		}
		if task.State != TaskCanceled {
			t.Errorf("State = %q", task.State)
		}
	})

	t.Run("CreateTaskPushNotificationConfig", func(t *testing.T) {
		cfg, err := ac.CreateTaskPushNotificationConfig(ctx, PushNotificationConfig{TaskID: "t1"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if cfg == nil {
			t.Error("expected non-nil cfg")
		}
	})

	t.Run("GetTaskPushNotificationConfig", func(t *testing.T) {
		cfg, err := ac.GetTaskPushNotificationConfig(ctx, "t1", "cfg-1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if cfg == nil {
			t.Error("expected non-nil cfg")
		}
	})

	t.Run("ListTaskPushNotificationConfigs", func(t *testing.T) {
		resp, err := ac.ListTaskPushNotificationConfigs(ctx, "t1")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if resp == nil {
			t.Error("expected non-nil response")
		}
	})

	t.Run("DeleteTaskPushNotificationConfig", func(t *testing.T) {
		if err := ac.DeleteTaskPushNotificationConfig(ctx, "t1", "cfg-1"); err != nil {
			t.Errorf("Delete: %v", err)
		}
	})
}
