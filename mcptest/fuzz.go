//go:build !official_sdk

package mcptest

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// fuzzInputs is a set of common adversarial inputs used by AssertNeverNilResult
// to probe handler behavior. It covers nil args, empty args, missing params,
// wrong types, and boundary values.
var fuzzInputs = []map[string]any{
	nil,
	{},
	{"": ""},
	{"name": ""},
	{"name": nil},
	{"name": 0},
	{"name": true},
	{"name": 42.5},
	{"name": []string{}},
	{"name": map[string]any{}},
	{"unknown_field": "value"},
	{"name": "valid", "extra": "extra"},
	{"name": "a very long string that might cause issues if not handled properly " +
		"0123456789012345678901234567890123456789012345678901234567890123456789"},
}

// AssertNeverNilResult exercises a handler with a set of common adversarial
// inputs and verifies that it never returns (nil, error). According to the
// MCP handler contract, handlers must always return (*CallToolResult, nil) and
// communicate errors through the result content, not the Go error return.
//
// The function calls the handler with each fuzz input and fails the test if
// any call returns a nil result, regardless of whether an error is also returned.
func AssertNeverNilResult(t testing.TB, handler registry.ToolHandlerFunc) {
	t.Helper()
	ctx := context.Background()

	for i, args := range fuzzInputs {
		req := makeFuzzRequest(args)
		result, err := handler(ctx, req)
		if result == nil {
			if err != nil {
				t.Errorf("fuzz input %d: handler returned (nil, error=%v); must return (*CallToolResult, nil)", i, err)
			} else {
				t.Errorf("fuzz input %d: handler returned (nil, nil); must return (*CallToolResult, nil)", i)
			}
		}
	}
}

// AssertNeverNilResultWithInputs is like AssertNeverNilResult but uses
// caller-provided inputs in addition to the built-in fuzz set.
func AssertNeverNilResultWithInputs(t testing.TB, handler registry.ToolHandlerFunc, extraInputs []map[string]any) {
	t.Helper()
	AssertNeverNilResult(t, handler)

	ctx := context.Background()
	for i, args := range extraInputs {
		req := makeFuzzRequest(args)
		result, err := handler(ctx, req)
		if result == nil {
			if err != nil {
				t.Errorf("extra input %d: handler returned (nil, error=%v); must return (*CallToolResult, nil)", i, err)
			} else {
				t.Errorf("extra input %d: handler returned (nil, nil); must return (*CallToolResult, nil)", i)
			}
		}
	}
}

// makeFuzzRequest constructs a CallToolRequest with the given arguments.
func makeFuzzRequest(args map[string]any) registry.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}
