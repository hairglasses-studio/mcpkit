package registry

import (
	"context"
	"testing"
)

// mockReporter is a test implementation of ProgressReporter.
type mockReporter struct {
	calls []progressCall
}

type progressCall struct {
	progress float64
	message  string
}

func (m *mockReporter) Report(ctx context.Context, progress float64, message string) error {
	m.calls = append(m.calls, progressCall{progress: progress, message: message})
	return nil
}

func TestGetProgressReporter_Nil(t *testing.T) {
	t.Parallel()
	reporter := GetProgressReporter(context.Background())
	if reporter != nil {
		t.Errorf("expected nil reporter from empty context, got %v", reporter)
	}
}

func TestWithProgressReporter_RoundTrip(t *testing.T) {
	t.Parallel()
	mock := &mockReporter{}
	ctx := WithProgressReporter(context.Background(), mock)
	got := GetProgressReporter(ctx)
	if got != mock {
		t.Errorf("GetProgressReporter returned different reporter than was set")
	}
}

func TestProgressReporter_Report(t *testing.T) {
	t.Parallel()
	mock := &mockReporter{}

	if err := mock.Report(context.Background(), 0.5, "halfway"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}
	if mock.calls[0].progress != 0.5 {
		t.Errorf("progress = %v, want 0.5", mock.calls[0].progress)
	}
	if mock.calls[0].message != "halfway" {
		t.Errorf("message = %q, want 'halfway'", mock.calls[0].message)
	}
}

func TestProgressMiddleware_InjectsReporter(t *testing.T) {
	t.Parallel()
	mock := &mockReporter{}

	factory := func(name string, td ToolDefinition) ProgressReporter {
		return mock
	}

	var capturedReporter ProgressReporter
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		capturedReporter = GetProgressReporter(ctx)
		return MakeTextResult("ok"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	wrapped := ProgressMiddleware(factory)("test_tool", td, inner)

	_, err := wrapped(context.Background(), CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReporter == nil {
		t.Fatal("expected reporter to be injected into context, got nil")
	}
	if capturedReporter != mock {
		t.Errorf("injected reporter is not the expected mock")
	}
}

func TestProgressMiddleware_NilFactory(t *testing.T) {
	t.Parallel()
	// A factory that returns nil should not inject anything.
	factory := func(name string, td ToolDefinition) ProgressReporter {
		return nil
	}

	var capturedReporter ProgressReporter
	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		capturedReporter = GetProgressReporter(ctx)
		return MakeTextResult("ok"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "test_tool"}}
	wrapped := ProgressMiddleware(factory)("test_tool", td, inner)

	_, err := wrapped(context.Background(), CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReporter != nil {
		t.Errorf("expected nil reporter when factory returns nil, got %v", capturedReporter)
	}
}

func TestProgressMiddleware_PassesToolNameToFactory(t *testing.T) {
	t.Parallel()
	var gotName string
	factory := func(name string, td ToolDefinition) ProgressReporter {
		gotName = name
		return &mockReporter{}
	}

	inner := func(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("ok"), nil
	}

	td := ToolDefinition{Tool: Tool{Name: "my_special_tool"}}
	wrapped := ProgressMiddleware(factory)("my_special_tool", td, inner)
	_, _ = wrapped(context.Background(), CallToolRequest{})

	if gotName != "my_special_tool" {
		t.Errorf("factory received name %q, want 'my_special_tool'", gotName)
	}
}
