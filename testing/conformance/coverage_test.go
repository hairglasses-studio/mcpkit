//go:build !official_sdk

package conformance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// --------------------------------------------------------------------------
// Module Description methods — 0% coverage
// --------------------------------------------------------------------------

func TestToolsModule_Description(t *testing.T) {
	t.Parallel()
	m := &ToolsModule{}
	if m.Description() == "" {
		t.Error("ToolsModule.Description() should not be empty")
	}
}

func TestResourcesModule_Description(t *testing.T) {
	t.Parallel()
	m := &ResourcesModule{}
	if m.Description() == "" {
		t.Error("ResourcesModule.Description() should not be empty")
	}
}

func TestPromptsModule_Description(t *testing.T) {
	t.Parallel()
	m := &PromptsModule{}
	if m.Description() == "" {
		t.Error("PromptsModule.Description() should not be empty")
	}
}

// --------------------------------------------------------------------------
// AnnotatedMessage tool — success and debug message types
// --------------------------------------------------------------------------

func TestEverythingServer_AnnotatedMessage_Success(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 20,
		"method": "tools/call",
		"params": {
			"name": "annotatedMessage",
			"arguments": {"messageType": "success"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "successfully") {
		t.Errorf("expected success message content, got: %s", resultStr)
	}
}

func TestEverythingServer_AnnotatedMessage_Debug(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 21,
		"method": "tools/call",
		"params": {
			"name": "annotatedMessage",
			"arguments": {"messageType": "debug"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "Debug") {
		t.Errorf("expected debug message, got: %s", resultStr)
	}
}

func TestEverythingServer_AnnotatedMessage_UnknownType(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 22,
		"method": "tools/call",
		"params": {
			"name": "annotatedMessage",
			"arguments": {"messageType": "unknown_type"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "Unknown message type") {
		t.Errorf("expected unknown message type content, got: %s", resultStr)
	}
}

func TestEverythingServer_AnnotatedMessage_WithImage(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 23,
		"method": "tools/call",
		"params": {
			"name": "annotatedMessage",
			"arguments": {"messageType": "success", "includeImage": true}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "image/png") {
		t.Errorf("expected image content when includeImage=true, got: %s", resultStr)
	}
}

// --------------------------------------------------------------------------
// LogMessage tool
// --------------------------------------------------------------------------

func TestEverythingServer_LogMessage(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 24,
		"method": "tools/call",
		"params": {
			"name": "logMessage",
			"arguments": {"message": "test log", "level": "warning"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "warning") || !strings.Contains(resultStr, "test log") {
		t.Errorf("expected log message output, got: %s", resultStr)
	}
}

func TestEverythingServer_LogMessage_DefaultLevel(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 25,
		"method": "tools/call",
		"params": {
			"name": "logMessage",
			"arguments": {"message": "default level log"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "info") {
		t.Errorf("expected default level 'info', got: %s", resultStr)
	}
}

// Note: longRunningOperation requires a full client session with progress token
// support, so it cannot be tested with HandleMessage directly.

// --------------------------------------------------------------------------
// Prompts — resource_prompt and image_prompt
// --------------------------------------------------------------------------

func TestEverythingServer_PromptsGetResource(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 30,
		"method": "prompts/get",
		"params": {"name": "resource_prompt"}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "static text resource") {
		t.Errorf("expected embedded resource content, got: %s", resultStr)
	}
}

func TestEverythingServer_PromptsGetImage(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 31,
		"method": "prompts/get",
		"params": {"name": "image_prompt"}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "image/png") {
		t.Errorf("expected image content in prompt, got: %s", resultStr)
	}
}

// --------------------------------------------------------------------------
// Completions — prompt name completion and resource completion
// --------------------------------------------------------------------------

func TestEverythingServer_Completion_PromptName(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	// Complete the "name" argument of complex_prompt.
	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 40,
		"method": "completion/complete",
		"params": {
			"ref": {"type": "ref/prompt", "name": "complex_prompt"},
			"argument": {"name": "name", "value": "A"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "Alice") {
		t.Errorf("expected 'Alice' in name completions for prefix 'A', got: %s", resultStr)
	}
}

func TestEverythingServer_Completion_StyleNoPrefix(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	// Complete with empty value — should return all options.
	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 41,
		"method": "completion/complete",
		"params": {
			"ref": {"type": "ref/prompt", "name": "complex_prompt"},
			"argument": {"name": "style", "value": ""}
		}
	}`)

	resultStr := string(result)
	// All four options should be present.
	for _, opt := range []string{"formal", "casual", "technical", "friendly"} {
		if !strings.Contains(resultStr, opt) {
			t.Errorf("expected %q in completions, got: %s", opt, resultStr)
		}
	}
}

func TestEverythingServer_Completion_UnknownPromptArg(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	// Unknown prompt name should return empty completions.
	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 42,
		"method": "completion/complete",
		"params": {
			"ref": {"type": "ref/prompt", "name": "nonexistent_prompt"},
			"argument": {"name": "anything", "value": "x"}
		}
	}`)

	var comp struct {
		Completion mcp.Completion `json:"completion"`
	}
	if err := json.Unmarshal(result, &comp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(comp.Completion.Values) != 0 {
		t.Errorf("expected empty values for unknown prompt, got %v", comp.Completion.Values)
	}
}

// --------------------------------------------------------------------------
// Resource completions
// --------------------------------------------------------------------------

func TestEverythingServer_Completion_Resource(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 50,
		"method": "completion/complete",
		"params": {
			"ref": {"type": "ref/resource", "uri": "test://dynamic/{name}"},
			"argument": {"name": "name", "value": "te"}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "test") {
		t.Errorf("expected 'test' in resource completions for prefix 'te', got: %s", resultStr)
	}
}

func TestEverythingServer_Completion_ResourceNoPrefix(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 51,
		"method": "completion/complete",
		"params": {
			"ref": {"type": "ref/resource", "uri": "test://dynamic/{name}"},
			"argument": {"name": "name", "value": ""}
		}
	}`)

	resultStr := string(result)
	for _, opt := range []string{"example", "test", "demo", "sample"} {
		if !strings.Contains(resultStr, opt) {
			t.Errorf("expected %q in completions, got: %s", opt, resultStr)
		}
	}
}

func TestEverythingServer_Completion_ResourceUnknownURI(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 52,
		"method": "completion/complete",
		"params": {
			"ref": {"type": "ref/resource", "uri": "test://unknown/{x}"},
			"argument": {"name": "x", "value": "abc"}
		}
	}`)

	var comp struct {
		Completion mcp.Completion `json:"completion"`
	}
	if err := json.Unmarshal(result, &comp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(comp.Completion.Values) != 0 {
		t.Errorf("expected empty values for unknown resource URI, got %v", comp.Completion.Values)
	}
}

// --------------------------------------------------------------------------
// Resource templates
// --------------------------------------------------------------------------

func TestEverythingServer_ResourceTemplateList(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 60,
		"method": "resources/templates/list",
		"params": {}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "test://dynamic/{name}") {
		t.Errorf("expected dynamic template in list, got: %s", resultStr)
	}
}

func TestEverythingServer_DynamicResourceRead(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 61,
		"method": "resources/read",
		"params": {"uri": "test://dynamic/myname"}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "Dynamic resource content for URI: test://dynamic/myname") {
		t.Errorf("expected dynamic resource content, got: %s", resultStr)
	}
}

