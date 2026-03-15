//go:build official_sdk

package sampling

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerSamplingClient wraps an official SDK ServerSession to implement SamplingClient.
// It delegates to the session's CreateMessage method which forwards to the client.
type ServerSamplingClient struct {
	Session *mcp.ServerSession
}

// CreateMessage sends a sampling request through the server session to the client.
// The CreateMessageRequest in the official SDK is ClientRequest[*CreateMessageParams],
// so we extract the Params field to pass to the session.
func (s *ServerSamplingClient) CreateMessage(ctx context.Context, req CreateMessageRequest) (*CreateMessageResult, error) {
	return s.Session.CreateMessage(ctx, req.Params)
}
