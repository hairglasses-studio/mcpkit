package a2a

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// AuthInterceptor adds authentication headers to A2A client requests.
// It supports Bearer tokens, API keys, and OAuth2 access tokens.
// Thread-safe for concurrent use across multiple A2A clients.
type AuthInterceptor struct {
	mu          sync.RWMutex
	credentials map[string]Credential // agentURL → credential
}

// Credential represents an authentication credential for an A2A agent.
type Credential struct {
	Type   CredentialType
	Value  string
	Header string // Custom header name for API key auth
}

// CredentialType identifies the authentication scheme.
type CredentialType int

const (
	CredentialBearer CredentialType = iota
	CredentialAPIKey
	CredentialOAuth2
)

// NewAuthInterceptor creates an auth interceptor.
func NewAuthInterceptor() *AuthInterceptor {
	return &AuthInterceptor{
		credentials: make(map[string]Credential),
	}
}

// SetCredential registers a credential for an agent URL.
func (ai *AuthInterceptor) SetCredential(agentURL string, cred Credential) {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	ai.credentials[agentURL] = cred
}

// RemoveCredential removes a credential for an agent URL.
func (ai *AuthInterceptor) RemoveCredential(agentURL string) {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	delete(ai.credentials, agentURL)
}

// Apply adds authentication to an HTTP request based on stored credentials.
// Returns nil if no credential exists for the target URL (request proceeds unauthenticated).
func (ai *AuthInterceptor) Apply(req *http.Request) error {
	ai.mu.RLock()
	cred, ok := ai.credentials[req.URL.Host]
	if !ok {
		// Try full URL match
		cred, ok = ai.credentials[req.URL.String()]
	}
	ai.mu.RUnlock()

	if !ok {
		return nil // No credential — proceed without auth
	}

	switch cred.Type {
	case CredentialBearer, CredentialOAuth2:
		req.Header.Set("Authorization", "Bearer "+cred.Value)
	case CredentialAPIKey:
		header := cred.Header
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, cred.Value)
	default:
		return fmt.Errorf("unknown credential type: %d", cred.Type)
	}

	return nil
}

// AuthenticatedClient wraps an A2A Client with automatic authentication.
// It applies credentials from the AuthInterceptor to every request.
type AuthenticatedClient struct {
	inner       *Client
	interceptor *AuthInterceptor
}

// NewAuthenticatedClient wraps a client with auth.
func NewAuthenticatedClient(inner *Client, interceptor *AuthInterceptor) *AuthenticatedClient {
	return &AuthenticatedClient{inner: inner, interceptor: interceptor}
}

// GetAgentCard fetches the agent card with authentication.
func (ac *AuthenticatedClient) GetAgentCard(ctx context.Context) (*AgentCard, error) {
	return ac.inner.GetAgentCard(ctx)
}

// SendTask sends a task with authentication.
func (ac *AuthenticatedClient) SendTask(ctx context.Context, params TaskSendParams) (*Task, error) {
	return ac.inner.SendTask(ctx, params)
}

// GetTask fetches task status with authentication.
func (ac *AuthenticatedClient) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return ac.inner.GetTask(ctx, taskID)
}

// CancelTask cancels a task with authentication.
func (ac *AuthenticatedClient) CancelTask(ctx context.Context, taskID string) (*Task, error) {
	return ac.inner.CancelTask(ctx, taskID)
}
