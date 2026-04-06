package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestNewBridgeTool_FireAndForget(t *testing.T) {
	t.Parallel()

	// Set up a mock A2A server that returns a submitted task.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		task := Task{
			ID:    "t1",
			State: TaskSubmitted,
		}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	td := NewBridgeTool()

	// Invoke the tool via its handler (fire-and-forget, no wait).
	req := registry.CallToolRequest{}
	req.Params.Name = "a2a_send_task"
	req.Params.Arguments = map[string]any{
		"agent_url": ts.URL,
		"message":   "hello agent",
		"wait":      false,
	}

	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.IsError {
		t.Error("expected non-error result")
	}
}

func TestNewBridgeTool_WaitForCompletion(t *testing.T) {
	t.Parallel()

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		callCount++
		var task Task

		switch req.Method {
		case "tasks/send":
			task = Task{
				ID:    "t1",
				State: TaskWorking,
			}
		case "tasks/get":
			if callCount >= 3 {
				task = Task{
					ID:    "t1",
					State: TaskCompleted,
					Messages: []Message{
						{Role: "agent", Parts: []Part{TextPart("task done")}},
					},
					Artifacts: []Artifact{
						{Name: "result.txt"},
					},
				}
			} else {
				task = Task{
					ID:    "t1",
					State: TaskWorking,
				}
			}
		}

		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	td := NewBridgeTool()

	req := registry.CallToolRequest{}
	req.Params.Name = "a2a_send_task"
	req.Params.Arguments = map[string]any{
		"agent_url": ts.URL,
		"message":   "compute something",
		"wait":      true,
	}

	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.IsError {
		t.Error("expected non-error result")
	}
}

func TestNewBridgeTool_ServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &JSONRPCError{Code: -32600, Message: "bad request"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	td := NewBridgeTool()

	req := registry.CallToolRequest{}
	req.Params.Name = "a2a_send_task"
	req.Params.Arguments = map[string]any{
		"agent_url": ts.URL,
		"message":   "hello",
	}

	// The tool should not return a Go error; it wraps failures in the output.
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestNewBridgeTool_WaitCanceled(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		task := Task{
			ID:    "t1",
			State: TaskWorking,
		}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	td := NewBridgeTool()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := registry.CallToolRequest{}
	req.Params.Name = "a2a_send_task"
	req.Params.Arguments = map[string]any{
		"agent_url": ts.URL,
		"message":   "hello",
		"wait":      true,
	}

	result, err := td.Handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// The output should indicate cancellation.
}

func TestTaskToOutput_NoAgentMessages(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:    "t1",
		State: TaskCompleted,
		Messages: []Message{
			{Role: "user", Parts: []Part{TextPart("hello")}},
		},
	}

	out := taskToOutput(task, "http://example.com")
	if out.Response != "" {
		t.Errorf("expected empty response, got %q", out.Response)
	}
}

func TestTaskToOutput_WithArtifacts(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:    "t1",
		State: TaskCompleted,
		Messages: []Message{
			{Role: "agent", Parts: []Part{TextPart("here is the result")}},
		},
		Artifacts: []Artifact{
			{Name: "output.json"},
			{Name: "report.pdf"},
		},
	}

	out := taskToOutput(task, "http://example.com")
	if out.Response != "here is the result" {
		t.Errorf("Response = %q, want %q", out.Response, "here is the result")
	}
	if len(out.Artifacts) != 2 {
		t.Errorf("Artifacts = %d, want 2", len(out.Artifacts))
	}
}

func TestTask_Snapshot(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:    "t1",
		State: TaskWorking,
		Messages: []Message{
			{Role: "user", Parts: []Part{TextPart("hello")}},
		},
		Artifacts: []Artifact{
			{Name: "a1"},
		},
		Metadata: map[string]string{
			"key": "value",
		},
	}

	snap := task.snapshot()

	// Modify the original to verify the snapshot is independent.
	task.State = TaskCompleted
	task.Messages = append(task.Messages, Message{Role: "agent", Parts: []Part{TextPart("done")}})
	task.Metadata["key2"] = "value2"

	if snap.State != TaskWorking {
		t.Errorf("snapshot state changed: %q", snap.State)
	}
	if len(snap.Messages) != 1 {
		t.Errorf("snapshot messages changed: %d", len(snap.Messages))
	}
	if _, ok := snap.Metadata["key2"]; ok {
		t.Error("snapshot metadata changed")
	}
}

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()

	customClient := &http.Client{Timeout: 5 * time.Second}
	c := NewClient("http://example.com", WithHTTPClient(customClient))
	if c.httpClient != customClient {
		t.Error("expected custom HTTP client")
	}
}

func TestWithPushNotifications(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithPushNotifications())
	if !card.Capabilities.PushNotifications {
		t.Error("expected PushNotifications to be true")
	}
}

func TestServer_HandleMethodNotAllowed(t *testing.T) {
	t.Parallel()

	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("test"))
	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// GET on the JSON-RPC endpoint should return method not allowed.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
