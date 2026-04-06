package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthInterceptor_SetAndApply_Bearer(t *testing.T) {
	t.Parallel()

	ai := NewAuthInterceptor()
	ai.SetCredential("example.com", Credential{Type: CredentialBearer, Value: "tok_123"})

	req, _ := http.NewRequest("GET", "http://example.com/.well-known/agent.json", nil)
	if err := ai.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := req.Header.Get("Authorization")
	if got != "Bearer tok_123" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer tok_123")
	}
}

func TestAuthInterceptor_SetAndApply_APIKey(t *testing.T) {
	t.Parallel()

	ai := NewAuthInterceptor()
	ai.SetCredential("example.com", Credential{Type: CredentialAPIKey, Value: "sk-abc"})

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := ai.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := req.Header.Get("X-API-Key")
	if got != "sk-abc" {
		t.Errorf("X-API-Key = %q, want %q", got, "sk-abc")
	}
}

func TestAuthInterceptor_SetAndApply_APIKeyCustomHeader(t *testing.T) {
	t.Parallel()

	ai := NewAuthInterceptor()
	ai.SetCredential("example.com", Credential{
		Type:   CredentialAPIKey,
		Value:  "key-xyz",
		Header: "X-Custom-Auth",
	})

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := ai.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := req.Header.Get("X-Custom-Auth")
	if got != "key-xyz" {
		t.Errorf("X-Custom-Auth = %q, want %q", got, "key-xyz")
	}
}

func TestAuthInterceptor_SetAndApply_OAuth2(t *testing.T) {
	t.Parallel()

	ai := NewAuthInterceptor()
	ai.SetCredential("example.com", Credential{Type: CredentialOAuth2, Value: "access-token-456"})

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := ai.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := req.Header.Get("Authorization")
	if got != "Bearer access-token-456" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer access-token-456")
	}
}

func TestAuthInterceptor_NoCredential(t *testing.T) {
	t.Parallel()

	ai := NewAuthInterceptor()
	req, _ := http.NewRequest("GET", "http://unknown.com/", nil)
	if err := ai.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// No credential -- no header should be set.
	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Errorf("expected no auth header, got %q", auth)
	}
}

func TestAuthInterceptor_RemoveCredential(t *testing.T) {
	t.Parallel()

	ai := NewAuthInterceptor()
	ai.SetCredential("example.com", Credential{Type: CredentialBearer, Value: "tok_123"})
	ai.RemoveCredential("example.com")

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := ai.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Errorf("expected no auth after removal, got %q", auth)
	}
}

func TestAuthInterceptor_FullURLMatch(t *testing.T) {
	t.Parallel()

	ai := NewAuthInterceptor()
	fullURL := "http://example.com/custom/path"
	ai.SetCredential(fullURL, Credential{Type: CredentialBearer, Value: "full-url-tok"})

	req, _ := http.NewRequest("GET", fullURL, nil)
	if err := ai.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := req.Header.Get("Authorization")
	if got != "Bearer full-url-tok" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer full-url-tok")
	}
}

func TestAuthenticatedClient_GetAgentCard(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(AgentCard{Name: "auth-agent"})
	}))
	defer ts.Close()

	inner := NewClient(ts.URL)
	ai := NewAuthInterceptor()
	ac := NewAuthenticatedClient(inner, ai)

	card, err := ac.GetAgentCard(context.Background())
	if err != nil {
		t.Fatalf("GetAgentCard: %v", err)
	}
	if card.Name != "auth-agent" {
		t.Errorf("Name = %q, want auth-agent", card.Name)
	}
}

func TestAuthenticatedClient_SendTask(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := Task{ID: "t1", State: TaskCompleted}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	inner := NewClient(ts.URL)
	ai := NewAuthInterceptor()
	ac := NewAuthenticatedClient(inner, ai)

	task, err := ac.SendTask(context.Background(), TaskSendParams{ID: "t1"})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}
	if task.State != TaskCompleted {
		t.Errorf("State = %q, want completed", task.State)
	}
}

func TestAuthenticatedClient_GetTask(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := Task{ID: "t1", State: TaskWorking}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	inner := NewClient(ts.URL)
	ai := NewAuthInterceptor()
	ac := NewAuthenticatedClient(inner, ai)

	task, err := ac.GetTask(context.Background(), "t1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.State != TaskWorking {
		t.Errorf("State = %q, want working", task.State)
	}
}

func TestAuthenticatedClient_CancelTask(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		task := Task{ID: "t1", State: TaskCanceled}
		taskJSON, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: taskJSON}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	inner := NewClient(ts.URL)
	ai := NewAuthInterceptor()
	ac := NewAuthenticatedClient(inner, ai)

	task, err := ac.CancelTask(context.Background(), "t1")
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if task.State != TaskCanceled {
		t.Errorf("State = %q, want canceled", task.State)
	}
}
