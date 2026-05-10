package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestPushNotificationConfigJSONRPCRoundTrip(t *testing.T) {
	t.Parallel()

	srv := newPushTestServer()
	seedTask(srv, "task-1", TaskWorking)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	created, err := client.CreateTaskPushNotificationConfig(context.Background(), PushNotificationConfig{
		TaskID: "task-1",
		URL:    "https://example.com/a2a/push",
		Token:  "session-token",
		Authentication: &AuthenticationInfo{
			Scheme:      "Bearer",
			Credentials: "auth-token",
		},
	})
	if err != nil {
		t.Fatalf("CreateTaskPushNotificationConfig: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated config ID")
	}
	if created.TaskID != "task-1" || created.URL != "https://example.com/a2a/push" {
		t.Fatalf("created config = %#v", created)
	}

	got, err := client.GetTaskPushNotificationConfig(context.Background(), "task-1", created.ID)
	if err != nil {
		t.Fatalf("GetTaskPushNotificationConfig: %v", err)
	}
	if got.Token != "session-token" || got.Authentication.Credentials != "auth-token" {
		t.Fatalf("got config = %#v", got)
	}

	list, err := client.ListTaskPushNotificationConfigs(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("ListTaskPushNotificationConfigs: %v", err)
	}
	if len(list.Configs) != 1 {
		t.Fatalf("configs = %d, want 1", len(list.Configs))
	}

	if err := client.DeleteTaskPushNotificationConfig(context.Background(), "task-1", created.ID); err != nil {
		t.Fatalf("DeleteTaskPushNotificationConfig: %v", err)
	}
	list, err = client.ListTaskPushNotificationConfigs(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("ListTaskPushNotificationConfigs after delete: %v", err)
	}
	if len(list.Configs) != 0 {
		t.Fatalf("configs after delete = %d, want 0", len(list.Configs))
	}
}

func TestPushNotificationConfigRESTEndpoints(t *testing.T) {
	t.Parallel()

	srv := newPushTestServer()
	seedTask(srv, "task-rest", TaskWorking)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	configJSON := []byte(`{"url":"https://example.com/webhook","id":"cfg-rest"}`)
	resp, err := http.Post(ts.URL+"/tasks/task-rest/pushNotificationConfigs", "application/json", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatalf("POST config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/tasks/task-rest/pushNotificationConfigs")
	if err != nil {
		t.Fatalf("GET list: %v", err)
	}
	defer resp.Body.Close()
	var list ListTaskPushNotificationConfigsResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Configs) != 1 || list.Configs[0].ID != "cfg-rest" {
		t.Fatalf("list = %#v, want cfg-rest", list)
	}

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/tasks/task-rest/pushNotificationConfigs/cfg-rest", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", resp.StatusCode)
	}
}

func TestPushNotificationDeliveredOnTaskTransition(t *testing.T) {
	t.Parallel()

	received := make(chan StreamResponse, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("X-A2A-Notification-Token"); got != "session-token" {
			t.Errorf("notification token = %q, want session-token", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer auth-token" {
			t.Errorf("authorization = %q, want Bearer auth-token", got)
		}
		var payload StreamResponse
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		received <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()

	srv := newPushTestServer()
	seedTask(srv, "task-notify", TaskWorking)
	srv.mu.Lock()
	srv.storePushConfigLocked(PushNotificationConfig{
		ID:     "cfg-notify",
		TaskID: "task-notify",
		URL:    webhook.URL,
		Token:  "session-token",
		Authentication: &AuthenticationInfo{
			Scheme:      "Bearer",
			Credentials: "auth-token",
		},
	})
	srv.mu.Unlock()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	task, err := client.CancelTask(context.Background(), "task-notify")
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if task.State != TaskCanceled {
		t.Fatalf("state = %s, want canceled", task.State)
	}

	select {
	case payload := <-received:
		if payload.StatusUpdate == nil {
			t.Fatal("expected statusUpdate payload")
		}
		if payload.StatusUpdate.TaskID != "task-notify" {
			t.Fatalf("taskID = %q, want task-notify", payload.StatusUpdate.TaskID)
		}
		if payload.StatusUpdate.Status.State != TaskCanceled {
			t.Fatalf("state = %s, want canceled", payload.StatusUpdate.Status.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for push notification")
	}

	srv.mu.RLock()
	_, stillStored := srv.pushConfigs["task-notify"]
	srv.mu.RUnlock()
	if stillStored {
		t.Fatal("expected terminal task push configs to be cleared")
	}
}

func TestTaskSendAcceptsPushNotificationConfig(t *testing.T) {
	t.Parallel()

	received := make(chan StreamResponse, 2)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload StreamResponse
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		received <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()

	srv := newPushTestServer()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	task, err := client.SendTask(context.Background(), TaskSendParams{
		ID:       "send-push",
		Messages: []Message{{Role: "user", Parts: []Part{TextPart("notify me")}}},
		PushNotification: &PushNotificationConfig{
			URL: webhook.URL,
		},
	})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}
	if task.ID != "send-push" {
		t.Fatalf("task ID = %q, want send-push", task.ID)
	}

	select {
	case payload := <-received:
		if payload.StatusUpdate == nil {
			t.Fatal("expected statusUpdate payload")
		}
		if payload.StatusUpdate.TaskID != "send-push" {
			t.Fatalf("taskID = %q, want send-push", payload.StatusUpdate.TaskID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for send-time push notification")
	}
}

func TestPushNotificationUnsupportedReturnsError(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("no-push"))
	srv := NewServer(reg, card)
	seedTask(srv, "task-unsupported", TaskWorking)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	_, err := client.CreateTaskPushNotificationConfig(context.Background(), PushNotificationConfig{
		TaskID: "task-unsupported",
		URL:    "https://example.com/push",
	})
	if err == nil {
		t.Fatal("expected unsupported push error")
	}
	if !strings.Contains(err.Error(), "-32003") {
		t.Fatalf("error = %q, want -32003", err.Error())
	}
}

func newPushTestServer() *Server {
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("push-agent"), WithPushNotifications())
	return NewServer(reg, card)
}

func seedTask(srv *Server, id string, state TaskState) {
	now := time.Now()
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.tasks[id] = &Task{
		ID:      id,
		State:   state,
		Created: now,
		Updated: now,
	}
}
