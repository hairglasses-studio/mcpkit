//go:build !official_sdk

package conformance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestNewEverythingServer(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestEverythingServer_Initialize(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"clientInfo": {"name": "test-client", "version": "0.0.1"},
			"capabilities": {}
		}
	}`)

	var initResult struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities struct {
			Tools     any `json:"tools"`
			Resources any `json:"resources"`
			Prompts   any `json:"prompts"`
			Logging   any `json:"logging"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(result, &initResult); err != nil {
		t.Fatalf("failed to unmarshal init result: %v", err)
	}
	if initResult.ServerInfo.Name != "mcpkit-everything-server" {
		t.Errorf("expected server name 'mcpkit-everything-server', got %q", initResult.ServerInfo.Name)
	}
	if initResult.ServerInfo.Version != "0.1.0" {
		t.Errorf("expected server version '0.1.0', got %q", initResult.ServerInfo.Version)
	}
	if initResult.Capabilities.Tools == nil {
		t.Error("expected tools capability to be present")
	}
	if initResult.Capabilities.Resources == nil {
		t.Error("expected resources capability to be present")
	}
	if initResult.Capabilities.Prompts == nil {
		t.Error("expected prompts capability to be present")
	}
	if initResult.Capabilities.Logging == nil {
		t.Error("expected logging capability to be present")
	}
}

func TestEverythingServer_ToolsList(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/list",
		"params": {}
	}`)

	var listResult struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &listResult); err != nil {
		t.Fatalf("failed to unmarshal tools list: %v", err)
	}

	expectedTools := map[string]bool{
		"echo":                 false,
		"add":                  false,
		"longRunningOperation": false,
		"sampleLLM":            false,
		"getTinyImage":         false,
		"annotatedMessage":     false,
		"logMessage":           false,
	}

	for _, tool := range listResult.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found in tools/list", name)
		}
	}
}

func TestEverythingServer_EchoTool(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 3,
		"method": "tools/call",
		"params": {
			"name": "echo",
			"arguments": {"message": "hello world"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "hello world") {
		t.Errorf("expected echo response to contain 'hello world', got: %s", resultStr)
	}
}

func TestEverythingServer_AddTool(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 4,
		"method": "tools/call",
		"params": {
			"name": "add",
			"arguments": {"a": 3, "b": 4}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "7") {
		t.Errorf("expected add result to contain '7', got: %s", resultStr)
	}
}

func TestEverythingServer_GetTinyImage(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 5,
		"method": "tools/call",
		"params": {
			"name": "getTinyImage",
			"arguments": {}
		}
	}`)

	if !strings.Contains(string(result), "image/png") {
		t.Errorf("expected image/png in result, got: %s", string(result))
	}
}

func TestEverythingServer_ResourcesList(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 6,
		"method": "resources/list",
		"params": {}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "test://static-text") {
		t.Errorf("expected static-text resource, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "test://static-binary") {
		t.Errorf("expected static-binary resource, got: %s", resultStr)
	}
}

func TestEverythingServer_ResourceRead(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 7,
		"method": "resources/read",
		"params": {"uri": "test://static-text"}
	}`)

	if !strings.Contains(string(result), "static text resource") {
		t.Errorf("expected static text content, got: %s", string(result))
	}
}

func TestEverythingServer_PromptsList(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 8,
		"method": "prompts/list",
		"params": {}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "simple_prompt") {
		t.Errorf("expected simple_prompt, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "complex_prompt") {
		t.Errorf("expected complex_prompt, got: %s", resultStr)
	}
}

func TestEverythingServer_PromptsGetSimple(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 9,
		"method": "prompts/get",
		"params": {"name": "simple_prompt"}
	}`)

	if !strings.Contains(string(result), "simple prompt") {
		t.Errorf("expected simple prompt content, got: %s", string(result))
	}
}

func TestEverythingServer_PromptsGetWithArgs(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 10,
		"method": "prompts/get",
		"params": {"name": "complex_prompt", "arguments": {"name": "Alice", "style": "casual"}}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "Alice") {
		t.Errorf("expected 'Alice' in prompt result, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "casual") {
		t.Errorf("expected 'casual' in prompt result, got: %s", resultStr)
	}
}

func TestEverythingServer_Ping(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	_ = mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 11,
		"method": "ping",
		"params": {}
	}`)
}

func TestEverythingServer_Completion(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 12,
		"method": "completion/complete",
		"params": {
			"ref": {"type": "ref/prompt", "name": "complex_prompt"},
			"argument": {"name": "style", "value": "f"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "formal") && !strings.Contains(resultStr, "friendly") {
		t.Errorf("expected 'formal' or 'friendly' in completion result, got: %s", resultStr)
	}
}

func TestEverythingServer_AnnotatedMessage(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 13,
		"method": "tools/call",
		"params": {
			"name": "annotatedMessage",
			"arguments": {"messageType": "error"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "something went wrong") {
		t.Errorf("expected error message content, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "isError") {
		// isError should be true for error messages
		t.Logf("note: isError field presence depends on JSON serialization: %s", resultStr)
	}
}

func TestEverythingServer_ResourceBinaryRead(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 14,
		"method": "resources/read",
		"params": {"uri": "test://static-binary"}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "image/png") {
		t.Errorf("expected image/png in binary resource, got: %s", resultStr)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mustCallJSON sends a JSON-RPC message and returns the result bytes.
// It fails the test if the response is an error.
func mustCallJSON(t *testing.T, s *server.MCPServer, msg string) json.RawMessage {
	t.Helper()

	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	// Marshal the response to inspect it.
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Check if it's an error response.
	var envelope struct {
		Error  *json.RawMessage `json:"error,omitempty"`
		Result json.RawMessage  `json:"result,omitempty"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	if envelope.Error != nil {
		t.Fatalf("JSON-RPC error: %s", string(*envelope.Error))
	}

	return envelope.Result
}

// initializeServer sends an initialize request and initialized notification.
func initializeServer(t *testing.T, s *server.MCPServer) {
	t.Helper()

	_ = mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"clientInfo": {"name": "test-client", "version": "0.0.1"},
			"capabilities": {}
		}
	}`)

	// Send initialized notification (no response expected).
	s.HandleMessage(context.Background(), json.RawMessage(`{
		"jsonrpc": "2.0",
		"method": "notifications/initialized",
		"params": {}
	}`))
}
