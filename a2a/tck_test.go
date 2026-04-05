package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// TCK Mandatory Compliance Tests — Core A2A v1.0 protocol compliance.

func TestTCK_Mandatory_AgentCardDiscovery(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg,
		WithName("tck-agent"),
		WithDescription("TCK test agent"),
		WithVersion("1.0.0"),
	)

	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/.well-known/agent.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("agent card status = %d, want 200", resp.StatusCode)
	}

	var discovered AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&discovered); err != nil {
		t.Fatalf("decode agent card: %v", err)
	}
	if discovered.Name != "tck-agent" {
		t.Errorf("Name = %q, want tck-agent", discovered.Name)
	}
}

func TestTCK_Mandatory_SendAndGetTask(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("tck-agent"))
	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	ctx := context.Background()

	task, err := client.SendTask(ctx, TaskSendParams{
		ID:       "tck-task-1",
		Messages: []Message{{Role: "user", Parts: []Part{TextPart("hello")}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "tck-task-1" {
		t.Errorf("task ID = %q, want tck-task-1", task.ID)
	}

	// Wait for terminal state
	for i := 0; i < 20 && !task.State.IsTerminal(); i++ {
		time.Sleep(50 * time.Millisecond)
		task, err = client.GetTask(ctx, "tck-task-1")
		if err != nil {
			t.Fatal(err)
		}
	}

	// With empty registry, task dispatches to no tool → fails.
	// Both completed and failed are valid terminal states.
	if !task.State.IsTerminal() {
		t.Errorf("final state = %q, want terminal", task.State)
	}
}

func TestTCK_Mandatory_TaskNotFound(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("tck-agent"))
	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	_, err := client.GetTask(context.Background(), "nonexistent-task")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestTCK_Mandatory_CancelTask(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("tck-agent"))
	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	ctx := context.Background()

	_, err := client.SendTask(ctx, TaskSendParams{
		ID:       "cancel-1",
		Messages: []Message{{Role: "user", Parts: []Part{TextPart("test")}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	task, err := client.CancelTask(ctx, "cancel-1")
	if err != nil {
		t.Fatal(err)
	}
	if !task.State.IsTerminal() {
		t.Errorf("state = %q, want terminal", task.State)
	}
}

func TestTCK_Mandatory_InvalidMethod(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("tck-agent"))
	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"invalid/method"}`
	resp, err := http.Post(ts.URL, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Error == nil {
		t.Fatal("expected error for invalid method")
	}
	if rpcResp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601 (method not found)", rpcResp.Error.Code)
	}
}

func TestTCK_Mandatory_TaskNotFoundErrorCode(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("tck-agent"))
	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{"id":"nonexistent"}}`
	resp, err := http.Post(ts.URL, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Error == nil {
		t.Fatal("expected error")
	}
	if rpcResp.Error.Code != -32001 {
		t.Errorf("error code = %d, want -32001 (task not found)", rpcResp.Error.Code)
	}
}

func TestTCK_Mandatory_InvalidParams(t *testing.T) {
	t.Parallel()
	reg := registry.NewToolRegistry()
	card := AgentCardFromRegistry(reg, WithName("tck-agent"))
	srv := NewServer(reg, card)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Send task with invalid params (missing required fields)
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":"not-an-object"}`
	resp, err := http.Post(ts.URL, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if rpcResp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602 (invalid params)", rpcResp.Error.Code)
	}
}
