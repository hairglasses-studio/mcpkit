package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestNewBridge_Defaults(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(reg, BridgeConfig{
		Name: "test-bridge",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	if b.config.Version != "1.0.0" {
		t.Errorf("expected default version %q, got %q", "1.0.0", b.config.Version)
	}
	if b.config.Addr != DefaultAddr {
		t.Errorf("expected default addr %q, got %q", DefaultAddr, b.config.Addr)
	}
	if b.config.Timeout != DefaultTaskTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultTaskTimeout, b.config.Timeout)
	}
	if b.translator == nil {
		t.Error("expected translator to be initialized")
	}
	if b.cardGen == nil {
		t.Error("expected card generator to be initialized")
	}
	if b.executor == nil {
		t.Error("expected executor to be initialized")
	}
}

func TestNewBridge_NilRegistry(t *testing.T) {
	_, err := NewBridge(nil, BridgeConfig{})
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestNewBridge_CustomConfig(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(reg, BridgeConfig{
		Name:        "custom",
		Description: "custom bridge",
		Version:     "2.0.0",
		Addr:        ":9090",
		Timeout:     60 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	if b.config.Version != "2.0.0" {
		t.Errorf("expected version %q, got %q", "2.0.0", b.config.Version)
	}
	if b.config.Addr != ":9090" {
		t.Errorf("expected addr %q, got %q", ":9090", b.config.Addr)
	}
	if b.config.Timeout != 60*time.Second {
		t.Errorf("expected timeout %v, got %v", 60*time.Second, b.config.Timeout)
	}
}

func TestBridge_AgentCard(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "tools",
		tools: []registry.ToolDefinition{
			{
				Tool:    registry.Tool{Name: "alpha", Description: "Alpha tool"},
				Handler: noopHandler,
			},
			{
				Tool:    registry.Tool{Name: "beta", Description: "Beta tool"},
				Handler: noopHandler,
			},
		},
	})

	b, err := NewBridge(reg, BridgeConfig{
		Name:        "skill-bridge",
		Description: "test bridge with skills",
		Version:     "1.0.0",
		URL:         "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	card := b.AgentCard()
	if card.Name != "skill-bridge" {
		t.Errorf("expected name %q, got %q", "skill-bridge", card.Name)
	}
	if card.Description != "test bridge with skills" {
		t.Errorf("expected description %q, got %q", "test bridge with skills", card.Description)
	}
	if card.Version != "1.0.0" {
		t.Errorf("expected version %q, got %q", "1.0.0", card.Version)
	}

	if len(card.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(card.Skills))
	}

	// Skills are sorted by ID.
	if card.Skills[0].ID != "alpha" {
		t.Errorf("expected first skill %q, got %q", "alpha", card.Skills[0].ID)
	}
	if card.Skills[1].ID != "beta" {
		t.Errorf("expected second skill %q, got %q", "beta", card.Skills[1].ID)
	}
}

func TestBridge_Handler_NotNil(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(reg, BridgeConfig{Name: "handler-test"})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	if b.Handler() == nil {
		t.Fatal("expected Handler() to return non-nil handler")
	}
}

func TestBridge_WellKnownAgentCard(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "demo",
		tools: []registry.ToolDefinition{
			{
				Tool:    registry.Tool{Name: "ping", Description: "Ping tool"},
				Handler: noopHandler,
			},
		},
	})

	b, err := NewBridge(reg, BridgeConfig{
		Name:    "card-endpoint",
		Version: "1.2.3",
		URL:     "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	ts := httptest.NewServer(b.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + a2asrv.WellKnownAgentCardPath)
	if err != nil {
		t.Fatalf("GET agent card: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var card a2atypes.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("decode agent card: %v", err)
	}

	if card.Name != "card-endpoint" {
		t.Errorf("expected name %q, got %q", "card-endpoint", card.Name)
	}
	if card.Version != "1.2.3" {
		t.Errorf("expected version %q, got %q", "1.2.3", card.Version)
	}
	if len(card.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(card.Skills))
	}
	if card.Skills[0].ID != "ping" {
		t.Errorf("expected skill ID %q, got %q", "ping", card.Skills[0].ID)
	}
}

func TestBridge_JSONRPC_SendMessage(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "echo",
		tools: []registry.ToolDefinition{
			{
				Tool:    registry.Tool{Name: "greet", Description: "Greet someone"},
				Handler: greetHandler,
			},
		},
	})

	b, err := NewBridge(reg, BridgeConfig{
		Name: "jsonrpc-test",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	ts := httptest.NewServer(b.Handler())
	defer ts.Close()

	// Build a JSON-RPC SendMessage request.
	msg := a2atypes.NewMessage(
		a2atypes.MessageRoleUser,
		a2atypes.NewDataPart(map[string]any{
			"skill":     "greet",
			"arguments": map[string]any{"name": "world"},
		}),
	)

	jsonrpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "SendMessage",
		"params": map[string]any{
			"message": msg,
		},
	}

	body, err := json.Marshal(jsonrpcReq)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	resp, err := http.Post(ts.URL+"/", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var jsonrpcResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jsonrpcResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if jsonrpcResp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %d %s", jsonrpcResp.Error.Code, jsonrpcResp.Error.Message)
	}

	if jsonrpcResp.Result == nil {
		t.Fatal("expected non-nil result")
	}

	// The a2a-go server returns the result as a StreamResponse wrapping a Task.
	// For non-streaming SendMessage, the response is {"task": {...}}.
	var resultMap map[string]json.RawMessage
	if err := json.Unmarshal(jsonrpcResp.Result, &resultMap); err != nil {
		t.Fatalf("decode result map: %v (raw: %s)", err, string(jsonrpcResp.Result))
	}

	taskRaw, ok := resultMap["task"]
	if !ok {
		t.Fatalf("no 'task' field in result: %s", string(jsonrpcResp.Result))
	}

	var task a2atypes.Task
	if err := json.Unmarshal(taskRaw, &task); err != nil {
		t.Fatalf("decode task: %v (raw: %s)", err, string(taskRaw))
	}

	if task.Status.State != a2atypes.TaskStateCompleted {
		t.Errorf("expected completed state, got %s", task.Status.State)
	}

	// Check artifacts contain the greet result.
	if len(task.Artifacts) == 0 {
		t.Fatal("expected at least one artifact")
	}

	found := false
	for _, art := range task.Artifacts {
		for _, part := range art.Parts {
			if part.Text() == "hello world" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected artifact containing 'hello world'")
	}
}

func TestBridge_Stop_NotStarted(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(reg, BridgeConfig{Name: "stop-test"})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	// Stop on a bridge that was never started should not error.
	if err := b.Stop(context.Background()); err != nil {
		t.Errorf("expected nil error from Stop on unstarted bridge, got: %v", err)
	}
}

func TestBridge_StartStop(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(reg, BridgeConfig{
		Name: "lifecycle-test",
		Addr: "127.0.0.1:0", // let OS assign port
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Start(ctx)
	}()

	// Give the server a moment to bind.
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to trigger shutdown.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil from Start after shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestBridge_DoubleStart(t *testing.T) {
	reg := registry.NewToolRegistry()
	b, err := NewBridge(reg, BridgeConfig{
		Name: "double-start",
		Addr: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Start(ctx)
	}()

	// Give the first Start time to set the server.
	time.Sleep(50 * time.Millisecond)

	// Second Start should fail immediately.
	err = b.Start(ctx)
	if err == nil {
		t.Fatal("expected error from second Start call")
	}

	cancel()
	<-errCh
}
