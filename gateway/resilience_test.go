//go:build !official_sdk

package gateway

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resilience"
)

func makeTestHandler(result string) registry.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: result}},
		}, nil
	}
}

func makeErrorHandler() registry.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("upstream error")
	}
}

func makeSlowHandler(d time.Duration) registry.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-time.After(d):
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "done"}},
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func TestUpstreamResilience_NilPolicy(t *testing.T) {
	ur := newUpstreamResilience("test", UpstreamPolicy{})
	if ur != nil {
		t.Fatal("expected nil resilience for zero-value policy")
	}
}

func TestUpstreamResilience_CircuitBreaker(t *testing.T) {
	cbConfig := resilience.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
		HalfOpenMaxCalls: 1,
	}
	ur := newUpstreamResilience("test", UpstreamPolicy{
		CircuitBreaker: &cbConfig,
	})
	if ur == nil {
		t.Fatal("expected non-nil resilience")
	}

	handler := ur.wrapHandler("test", makeErrorHandler())
	ctx := context.Background()
	req := mcp.CallToolRequest{}

	// Two failures should open the circuit
	for i := range 2 {
		_, err := handler(ctx, req)
		if err == nil {
			t.Fatalf("expected error on call %d", i)
		}
	}

	// Circuit should now be open — next call gets error result, not Go error
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("expected nil error when circuit is open, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result when circuit is open")
	}
	if !result.IsError {
		// Check content for circuit breaker message
		found := false
		for _, c := range result.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				if strings.Contains(tc.Text, "circuit breaker is open") {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("expected circuit breaker error message in result")
		}
	}

	// Verify circuit state
	state := ur.circuitState()
	if state != "open" {
		t.Fatalf("expected circuit state 'open', got %q", state)
	}
}

func TestUpstreamResilience_RateLimit(t *testing.T) {
	ur := newUpstreamResilience("test", UpstreamPolicy{
		RateLimit: &resilience.RateLimitConfig{
			Rate:  1,
			Burst: 1,
		},
	})
	handler := ur.wrapHandler("test", makeTestHandler("ok"))
	ctx := context.Background()
	req := mcp.CallToolRequest{}

	// First call should succeed (uses the burst token)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("first call should succeed: %v", err)
	}
	if result.IsError {
		t.Fatal("first call should not be an error result")
	}

	// Second immediate call should be rate limited
	result, err = handler(ctx, req)
	if err != nil {
		t.Fatalf("rate limited call should return nil error: %v", err)
	}
	found := false
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			if strings.Contains(tc.Text, "rate limit exceeded") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected rate limit error in result")
	}
}

func TestUpstreamResilience_Timeout(t *testing.T) {
	ur := newUpstreamResilience("test", UpstreamPolicy{
		CallTimeout: 50 * time.Millisecond,
	})
	handler := ur.wrapHandler("test", makeSlowHandler(5*time.Second))
	ctx := context.Background()
	req := mcp.CallToolRequest{}

	_, err := handler(ctx, req)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestUpstreamResilience_NoPolicy(t *testing.T) {
	// Wrapping nil resilience should return the original handler
	handler := makeTestHandler("passthrough")
	wrapped := (*upstreamResilience)(nil).wrapHandler("test", handler)

	ctx := context.Background()
	result, err := wrapped(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			if tc.Text == "passthrough" {
				return
			}
		}
	}
	t.Fatal("expected passthrough result")
}

func TestUpstreamResilience_CircuitStateEmpty(t *testing.T) {
	var ur *upstreamResilience
	if s := ur.circuitState(); s != "" {
		t.Fatalf("expected empty state, got %q", s)
	}
}

func TestUpstreamPolicy_InUpstreamConfig(t *testing.T) {
	cfg := UpstreamConfig{
		Name: "test",
		URL:  "http://localhost:8080",
		Policy: UpstreamPolicy{
			CircuitBreaker: &resilience.CircuitBreakerConfig{
				FailureThreshold: 3,
				SuccessThreshold: 1,
				Timeout:          time.Second,
				HalfOpenMaxCalls: 1,
			},
			RateLimit: &resilience.RateLimitConfig{
				Rate:  100,
				Burst: 10,
			},
			CallTimeout: 5 * time.Second,
		},
	}
	u := &upstream{
		config:     cfg,
		resilience: newUpstreamResilience(cfg.Name, cfg.Policy),
	}
	if u.resilience == nil {
		t.Fatal("expected resilience to be initialized")
	}
	if u.resilience.cb == nil {
		t.Fatal("expected circuit breaker")
	}
	if u.resilience.limiter == nil {
		t.Fatal("expected rate limiter")
	}
	if u.resilience.timeout != 5*time.Second {
		t.Fatalf("expected 5s timeout, got %v", u.resilience.timeout)
	}
}
