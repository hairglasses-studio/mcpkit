//go:build official_sdk

package roots

import (
	"context"
)

// ServerRootsClient implements RootsClient for the official go-sdk.
// TODO: The official SDK's ServerSession.ListRoots requires a *ServerSession
// which is not yet available in mcpkit's context chain. This will be wired
// when mcpkit adds official SDK session propagation.
type ServerRootsClient struct{}

// ListRoots returns ErrRootsUnavailable — official SDK session not yet wired.
func (s *ServerRootsClient) ListRoots(ctx context.Context) ([]Root, error) {
	return nil, ErrRootsUnavailable
}
