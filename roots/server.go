//go:build !official_sdk

package roots

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ServerRootsClient implements RootsClient by extracting the session from context
// and delegating to the mcp-go SessionWithRoots interface.
type ServerRootsClient struct{}

// ListRoots extracts the client session from context and requests roots.
func (s *ServerRootsClient) ListRoots(ctx context.Context) ([]Root, error) {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return nil, ErrRootsUnavailable
	}

	rootsSession, ok := session.(server.SessionWithRoots)
	if !ok {
		return nil, ErrRootsUnavailable
	}

	result, err := rootsSession.ListRoots(ctx, mcp.ListRootsRequest{})
	if err != nil {
		return nil, err
	}

	roots := make([]Root, len(result.Roots))
	for i, r := range result.Roots {
		roots[i] = Root{
			URI:  r.URI,
			Name: r.Name,
		}
	}
	return roots, nil
}
