//go:build !official_sdk

package mcptest

import (
	"runtime"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// probeT is a testing.TB implementation that records failures without
// propagating them to the real test. Fatalf/Fatal call runtime.Goexit() so
// that the probed function terminates naturally (matching real testing.T).
type probeT struct {
	testing.TB   // embed real TB for any methods we don't override
	mu     sync.Mutex
	failed bool
}

func (p *probeT) Helper()                          {}
func (p *probeT) Log(_ ...interface{})             {}
func (p *probeT) Logf(_ string, _ ...interface{}) {}
func (p *probeT) Errorf(_ string, _ ...interface{}) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
}
func (p *probeT) Fatalf(_ string, _ ...interface{}) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	runtime.Goexit()
}
func (p *probeT) Fatal(_ ...interface{}) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	runtime.Goexit()
}

// runProbe executes fn in a fresh goroutine with a probeT, waits for it to
// finish (including via runtime.Goexit), and returns whether fn reported a failure.
// The outer test t is NOT marked as failed regardless of the probe outcome.
func runProbe(t *testing.T, fn func(tb testing.TB)) bool {
	t.Helper()
	p := &probeT{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(p)
	}()
	<-done
	return p.failed
}

// --- ExtractText ---

func TestExtractText_Pass(t *testing.T) {
	result := registry.MakeTextResult("hello")
	var got string
	if runProbe(t, func(tb testing.TB) { got = ExtractText(tb, result) }) {
		t.Error("ExtractText should not fail on valid TextContent")
	}
	if got != "hello" {
		t.Errorf("ExtractText = %q, want %q", got, "hello")
	}
}

func TestExtractText_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { ExtractText(tb, nil) }) {
		t.Error("ExtractText should fail on nil result")
	}
}

func TestExtractText_EmptyContent(t *testing.T) {
	result := &registry.CallToolResult{}
	if !runProbe(t, func(tb testing.TB) { ExtractText(tb, result) }) {
		t.Error("ExtractText should fail on empty content")
	}
}

// --- AssertToolResult ---

func TestAssertToolResult_Pass(t *testing.T) {
	result := registry.MakeTextResult("expected")
	if runProbe(t, func(tb testing.TB) { AssertToolResult(tb, result, "expected") }) {
		t.Error("AssertToolResult should not fail when text matches")
	}
}

func TestAssertToolResult_Fail(t *testing.T) {
	result := registry.MakeTextResult("actual")
	if !runProbe(t, func(tb testing.TB) { AssertToolResult(tb, result, "expected") }) {
		t.Error("AssertToolResult should fail when text does not match")
	}
}

func TestAssertToolResult_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertToolResult(tb, nil, "anything") }) {
		t.Error("AssertToolResult should fail on nil result")
	}
}

// --- AssertToolResultContains ---

func TestAssertToolResultContains_Pass(t *testing.T) {
	result := registry.MakeTextResult("hello world")
	if runProbe(t, func(tb testing.TB) { AssertToolResultContains(tb, result, "world") }) {
		t.Error("AssertToolResultContains should not fail when substring is present")
	}
}

func TestAssertToolResultContains_Fail(t *testing.T) {
	result := registry.MakeTextResult("hello world")
	if !runProbe(t, func(tb testing.TB) { AssertToolResultContains(tb, result, "missing") }) {
		t.Error("AssertToolResultContains should fail when substring is absent")
	}
}

func TestAssertToolResultContains_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertToolResultContains(tb, nil, "anything") }) {
		t.Error("AssertToolResultContains should fail on nil result")
	}
}

// --- AssertError ---

func TestAssertError_Pass(t *testing.T) {
	result := registry.MakeErrorResult("[NOT_FOUND] item not found")
	if runProbe(t, func(tb testing.TB) { AssertError(tb, result, "NOT_FOUND") }) {
		t.Error("AssertError should not fail on a matching error result")
	}
}

func TestAssertError_PassNoCode(t *testing.T) {
	result := registry.MakeErrorResult("some error")
	if runProbe(t, func(tb testing.TB) { AssertError(tb, result, "") }) {
		t.Error("AssertError with empty code should not fail for any error result")
	}
}

func TestAssertError_FailNotError(t *testing.T) {
	result := registry.MakeTextResult("success")
	if !runProbe(t, func(tb testing.TB) { AssertError(tb, result, "") }) {
		t.Error("AssertError should fail when result is not an error")
	}
}

func TestAssertError_FailWrongCode(t *testing.T) {
	result := registry.MakeErrorResult("[TIMEOUT] timed out")
	if !runProbe(t, func(tb testing.TB) { AssertError(tb, result, "NOT_FOUND") }) {
		t.Error("AssertError should fail when error code does not match")
	}
}

func TestAssertError_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertError(tb, nil, "") }) {
		t.Error("AssertError should fail on nil result")
	}
}

// --- AssertNotError ---

func TestAssertNotError_Pass(t *testing.T) {
	result := registry.MakeTextResult("all good")
	if runProbe(t, func(tb testing.TB) { AssertNotError(tb, result) }) {
		t.Error("AssertNotError should not fail on a success result")
	}
}

