//go:build !official_sdk

package sampling

import (
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestWithSystemPrompt_Empty(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithSystemPrompt("")(&p)
	if p.SystemPrompt != "" {
		t.Errorf("expected empty system prompt, got %q", p.SystemPrompt)
	}
}

func TestWithSystemPrompt_MultiLine(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	s := "Line 1.\nLine 2.\nLine 3."
	WithSystemPrompt(s)(&p)
	if p.SystemPrompt != s {
		t.Errorf("expected %q, got %q", s, p.SystemPrompt)
	}
}

func TestWithSystemPrompt_Overwrite(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithSystemPrompt("first")(&p)
	WithSystemPrompt("second")(&p)
	if p.SystemPrompt != "second" {
		t.Errorf("expected system prompt to be overwritten to %q, got %q", "second", p.SystemPrompt)
	}
}

func TestWithTemperature_Zero(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithTemperature(0)(&p)
	if p.Temperature != 0 {
		t.Errorf("expected temperature 0, got %f", p.Temperature)
	}
}

func TestWithTemperature_One(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithTemperature(1.0)(&p)
	if p.Temperature != 1.0 {
		t.Errorf("expected temperature 1.0, got %f", p.Temperature)
	}
}

func TestWithTemperature_Overwrite(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithTemperature(0.3)(&p)
	WithTemperature(0.9)(&p)
	if p.Temperature != 0.9 {
		t.Errorf("expected temperature 0.9, got %f", p.Temperature)
	}
}

func TestWithModel_SetsMetadata(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithModel("claude-3-7-sonnet")(&p)
	meta, ok := p.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any metadata, got %T", p.Metadata)
	}
	if meta["preferredModel"] != "claude-3-7-sonnet" {
		t.Errorf("expected preferredModel %q, got %v", "claude-3-7-sonnet", meta["preferredModel"])
	}
}

func TestWithModel_EmptyString(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithModel("")(&p)
	meta, ok := p.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any metadata, got %T", p.Metadata)
	}
	if meta["preferredModel"] != "" {
		t.Errorf("expected empty preferredModel, got %v", meta["preferredModel"])
	}
}

func TestWithModel_Overwrite(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithModel("old-model")(&p)
	WithModel("new-model")(&p)
	meta, ok := p.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any metadata, got %T", p.Metadata)
	}
	if meta["preferredModel"] != "new-model" {
		t.Errorf("expected preferredModel to be overwritten to %q, got %v", "new-model", meta["preferredModel"])
	}
}

func TestWithMaxTokens_Sets(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithMaxTokens(256)(&p)
	if p.MaxTokens != 256 {
		t.Errorf("expected MaxTokens 256, got %d", p.MaxTokens)
	}
}

func TestWithMaxTokens_Zero(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithMaxTokens(0)(&p)
	if p.MaxTokens != 0 {
		t.Errorf("expected MaxTokens 0, got %d", p.MaxTokens)
	}
}

func TestWithMaxTokens_Overwrite(t *testing.T) {
	t.Parallel()
	var p CreateMessageParams
	WithMaxTokens(512)(&p)
	WithMaxTokens(128)(&p)
	if p.MaxTokens != 128 {
		t.Errorf("expected MaxTokens 128, got %d", p.MaxTokens)
	}
}

func TestCompletionRequest_DefaultMaxTokens(t *testing.T) {
	t.Parallel()
	msgs := []SamplingMessage{TextMessage("user", "hi")}
	req := CompletionRequest(msgs)
	if req.MaxTokens != 1024 {
		t.Errorf("expected default MaxTokens 1024, got %d", req.MaxTokens)
	}
}

func TestCompletionRequest_OverrideMaxTokens(t *testing.T) {
	t.Parallel()
	msgs := []SamplingMessage{TextMessage("user", "hi")}
	req := CompletionRequest(msgs, WithMaxTokens(512))
	if req.MaxTokens != 512 {
		t.Errorf("expected MaxTokens 512, got %d", req.MaxTokens)
	}
}

func TestCompletionRequest_EmptyMessages(t *testing.T) {
	t.Parallel()
	req := CompletionRequest(nil)
	if req.Messages != nil {
		t.Errorf("expected nil messages, got %v", req.Messages)
	}
	if req.MaxTokens != 1024 {
		t.Errorf("expected default MaxTokens 1024, got %d", req.MaxTokens)
	}
}

func TestCompletionRequest_MultipleMessages(t *testing.T) {
	t.Parallel()
	msgs := []SamplingMessage{
		TextMessage("user", "first"),
		TextMessage("assistant", "second"),
		TextMessage("user", "third"),
	}
	req := CompletionRequest(msgs)
	if len(req.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(req.Messages))
	}
}

func TestCompletionRequest_AllOptions(t *testing.T) {
	t.Parallel()
	msgs := []SamplingMessage{TextMessage("user", "test")}
	req := CompletionRequest(msgs,
		WithMaxTokens(200),
		WithSystemPrompt("You are a test assistant."),
		WithTemperature(0.2),
		WithModel("claude-opus-4"),
	)
	if req.MaxTokens != 200 {
		t.Errorf("expected MaxTokens 200, got %d", req.MaxTokens)
	}
	if req.SystemPrompt != "You are a test assistant." {
		t.Errorf("expected system prompt, got %q", req.SystemPrompt)
	}
	if req.Temperature != 0.2 {
		t.Errorf("expected temperature 0.2, got %f", req.Temperature)
	}
	meta, ok := req.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected map metadata, got %T", req.Metadata)
	}
	if meta["preferredModel"] != "claude-opus-4" {
		t.Errorf("expected preferredModel %q, got %v", "claude-opus-4", meta["preferredModel"])
	}
}

func TestTextMessage_AssistantRole(t *testing.T) {
	t.Parallel()
	msg := TextMessage("assistant", "I can help.")
	if string(msg.Role) != "assistant" {
		t.Errorf("expected role %q, got %q", "assistant", msg.Role)
	}
}

func TestTextMessage_EmptyText(t *testing.T) {
	t.Parallel()
	msg := TextMessage("user", "")
	if msg.Content == nil {
		t.Error("expected non-nil content for empty text message")
	}
	content, ok := msg.Content.(registry.Content)
	if !ok {
		t.Fatalf("expected Content to implement registry.Content, got %T", msg.Content)
	}
	text, ok := registry.ExtractTextContent(content)
	if !ok {
		t.Fatal("expected text content")
	}
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}

func TestRequestOption_Independence(t *testing.T) {
	t.Parallel()
	// Options applied to different params structs should not interfere.
	var p1, p2 CreateMessageParams
	opt := WithSystemPrompt("shared option")
	opt(&p1)
	p2.SystemPrompt = "different"
	if p2.SystemPrompt != "different" {
		t.Error("applying option to p1 should not affect p2")
	}
}
