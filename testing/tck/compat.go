//go:build !official_sdk

package tck

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// createTestSession creates and registers an in-process session with the server.
func createTestSession(srv *registry.MCPServer) (*server.InProcessSession, error) {
	session := server.NewInProcessSession(server.GenerateInProcessSessionID(), nil)
	session.Initialize()
	if err := srv.RegisterSession(context.Background(), session); err != nil {
		return nil, fmt.Errorf("register session: %w", err)
	}
	return session, nil
}
