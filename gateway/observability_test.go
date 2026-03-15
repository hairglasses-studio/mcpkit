//go:build !official_sdk

package gateway

import (
	"context"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestTracingMiddleware(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}},
		}, nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "upstream.tool"},
	}

	wrapped := mw("upstream.tool", td, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestTracingMiddleware_Error(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("upstream failed")
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "upstream.tool"},
	}

	wrapped := mw("upstream.tool", td, handler)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTracingMiddleware_ErrorResult(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return registry.MakeErrorResult("something failed"), nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "svc.action"},
	}

	wrapped := mw("svc.action", td, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}
}

func TestTracingMiddleware_NoNamespace(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := TracingMiddleware(tracer)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	}

	td := registry.ToolDefinition{
		Tool: mcp.Tool{Name: "simpletool"},
	}

	wrapped := mw("simpletool", td, handler)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
