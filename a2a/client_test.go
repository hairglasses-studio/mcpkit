package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetAgentCard(t *testing.T) {
	t.Parallel()
	card := AgentCard{Name: "test", Version: "1.0", Skills: []Skill{{ID: "s1", Name: "s1"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/agent.json" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(card)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.GetAgentCard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test" {
		t.Errorf("Name = %q, want test", got.Name)
	}
	if len(got.Skills) != 1 {
		t.Errorf("Skills = %d, want 1", len(got.Skills))
	}
}

func TestClient_SendTask(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Method != "tasks/send" {
			t.Errorf("method = %q, want tasks/send", req.Method)
		}
		task := Task{ID: "t1", State: TaskCompleted, Messages: []Message{
			{Role: "agent", Parts: []Part{TextPart("done")}},
		}}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	task, err := c.SendTask(context.Background(), TaskSendParams{
		ID:       "t1",
		Messages: []Message{{Role: "user", Parts: []Part{TextPart("hello")}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "t1" {
		t.Errorf("ID = %q", task.ID)
	}
	if task.State != TaskCompleted {
		t.Errorf("State = %q", task.State)
	}
}

func TestClient_GetTask(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := Task{ID: "t1", State: TaskWorking}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	task, err := c.GetTask(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if task.State != TaskWorking {
		t.Errorf("State = %q, want working", task.State)
	}
}

func TestClient_CancelTask(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := Task{ID: "t1", State: TaskCanceled}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	task, err := c.CancelTask(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if task.State != TaskCanceled {
		t.Errorf("State = %q, want canceled", task.State)
	}
}

func TestClient_AuthHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Auth = %q, want Bearer test-token", auth)
		}
		json.NewEncoder(w).Encode(AgentCard{Name: "auth-test"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAuthToken("test-token"))
	c.GetAgentCard(context.Background())
}

func TestClient_ErrorResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error:   &JSONRPCError{Code: -32600, Message: "invalid request"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.SendTask(context.Background(), TaskSendParams{ID: "t1"})
	if err == nil {
		t.Fatal("expected error")
	}
}
