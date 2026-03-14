package mcptest

import (
	"context"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// Call records a single tool invocation.
type Call struct {
	Name   string
	Args   map[string]interface{}
	Result *mcp.CallToolResult
	Err    error
}

// Recorder is a middleware that records all tool calls.
type Recorder struct {
	mu    sync.Mutex
	calls []Call
}

// NewRecorder creates a new call recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Middleware returns a registry.Middleware that records tool calls.
func (r *Recorder) Middleware() registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := next(ctx, request)

			var args map[string]interface{}
			if request.Params.Arguments != nil {
				if a, ok := request.Params.Arguments.(map[string]interface{}); ok {
					args = a
				}
			}

			r.mu.Lock()
			r.calls = append(r.calls, Call{
				Name:   name,
				Args:   args,
				Result: result,
				Err:    err,
			})
			r.mu.Unlock()

			return result, err
		}
	}
}

// Calls returns all recorded calls.
func (r *Recorder) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Call, len(r.calls))
	copy(result, r.calls)
	return result
}

// CallCount returns the number of recorded calls.
func (r *Recorder) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// CallsFor returns all recorded calls for the given tool name.
func (r *Recorder) CallsFor(name string) []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []Call
	for _, c := range r.calls {
		if c.Name == name {
			result = append(result, c)
		}
	}
	return result
}

// Reset clears all recorded calls.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

// AssertCallCount checks that the expected number of calls were recorded.
func (r *Recorder) AssertCallCount(t testing.TB, expected int) {
	t.Helper()
	r.mu.Lock()
	got := len(r.calls)
	r.mu.Unlock()
	if got != expected {
		t.Errorf("call count = %d, want %d", got, expected)
	}
}

// AssertCalled checks that the given tool was called at least once.
func (r *Recorder) AssertCalled(t testing.TB, name string) {
	t.Helper()
	if len(r.CallsFor(name)) == 0 {
		t.Errorf("tool %q was never called", name)
	}
}
