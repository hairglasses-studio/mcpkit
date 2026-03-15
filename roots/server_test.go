//go:build !official_sdk

package roots

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// mockBasicSession implements server.ClientSession but NOT server.SessionWithRoots.
type mockBasicSession struct{}

func (m *mockBasicSession) Initialize()                                          {}
func (m *mockBasicSession) Initialized() bool                                    { return true }
func (m *mockBasicSession) NotificationChannel() chan<- mcp.JSONRPCNotification  { return nil }
func (m *mockBasicSession) SessionID() string                                    { return "basic" }

// mockRootsSession implements server.SessionWithRoots.
type mockRootsSession struct {
	roots []mcp.Root
	err   error
}

func (m *mockRootsSession) Initialize()                                         {}
func (m *mockRootsSession) Initialized() bool                                   { return true }
func (m *mockRootsSession) NotificationChannel() chan<- mcp.JSONRPCNotification { return nil }
func (m *mockRootsSession) SessionID() string                                   { return "roots" }
func (m *mockRootsSession) ListRoots(_ context.Context, _ mcp.ListRootsRequest) (*mcp.ListRootsResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &mcp.ListRootsResult{Roots: m.roots}, nil
}

// injectSession places a ClientSession into the context using the mcp-go server package helper.
func injectSession(ctx context.Context, session server.ClientSession) context.Context {
	srv := server.NewMCPServer("test", "1.0")
	return srv.WithContext(ctx, session)
}

func TestServerRootsClient_NoSession(t *testing.T) {
	t.Parallel()
	client := &ServerRootsClient{}
	_, err := client.ListRoots(context.Background())
	if err != ErrRootsUnavailable {
		t.Errorf("expected ErrRootsUnavailable, got %v", err)
	}
}

func TestServerRootsClient_SessionNotRootsCapable(t *testing.T) {
	t.Parallel()
	client := &ServerRootsClient{}
	ctx := injectSession(context.Background(), &mockBasicSession{})
	_, err := client.ListRoots(ctx)
	if !errors.Is(err, ErrRootsUnavailable) {
		t.Errorf("expected ErrRootsUnavailable, got %v", err)
	}
}

func TestServerRootsClient_Success(t *testing.T) {
	t.Parallel()
	session := &mockRootsSession{
		roots: []mcp.Root{
			{URI: "file:///workspace", Name: "workspace"},
			{URI: "file:///home", Name: "home"},
		},
	}
	client := &ServerRootsClient{}
	ctx := injectSession(context.Background(), session)
	roots, err := client.ListRoots(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
	if roots[0].URI != "file:///workspace" || roots[0].Name != "workspace" {
		t.Errorf("unexpected root[0]: %+v", roots[0])
	}
	if roots[1].URI != "file:///home" || roots[1].Name != "home" {
		t.Errorf("unexpected root[1]: %+v", roots[1])
	}
}

func TestServerRootsClient_ListRootsError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("connection lost")
	session := &mockRootsSession{err: wantErr}
	client := &ServerRootsClient{}
	ctx := injectSession(context.Background(), session)
	_, err := client.ListRoots(ctx)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

func TestServerRootsClient_EmptyRoots(t *testing.T) {
	t.Parallel()
	session := &mockRootsSession{roots: []mcp.Root{}}
	client := &ServerRootsClient{}
	ctx := injectSession(context.Background(), session)
	roots, err := client.ListRoots(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roots) != 0 {
		t.Errorf("expected 0 roots, got %d", len(roots))
	}
}
