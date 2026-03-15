//go:build official_sdk

package roots

import (
	"context"
)

// ServerRootsClient implements RootsClient for the official go-sdk.
//
// The official go-sdk does not expose a server-side root listing capability:
// roots are a client-side concept where the MCP client advertises workspace
// boundaries to the server, but the official SDK does not surface a
// ListRoots RPC that the server can call back through. As a result,
// ListRoots returns ErrRootsUnavailable as a clear signal to callers that
// this transport cannot satisfy the request.
//
// Extension point: if your integration has a way to discover roots (for
// example, by reading them from request context populated by a custom
// transport layer or by an out-of-band configuration channel), implement
// the RootsClient interface directly rather than using ServerRootsClient.
type ServerRootsClient struct{}

// ListRoots always returns ErrRootsUnavailable because the official go-sdk
// does not provide a server-side API for listing client roots.
func (s *ServerRootsClient) ListRoots(ctx context.Context) ([]Root, error) {
	return nil, ErrRootsUnavailable
}
