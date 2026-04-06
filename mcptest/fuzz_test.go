//go:build !official_sdk

package mcptest

import (
	"context"
	"fmt"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// goodHandler always returns a proper result.
func goodHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return registry.MakeTextResult("ok"), nil
}

// badHandler violates the MCP contract by returning (nil, error).
func badHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return nil, fmt.Errorf("something went wrong")
}

// nilNilHandler returns (nil, nil) — also a violation.
func nilNilHandler(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
	return nil, nil
}

func TestAssertNeverNilResult_Good(t *testing.T) {
	if runProbe(t, func(tb testing.TB) {
		AssertNeverNilResult(tb, goodHandler)
	}) {
		t.Error("AssertNeverNilResult should not fail for a well-behaved handler")
	}
}

func TestAssertNeverNilResult_Bad(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) {
		AssertNeverNilResult(tb, badHandler)
	}) {
		t.Error("AssertNeverNilResult should fail for a handler returning (nil, error)")
	}
}

func TestAssertNeverNilResult_NilNil(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) {
		AssertNeverNilResult(tb, nilNilHandler)
	}) {
		t.Error("AssertNeverNilResult should fail for a handler returning (nil, nil)")
	}
}

func TestAssertNeverNilResultWithInputs_Extra(t *testing.T) {
	// Handler that fails only on a specific extra input.
	selective := func(_ context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		args := registry.ExtractArguments(req)
		if args != nil {
			if v, ok := args["trigger"]; ok && v == "fail" {
				return nil, fmt.Errorf("triggered failure")
			}
		}
		return registry.MakeTextResult("ok"), nil
	}

	extra := []map[string]any{
		{"trigger": "fail"},
	}

	if !runProbe(t, func(tb testing.TB) {
		AssertNeverNilResultWithInputs(tb, selective, extra)
	}) {
		t.Error("AssertNeverNilResultWithInputs should fail when extra input triggers nil result")
	}
}

func TestSafeHandlerMiddleware_FixesNilResult(t *testing.T) {
	mw := SafeHandlerMiddleware()
	td := registry.ToolDefinition{}
	wrapped := mw("test", td, badHandler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("SafeHandlerMiddleware should clear errors, got: %v", err)
	}
	if result == nil {
		t.Fatal("SafeHandlerMiddleware should never return nil result")
	}
	if !registry.IsResultError(result) {
		t.Error("SafeHandlerMiddleware should mark the result as an error")
	}
}

func TestSafeHandlerMiddleware_FixesNilNil(t *testing.T) {
	mw := SafeHandlerMiddleware()
	td := registry.ToolDefinition{}
	wrapped := mw("test", td, nilNilHandler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("SafeHandlerMiddleware should not return error, got: %v", err)
	}
	if result == nil {
		t.Fatal("SafeHandlerMiddleware should never return nil result")
	}
}

func TestSafeHandlerMiddleware_PassesThrough(t *testing.T) {
	mw := SafeHandlerMiddleware()
	td := registry.ToolDefinition{}
	wrapped := mw("test", td, goodHandler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if registry.IsResultError(result) {
		t.Error("good handler result should not be marked as error")
	}
}

func TestSafeHandlerMiddleware_ClearsErrorWithResult(t *testing.T) {
	// Handler returns both a result AND an error.
	bothHandler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("partial"), fmt.Errorf("also an error")
	}

	mw := SafeHandlerMiddleware()
	td := registry.ToolDefinition{}
	wrapped := mw("test", td, bothHandler)

	result, err := wrapped(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("SafeHandlerMiddleware should clear error when result is present, got: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}
