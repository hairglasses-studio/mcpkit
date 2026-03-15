package mcptest

import (
	"context"
	"sync"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// MockSamplingClient is a configurable test double for sampling.SamplingClient.
type MockSamplingClient struct {
	mu       sync.Mutex
	calls    []SamplingCall
	response func(ctx context.Context, req registry.CreateMessageRequest) (*registry.CreateMessageResult, error)
}

// SamplingCall records a single CreateMessage invocation.
type SamplingCall struct {
	Request registry.CreateMessageRequest
}

// NewMockSamplingClient creates a MockSamplingClient that always returns the given response.
// If response is nil, CreateMessage returns (nil, nil).
func NewMockSamplingClient(response *registry.CreateMessageResult) *MockSamplingClient {
	return &MockSamplingClient{
		response: func(ctx context.Context, req registry.CreateMessageRequest) (*registry.CreateMessageResult, error) {
			return response, nil
		},
	}
}

// NewMockSamplingClientFunc creates a MockSamplingClient with a custom response function.
func NewMockSamplingClientFunc(fn func(ctx context.Context, req registry.CreateMessageRequest) (*registry.CreateMessageResult, error)) *MockSamplingClient {
	return &MockSamplingClient{response: fn}
}

// CreateMessage implements sampling.SamplingClient.
func (m *MockSamplingClient) CreateMessage(ctx context.Context, req registry.CreateMessageRequest) (*registry.CreateMessageResult, error) {
	m.mu.Lock()
	m.calls = append(m.calls, SamplingCall{Request: req})
	m.mu.Unlock()
	return m.response(ctx, req)
}

// Calls returns all recorded CreateMessage calls.
func (m *MockSamplingClient) Calls() []SamplingCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SamplingCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// AssertCallCount fails the test if the number of calls doesn't match expected.
func (m *MockSamplingClient) AssertCallCount(t testing.TB, expected int) {
	t.Helper()
	m.mu.Lock()
	got := len(m.calls)
	m.mu.Unlock()
	if got != expected {
		t.Errorf("MockSamplingClient: expected %d calls, got %d", expected, got)
	}
}

// AssertCalled fails the test if CreateMessage was never called.
func (m *MockSamplingClient) AssertCalled(t testing.TB) {
	t.Helper()
	m.mu.Lock()
	got := len(m.calls)
	m.mu.Unlock()
	if got == 0 {
		t.Error("MockSamplingClient: expected at least one call, got none")
	}
}
