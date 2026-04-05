package roots

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

type callToolRequest = registry.CallToolRequest
type callToolResult = registry.CallToolResult

func dummyToolDef() registry.ToolDefinition {
	return registry.ToolDefinition{}
}

func makeTextResult(text string) *registry.CallToolResult {
	return registry.MakeTextResult(text)
}

// mockClient is a test double for RootsClient.
type mockClient struct {
	listRootsFunc func(ctx context.Context) ([]Root, error)
	callCount     int
	mu            sync.Mutex
}

func (m *mockClient) ListRoots(ctx context.Context) ([]Root, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()
	return m.listRootsFunc(ctx)
}

func TestListRoots_NoClient(t *testing.T) {
	t.Parallel()
	_, err := ListRoots(context.Background())
	if !errors.Is(err, ErrRootsUnavailable) {
		t.Errorf("expected ErrRootsUnavailable, got %v", err)
	}
}

func TestListRoots_WithClient(t *testing.T) {
	t.Parallel()
	expected := []Root{
		{URI: "file:///workspace", Name: "workspace"},
		{URI: "file:///home", Name: "home"},
	}
	client := &mockClient{
		listRootsFunc: func(ctx context.Context) ([]Root, error) {
			return expected, nil
		},
	}
	ctx := WithRootsClient(context.Background(), client)
	roots, err := ListRoots(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roots) != len(expected) {
		t.Fatalf("expected %d roots, got %d", len(expected), len(roots))
	}
	for i, r := range roots {
		if r.URI != expected[i].URI || r.Name != expected[i].Name {
			t.Errorf("root[%d]: expected %+v, got %+v", i, expected[i], r)
		}
	}
}

func TestClientFromContext_Nil(t *testing.T) {
	t.Parallel()
	c := ClientFromContext(context.Background())
	if c != nil {
		t.Errorf("expected nil, got %v", c)
	}
}

func TestWithRootsClient_RoundTrip(t *testing.T) {
	t.Parallel()
	client := &mockClient{
		listRootsFunc: func(ctx context.Context) ([]Root, error) {
			return nil, nil
		},
	}
	ctx := WithRootsClient(context.Background(), client)
	got := ClientFromContext(ctx)
	if got != client {
		t.Errorf("expected %v, got %v", client, got)
	}
}

func TestMiddleware_InjectsClient(t *testing.T) {
	t.Parallel()
	client := &mockClient{
		listRootsFunc: func(ctx context.Context) ([]Root, error) {
			return nil, nil
		},
	}

	mw := Middleware(client)

	var capturedClient RootsClient
	handler := mw("tool", dummyToolDef(), func(ctx context.Context, req callToolRequest) (*callToolResult, error) {
		capturedClient = ClientFromContext(ctx)
		return makeTextResult("ok"), nil
	})

	_, err := handler(context.Background(), callToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedClient != client {
		t.Errorf("expected client to be injected, got %v", capturedClient)
	}
}

func TestCachedClient_CachesResult(t *testing.T) {
	t.Parallel()
	expected := []Root{{URI: "file:///ws", Name: "ws"}}
	inner := &mockClient{
		listRootsFunc: func(ctx context.Context) ([]Root, error) {
			return expected, nil
		},
	}
	cached := NewCachedClient(inner)

	// First call — fetches
	roots1, err := cached.ListRoots(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roots1) != 1 || roots1[0].URI != "file:///ws" {
		t.Errorf("unexpected roots: %+v", roots1)
	}

	// Second call — should use cache
	roots2, err := cached.ListRoots(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roots2) != 1 {
		t.Errorf("unexpected roots: %+v", roots2)
	}

	inner.mu.Lock()
	count := inner.callCount
	inner.mu.Unlock()
	if count != 1 {
		t.Errorf("expected inner to be called once, got %d", count)
	}
}

func TestCachedClient_Invalidate(t *testing.T) {
	t.Parallel()
	call := 0
	inner := &mockClient{
		listRootsFunc: func(ctx context.Context) ([]Root, error) {
			call++
			return []Root{{URI: "file:///v" + string(rune('0'+call)), Name: "ws"}}, nil
		},
	}
	cached := NewCachedClient(inner)

	// First call
	roots1, _ := cached.ListRoots(context.Background())

	// Invalidate
	cached.Invalidate()

	// Second call — should re-fetch
	roots2, _ := cached.ListRoots(context.Background())

	if roots1[0].URI == roots2[0].URI {
		t.Error("expected different URIs after invalidation")
	}
}

func TestCachedClient_InnerFetchError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("backend unavailable")
	inner := &mockClient{
		listRootsFunc: func(ctx context.Context) ([]Root, error) {
			return nil, wantErr
		},
	}
	cached := NewCachedClient(inner)

	_, err := cached.ListRoots(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}

	// Cache should NOT be marked valid after an error; subsequent call should retry.
	inner.mu.Lock()
	inner.listRootsFunc = func(ctx context.Context) ([]Root, error) {
		return []Root{{URI: "file:///ok", Name: "ok"}}, nil
	}
	inner.mu.Unlock()

	roots, err := cached.ListRoots(context.Background())
	if err != nil {
		t.Fatalf("expected success on retry, got %v", err)
	}
	if len(roots) != 1 || roots[0].URI != "file:///ok" {
		t.Errorf("unexpected roots after retry: %+v", roots)
	}
}

func TestMiddleware_NilClient(t *testing.T) {
	t.Parallel()
	mw := Middleware(nil)

	var capturedClient RootsClient
	handler := mw("tool", dummyToolDef(), func(ctx context.Context, req callToolRequest) (*callToolResult, error) {
		capturedClient = ClientFromContext(ctx)
		return makeTextResult("ok"), nil
	})

	_, err := handler(context.Background(), callToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedClient != nil {
		t.Errorf("expected nil client when Middleware called with nil, got %v", capturedClient)
	}
}

func TestCachedClient_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	inner := &mockClient{
		listRootsFunc: func(ctx context.Context) ([]Root, error) {
			return []Root{{URI: "file:///ws", Name: "ws"}}, nil
		},
	}
	cached := NewCachedClient(inner)

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			_, err := cached.ListRoots(context.Background())
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
		if i%10 == 0 {
			wg.Go(func() {
				cached.Invalidate()
			})
		}
	}
	wg.Wait()
}