func TestEverythingServer_TemplateResourceRead(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 62,
		"method": "resources/read",
		"params": {"uri": "test://template/abc123/data"}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "abc123") {
		t.Errorf("expected template id in result, got: %s", resultStr)
	}
}

// --------------------------------------------------------------------------
// extractTemplateID unit tests
// --------------------------------------------------------------------------

func TestExtractTemplateID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		uri  string
		want string
	}{
		{"test://template/abc/data", "abc"},
		{"test://template/123/data", "123"},
		{"test://template/long-id-value/data", "long-id-value"},
		{"too-short", "unknown"},
		{"test://template//data", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			got := extractTemplateID(tt.uri)
			if got != tt.want {
				t.Errorf("extractTemplateID(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// filterCompletions unit tests
// --------------------------------------------------------------------------

func TestFilterCompletions_NoPrefix(t *testing.T) {
	t.Parallel()
	result := filterCompletions([]string{"a", "b", "c"}, "")
	if len(result.Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(result.Values))
	}
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if result.HasMore {
		t.Error("expected HasMore=false")
	}
}

func TestFilterCompletions_WithPrefix(t *testing.T) {
	t.Parallel()
	result := filterCompletions([]string{"formal", "friendly", "casual"}, "f")
	if len(result.Values) != 2 {
		t.Errorf("expected 2 values matching 'f', got %d: %v", len(result.Values), result.Values)
	}
}

func TestFilterCompletions_NoMatch(t *testing.T) {
	t.Parallel()
	result := filterCompletions([]string{"abc", "def"}, "xyz")
	if len(result.Values) != 0 {
		t.Errorf("expected 0 values for non-matching prefix, got %d", len(result.Values))
	}
}

// --------------------------------------------------------------------------
// CompletePromptArgument and CompleteResourceArgument unit tests
// --------------------------------------------------------------------------

func TestPromptCompletionProvider_CompletePromptArgument(t *testing.T) {
	t.Parallel()
	p := &PromptCompletionProvider{}

	tests := []struct {
		name      string
		prompt    string
		argName   string
		argValue  string
		wantMin   int
		wantMatch string
	}{
		{"style with f prefix", "complex_prompt", "style", "f", 1, "formal"},
		{"name with empty prefix", "complex_prompt", "name", "", 3, "Alice"},
		{"unknown prompt", "other_prompt", "style", "f", 0, ""},
		{"unknown arg", "complex_prompt", "unknown", "f", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := p.CompletePromptArgument(context.Background(), tt.prompt, mcp.CompleteArgument{
				Name:  tt.argName,
				Value: tt.argValue,
			}, mcp.CompleteContext{})
			if err != nil {
				t.Fatalf("CompletePromptArgument: %v", err)
			}
			if len(comp.Values) < tt.wantMin {
				t.Errorf("expected at least %d values, got %d: %v", tt.wantMin, len(comp.Values), comp.Values)
			}
			if tt.wantMatch != "" {
				found := false
				for _, v := range comp.Values {
					if v == tt.wantMatch {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in values %v", tt.wantMatch, comp.Values)
				}
			}
		})
	}
}

func TestResourceCompletionProvider_CompleteResourceArgument(t *testing.T) {
	t.Parallel()
	p := &ResourceCompletionProvider{}

	tests := []struct {
		name     string
		uri      string
		argName  string
		argValue string
		wantMin  int
	}{
		{"matching uri and name", "test://dynamic/{name}", "name", "ex", 1},
		{"matching uri no prefix", "test://dynamic/{name}", "name", "", 4},
		{"non-matching uri", "test://other/{x}", "x", "a", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := p.CompleteResourceArgument(context.Background(), tt.uri, mcp.CompleteArgument{
				Name:  tt.argName,
				Value: tt.argValue,
			}, mcp.CompleteContext{})
			if err != nil {
				t.Fatalf("CompleteResourceArgument: %v", err)
			}
			if len(comp.Values) < tt.wantMin {
				t.Errorf("expected at least %d values, got %d: %v", tt.wantMin, len(comp.Values), comp.Values)
			}
		})
	}
}

// --------------------------------------------------------------------------
// PromptsModule — prompts/get with default style
// --------------------------------------------------------------------------

func TestEverythingServer_PromptsGetComplexDefaultStyle(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	// Call complex_prompt without style arg — should default to "formal".
	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 70,
		"method": "prompts/get",
		"params": {"name": "complex_prompt", "arguments": {"name": "Bob"}}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "Bob") {
		t.Errorf("expected 'Bob' in result, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "formal") {
		t.Errorf("expected default 'formal' style, got: %s", resultStr)
	}
}

// --------------------------------------------------------------------------
// Additional tool call tests for expanded Tools() coverage
// --------------------------------------------------------------------------

func TestEverythingServer_TestImageContent(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 80,
		"method": "tools/call",
		"params": {
			"name": "test_image_content",
			"arguments": {}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "image/png") {
		t.Errorf("expected image/png, got: %s", resultStr)
	}
}

func TestEverythingServer_TestAudioContent(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 81,
		"method": "tools/call",
		"params": {
			"name": "test_audio_content",
			"arguments": {}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "audio/wav") {
		t.Errorf("expected audio/wav, got: %s", resultStr)
	}
}

func TestEverythingServer_TestEmbeddedResource(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 82,
		"method": "tools/call",
		"params": {
			"name": "test_embedded_resource",
			"arguments": {}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "embedded resource") {
		t.Errorf("expected embedded resource content, got: %s", resultStr)
	}
}

func TestEverythingServer_TestMultipleContentTypes(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 83,
		"method": "tools/call",
		"params": {
			"name": "test_multiple_content_types",
			"arguments": {}
		}
	}`)

	resultStr := string(result)
	if !strings.Contains(resultStr, "Multiple content types") {
		t.Errorf("expected multiple content types text, got: %s", resultStr)
	}
}

func TestEverythingServer_SampleLLMNoServer(t *testing.T) {
	s := NewEverythingServer(DefaultConfig())
	initializeServer(t, s)

	// sampleLLM tool: will fail because there's no sampling client connected.
	result := mustCallJSON(t, s, `{
		"jsonrpc": "2.0",
		"id": 84,
		"method": "tools/call",
		"params": {
			"name": "sampleLLM",
			"arguments": {"prompt": "test", "maxWords": 10}
		}
	}`)

	resultStr := string(result)
	// Should contain some result (either sampling failure or fallback).
	if len(resultStr) == 0 {
		t.Error("expected non-empty result from sampleLLM")
	}
}
