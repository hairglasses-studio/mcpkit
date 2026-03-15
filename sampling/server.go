//go:build !official_sdk

package sampling

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ServerSamplingClient wraps an MCPServer to implement SamplingClient.
// It delegates to the server's RequestSampling method which forwards
// to the connected client session.
type ServerSamplingClient struct {
	Server *server.MCPServer
}

// CreateMessage sends a sampling request through the MCP server to the client.
func (s *ServerSamplingClient) CreateMessage(ctx context.Context, req mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
	return s.Server.RequestSampling(ctx, req)
}
