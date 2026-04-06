//go:build !official_sdk

package gate

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func makeReq() registry.CallToolRequest {
	return mcp.CallToolRequest{}
}

func okHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("ok"), nil
}

func TestMiddleware_Proceed(t *testing.T) {
	mw := Middleware(Config{Gate: AlwaysProceed})
	td := registry.ToolDefinition{}
	handler := mw("test", td, okHandler)

	result, err := handler(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if registry.IsResultError(result) {
		t.Error("expected success result")
	}
}

func TestMiddleware_Deny(t *testing.T) {
	deny := func(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
		return VerdictDeny
	}
	mw := Middleware(Config{Gate: deny, DenyMessage: "blocked"})
	td := registry.ToolDefinition{}
	handler := mw("test", td, okHandler)

	result, err := handler(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for denied call")
	}
}

func TestMiddleware_PauseApproved(t *testing.T) {
	pause := func(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
		return VerdictPause
	}
	approve := func(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) bool {
		return true
	}
	mw := Middleware(Config{Gate: pause, Approval: approve})
	td := registry.ToolDefinition{}
	handler := mw("test", td, okHandler)

	result, err := handler(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if registry.IsResultError(result) {
		t.Error("expected success result after approval")
	}
}

func TestMiddleware_PauseRejected(t *testing.T) {
	pause := func(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
		return VerdictPause
	}
	reject := func(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) bool {
		return false
	}
	mw := Middleware(Config{Gate: pause, Approval: reject})
	td := registry.ToolDefinition{}
	handler := mw("test", td, okHandler)

	result, err := handler(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for rejected pause")
	}
}

func TestMiddleware_PauseNoApprovalHandler(t *testing.T) {
	pause := func(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
		return VerdictPause
	}
	mw := Middleware(Config{Gate: pause}) // no Approval func
	td := registry.ToolDefinition{}
	handler := mw("test", td, okHandler)

	result, err := handler(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result when no approval handler is configured")
	}
}

func TestDenyWrites(t *testing.T) {
	readTD := registry.ToolDefinition{IsWrite: false}
	writeTD := registry.ToolDefinition{IsWrite: true}

	if v := DenyWrites(context.Background(), "read", readTD, makeReq()); v != VerdictProceed {
		t.Errorf("DenyWrites on read tool = %v, want Proceed", v)
	}
	if v := DenyWrites(context.Background(), "write", writeTD, makeReq()); v != VerdictDeny {
		t.Errorf("DenyWrites on write tool = %v, want Deny", v)
	}
}

func TestPauseWrites(t *testing.T) {
	readTD := registry.ToolDefinition{IsWrite: false}
	writeTD := registry.ToolDefinition{IsWrite: true}

	if v := PauseWrites(context.Background(), "read", readTD, makeReq()); v != VerdictProceed {
		t.Errorf("PauseWrites on read tool = %v, want Proceed", v)
	}
	if v := PauseWrites(context.Background(), "write", writeTD, makeReq()); v != VerdictPause {
		t.Errorf("PauseWrites on write tool = %v, want Pause", v)
	}
}

func TestVerdict_String(t *testing.T) {
	tests := []struct {
		v    Verdict
		want string
	}{
		{VerdictProceed, "proceed"},
		{VerdictPause, "pause"},
		{VerdictDeny, "deny"},
		{Verdict(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.v.String(); got != tt.want {
			t.Errorf("Verdict(%d).String() = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestMiddleware_UnknownVerdict(t *testing.T) {
	unknown := func(_ context.Context, _ string, _ registry.ToolDefinition, _ registry.CallToolRequest) Verdict {
		return Verdict(99)
	}
	mw := Middleware(Config{Gate: unknown})
	td := registry.ToolDefinition{}
	handler := mw("test", td, okHandler)

	result, err := handler(context.Background(), makeReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Error("expected error result for unknown verdict")
	}
}