func TestAssertNotError_Fail(t *testing.T) {
	result := registry.MakeErrorResult("oops")
	if !runProbe(t, func(tb testing.TB) { AssertNotError(tb, result) }) {
		t.Error("AssertNotError should fail on an error result")
	}
}

func TestAssertNotError_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertNotError(tb, nil) }) {
		t.Error("AssertNotError should fail on nil result")
	}
}

// --- AssertStructured ---

func TestAssertStructured_Pass(t *testing.T) {
	type Payload struct {
		Key string `json:"key"`
	}
	data := Payload{Key: "value"}
	result := registry.MakeStructuredResult(registry.MakeTextContent("ok"), data)

	var got Payload
	if runProbe(t, func(tb testing.TB) { AssertStructured(tb, result, &got) }) {
		t.Error("AssertStructured should not fail on valid structured content")
	}
	if got.Key != "value" {
		t.Errorf("AssertStructured: got.Key = %q, want %q", got.Key, "value")
	}
}

func TestAssertStructured_NilResult(t *testing.T) {
	var got struct{}
	if !runProbe(t, func(tb testing.TB) { AssertStructured(tb, nil, &got) }) {
		t.Error("AssertStructured should fail on nil result")
	}
}

func TestAssertStructured_NilStructuredContent(t *testing.T) {
	result := registry.MakeTextResult("no structured content")
	var got struct{}
	if !runProbe(t, func(tb testing.TB) { AssertStructured(tb, result, &got) }) {
		t.Error("AssertStructured should fail when structured content is nil")
	}
}

// --- AssertResourceText ---

func makeReadResourceResult(text string) *registry.ReadResourceResult {
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{URI: "test://uri", Text: text},
		},
	}
}

func TestAssertResourceText_Pass(t *testing.T) {
	result := makeReadResourceResult("Hello, world!")
	if runProbe(t, func(tb testing.TB) { AssertResourceText(tb, result, "Hello, world!") }) {
		t.Error("AssertResourceText should not fail when text matches")
	}
}

func TestAssertResourceText_Fail(t *testing.T) {
	result := makeReadResourceResult("actual")
	if !runProbe(t, func(tb testing.TB) { AssertResourceText(tb, result, "expected") }) {
		t.Error("AssertResourceText should fail when text does not match")
	}
}

func TestAssertResourceText_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertResourceText(tb, nil, "anything") }) {
		t.Error("AssertResourceText should fail on nil result")
	}
}

// --- AssertResourceContains ---

func TestAssertResourceContains_Pass(t *testing.T) {
	result := makeReadResourceResult("hello world")
	if runProbe(t, func(tb testing.TB) { AssertResourceContains(tb, result, "world") }) {
		t.Error("AssertResourceContains should not fail when text contains substring")
	}
}

func TestAssertResourceContains_Fail(t *testing.T) {
	result := makeReadResourceResult("hello world")
	if !runProbe(t, func(tb testing.TB) { AssertResourceContains(tb, result, "missing") }) {
		t.Error("AssertResourceContains should fail when substring is absent")
	}
}

func TestAssertResourceContains_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertResourceContains(tb, nil, "anything") }) {
		t.Error("AssertResourceContains should fail on nil result")
	}
}

// --- AssertPromptMessages ---

func makeGetPromptResult(texts ...string) *registry.GetPromptResult {
	messages := make([]mcp.PromptMessage, len(texts))
	for i, text := range texts {
		messages[i] = mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))
	}
	return mcp.NewGetPromptResult("test", messages)
}

func TestAssertPromptMessages_Pass(t *testing.T) {
	result := makeGetPromptResult("msg1", "msg2")
	if runProbe(t, func(tb testing.TB) { AssertPromptMessages(tb, result, 2) }) {
		t.Error("AssertPromptMessages should not fail when count matches")
	}
}

func TestAssertPromptMessages_Fail(t *testing.T) {
	result := makeGetPromptResult("msg1")
	if !runProbe(t, func(tb testing.TB) { AssertPromptMessages(tb, result, 3) }) {
		t.Error("AssertPromptMessages should fail when count does not match")
	}
}

func TestAssertPromptMessages_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertPromptMessages(tb, nil, 0) }) {
		t.Error("AssertPromptMessages should fail on nil result")
	}
}

// --- AssertPromptContains ---

func TestAssertPromptContains_Pass(t *testing.T) {
	result := makeGetPromptResult("hello world", "another message")
	if runProbe(t, func(tb testing.TB) { AssertPromptContains(tb, result, "world") }) {
		t.Error("AssertPromptContains should not fail when a message contains the substring")
	}
}

func TestAssertPromptContains_Fail(t *testing.T) {
	result := makeGetPromptResult("hello world")
	if !runProbe(t, func(tb testing.TB) { AssertPromptContains(tb, result, "missing") }) {
		t.Error("AssertPromptContains should fail when no message contains the substring")
	}
}

func TestAssertPromptContains_NilResult(t *testing.T) {
	if !runProbe(t, func(tb testing.TB) { AssertPromptContains(tb, nil, "anything") }) {
		t.Error("AssertPromptContains should fail on nil result")
	}
}
