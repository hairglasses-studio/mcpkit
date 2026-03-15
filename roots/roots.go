// Package roots provides client workspace root discovery, caching, and context
// helpers for accessing the filesystem roots reported by an MCP client.
package roots

import (
	"context"
	"errors"
)

// Root represents a client workspace root (SDK-agnostic).
type Root struct {
	URI  string
	Name string
}

// RootsClient requests workspace roots from the connected MCP client.
type RootsClient interface {
	ListRoots(ctx context.Context) ([]Root, error)
}

// ErrRootsUnavailable is returned when no RootsClient is available in the context.
var ErrRootsUnavailable = errors.New("roots: no client available in context")

type rootsKey struct{}

// WithRootsClient returns a context with the given RootsClient attached.
func WithRootsClient(ctx context.Context, client RootsClient) context.Context {
	return context.WithValue(ctx, rootsKey{}, client)
}

// ClientFromContext extracts the RootsClient from the context, or nil if none.
func ClientFromContext(ctx context.Context) RootsClient {
	c, _ := ctx.Value(rootsKey{}).(RootsClient)
	return c
}

// ListRoots is a convenience function that extracts the client from context and calls ListRoots.
func ListRoots(ctx context.Context) ([]Root, error) {
	client := ClientFromContext(ctx)
	if client == nil {
		return nil, ErrRootsUnavailable
	}
	return client.ListRoots(ctx)
}
