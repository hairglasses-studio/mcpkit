package a2a

import (
	"context"
	"fmt"
	"testing"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestBearerTokenExtractor_HappyPath(t *testing.T) {
	ctx := WithAuthHeader(context.Background(), "Authorization", "Bearer tok_abc123")
	ext := &BearerTokenExtractor{}

	token, err := ext.Extract(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tok_abc123" {
		t.Errorf("expected token %q, got %q", "tok_abc123", token)
	}
}

func TestBearerTokenExtractor_MissingHeader(t *testing.T) {
	ext := &BearerTokenExtractor{}

	token, err := ext.Extract(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
}

func TestBearerTokenExtractor_WrongPrefix(t *testing.T) {
	ctx := WithAuthHeader(context.Background(), "Authorization", "Basic dXNlcjpwYXNz")
	ext := &BearerTokenExtractor{}

	token, err := ext.Extract(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token for non-Bearer scheme, got %q", token)
	}
}

func TestBearerTokenExtractor_CustomHeaderAndPrefix(t *testing.T) {
	ctx := WithAuthHeader(context.Background(), "X-API-Key", "Token sk-12345")
	ext := &BearerTokenExtractor{
		HeaderName:  "X-API-Key",
		TokenPrefix: "Token ",
	}

	token, err := ext.Extract(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "sk-12345" {
		t.Errorf("expected token %q, got %q", "sk-12345", token)
	}
}

func TestContextTokenInjector(t *testing.T) {
	inj := &ContextTokenInjector{}
	ctx := inj.Inject(context.Background(), "my-secret-token")

	got := TokenFromContext(ctx)
	if got != "my-secret-token" {
		t.Errorf("expected token %q, got %q", "my-secret-token", got)
	}
}

func TestTokenFromContext_Empty(t *testing.T) {
	got := TokenFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty token, got %q", got)
	}
}

func TestAuthMiddleware_PropagatesToken(t *testing.T) {
	var capturedToken string
	handler := func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		capturedToken = TokenFromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	mw := AuthMiddleware(AuthConfig{})
	td := registry.ToolDefinition{Tool: registry.Tool{Name: "test_tool"}}
	wrapped := mw("test_tool", td, handler)

	ctx := WithAuthHeader(context.Background(), "Authorization", "Bearer tok_propagated")
	req := registry.CallToolRequest{}
	result, err := wrapped(ctx, req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatal("expected success result, got error")
	}
	if capturedToken != "tok_propagated" {
		t.Errorf("expected propagated token %q, got %q", "tok_propagated", capturedToken)
	}
}

func TestAuthMiddleware_RequiredMissingToken(t *testing.T) {
	handler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		t.Fatal("handler should not be called when auth is required and missing")
		return nil, nil
	}

	mw := AuthMiddleware(AuthConfig{Required: true})
	td := registry.ToolDefinition{Tool: registry.Tool{Name: "test_tool"}}
	wrapped := mw("test_tool", td, handler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Fatal("expected error result when auth is required and missing")
	}
}

func TestAuthMiddleware_OptionalMissingToken(t *testing.T) {
	var handlerCalled bool
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		handlerCalled = true
		token := TokenFromContext(ctx)
		if token != "" {
			t.Errorf("expected no token in context, got %q", token)
		}
		return registry.MakeTextResult("ok"), nil
	}

	mw := AuthMiddleware(AuthConfig{Required: false})
	td := registry.ToolDefinition{Tool: registry.Tool{Name: "test_tool"}}
	wrapped := mw("test_tool", td, handler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatal("expected success result for optional auth with no token")
	}
	if !handlerCalled {
		t.Error("expected handler to be called when auth is optional")
	}
}

func TestAuthMiddleware_CustomHeaderName(t *testing.T) {
	var capturedToken string
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		capturedToken = TokenFromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	mw := AuthMiddleware(AuthConfig{
		HeaderName:  "X-API-Key",
		TokenPrefix: "Token ",
	})
	td := registry.ToolDefinition{Tool: registry.Tool{Name: "test_tool"}}
	wrapped := mw("test_tool", td, handler)

	ctx := WithAuthHeader(context.Background(), "X-API-Key", "Token custom-key-456")
	result, err := wrapped(ctx, registry.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatal("expected success result")
	}
	if capturedToken != "custom-key-456" {
		t.Errorf("expected token %q, got %q", "custom-key-456", capturedToken)
	}
}

func TestAuthMiddleware_CustomTokenPrefix(t *testing.T) {
	var capturedToken string
	handler := func(ctx context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		capturedToken = TokenFromContext(ctx)
		return registry.MakeTextResult("ok"), nil
	}

	mw := AuthMiddleware(AuthConfig{
		TokenPrefix: "ApiKey ",
	})
	td := registry.ToolDefinition{Tool: registry.Tool{Name: "test_tool"}}
	wrapped := mw("test_tool", td, handler)

	ctx := WithAuthHeader(context.Background(), "Authorization", "ApiKey my-api-key")
	result, err := wrapped(ctx, registry.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatal("expected success result")
	}
	if capturedToken != "my-api-key" {
		t.Errorf("expected token %q, got %q", "my-api-key", capturedToken)
	}
}

func TestAuthMiddleware_ExtractionError(t *testing.T) {
	failingExtractor := &failExtractor{err: fmt.Errorf("token expired")}

	mw := AuthMiddleware(AuthConfig{
		Extractor: failingExtractor,
	})
	td := registry.ToolDefinition{Tool: registry.Tool{Name: "test_tool"}}
	wrapped := mw("test_tool", td, func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		t.Fatal("handler should not be called on extraction error")
		return nil, nil
	})

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Fatal("expected error result on extraction failure")
	}
}

func TestAuthMiddleware_IntegrationWithExecutor(t *testing.T) {
	var capturedToken string

	reg := registry.NewToolRegistry()
	reg.RegisterModule(&testModule{
		name: "auth_test",
		tools: []registry.ToolDefinition{
			{
				Tool: registry.Tool{Name: "secure_tool", Description: "Requires auth"},
				Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
					capturedToken = TokenFromContext(ctx)
					return registry.MakeTextResult("authenticated"), nil
				},
			},
		},
	})

	authMW := AuthMiddleware(AuthConfig{Required: true})
	exec := NewBridgeExecutor(reg, ExecutorConfig{
		Middleware: []registry.Middleware{authMW},
	})

	// With valid token: should succeed.
	ctx := WithAuthHeader(context.Background(), "Authorization", "Bearer exec-token-789")
	execCtx := makeExecCtx("secure_tool", nil)
	events := collectEvents(t, exec.Execute(ctx, execCtx))

	lastEvent := events[len(events)-1]
	assertStatusUpdate(t, lastEvent, a2atypes.TaskStateCompleted)

	if capturedToken != "exec-token-789" {
		t.Errorf("expected token %q in handler, got %q", "exec-token-789", capturedToken)
	}

	// Without token: should fail.
	execCtx2 := makeExecCtx("secure_tool", nil)
	events2 := collectEvents(t, exec.Execute(context.Background(), execCtx2))

	lastEvent2 := events2[len(events2)-1]
	assertStatusUpdate(t, lastEvent2, a2atypes.TaskStateFailed)
}

// --- test helpers ---

// failExtractor is a TokenExtractor that always returns an error.
type failExtractor struct {
	err error
}

func (f *failExtractor) Extract(_ context.Context) (string, error) {
	return "", f.err
}
