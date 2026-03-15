package mcptest

import (
	"context"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestMockSamplingClient_Returns(t *testing.T) {
	t.Parallel()

	want := &registry.CreateMessageResult{
		Model:      "claude-3-5-sonnet",
		StopReason: "end_turn",
	}

	mock := NewMockSamplingClient(want)
	got, err := mock.CreateMessage(context.Background(), registry.CreateMessageRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected response %v, got %v", want, got)
	}
}

func TestMockSamplingClient_RecordsCalls(t *testing.T) {
	t.Parallel()

	mock := NewMockSamplingClient(nil)

	req1 := registry.CreateMessageRequest{}
	req1.MaxTokens = 100

	req2 := registry.CreateMessageRequest{}
	req2.MaxTokens = 200

	mock.CreateMessage(context.Background(), req1) //nolint:errcheck
	mock.CreateMessage(context.Background(), req2) //nolint:errcheck

	calls := mock.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Request.MaxTokens != 100 {
		t.Errorf("call[0] MaxTokens = %d, want 100", calls[0].Request.MaxTokens)
	}
	if calls[1].Request.MaxTokens != 200 {
		t.Errorf("call[1] MaxTokens = %d, want 200", calls[1].Request.MaxTokens)
	}
}

func TestMockSamplingClient_AssertCallCount(t *testing.T) {
	t.Parallel()

	mock := NewMockSamplingClient(nil)
	mock.CreateMessage(context.Background(), registry.CreateMessageRequest{}) //nolint:errcheck

	mock.AssertCallCount(t, 1)
	mock.AssertCalled(t)
}

func TestMockSamplingClient_NilResponse(t *testing.T) {
	t.Parallel()

	mock := NewMockSamplingClient(nil)
	result, err := mock.CreateMessage(context.Background(), registry.CreateMessageRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestNewMockSamplingClientFunc(t *testing.T) {
	t.Parallel()

	want := &registry.CreateMessageResult{Model: "custom-model"}
	mock := NewMockSamplingClientFunc(func(ctx context.Context, req registry.CreateMessageRequest) (*registry.CreateMessageResult, error) {
		return want, nil
	})

	got, err := mock.CreateMessage(context.Background(), registry.CreateMessageRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected %v, got %v", want, got)
	}
	mock.AssertCallCount(t, 1)
	mock.AssertCalled(t)
}

func TestMockSamplingClient_AssertCallCount_Fail(t *testing.T) {
	mock := NewMockSamplingClient(nil)
	// No calls made — asserting 1 call should fail.
	failed := false
	mockT := &mockTB{TB: t, onError: func() { failed = true }}
	mock.AssertCallCount(mockT, 1)
	if !failed {
		t.Error("AssertCallCount should have failed when call count doesn't match")
	}
}

func TestMockSamplingClient_AssertCalled_Fail(t *testing.T) {
	mock := NewMockSamplingClient(nil)
	// No calls made — AssertCalled should fail.
	failed := false
	mockT := &mockTB{TB: t, onError: func() { failed = true }}
	mock.AssertCalled(mockT)
	if !failed {
		t.Error("AssertCalled should have failed when no calls were made")
	}
}
