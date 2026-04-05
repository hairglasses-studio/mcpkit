package multi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIAdapter implements Adapter for OpenAI function calling format.
// It translates between OpenAI chat completion tool_calls and the gateway's
// canonical request/response model.
//
// This adapter handles only the tool invocation portion of the chat completions
// API. It does not provide LLM inference. The adapter intercepts tool calls
// from an OpenAI-compatible client, invokes the corresponding mcpkit tools,
// and returns results in the format the client expects.
//
// No OpenAI SDK dependency — only JSON parsing/emission.
//
// OpenAIAdapter is safe for concurrent use.
type OpenAIAdapter struct {
	// modelName is returned in encoded responses (e.g., "mcpkit-gateway").
	modelName string
}

// OpenAIAdapterOption configures an OpenAIAdapter.
type OpenAIAdapterOption func(*OpenAIAdapter)

// WithModelName sets the model name returned in OpenAI-format responses.
func WithModelName(name string) OpenAIAdapterOption {
	return func(a *OpenAIAdapter) {
		a.modelName = name
	}
}

// NewOpenAIAdapter creates an OpenAI function calling adapter.
func NewOpenAIAdapter(opts ...OpenAIAdapterOption) *OpenAIAdapter {
	a := &OpenAIAdapter{
		modelName: "mcpkit-gateway",
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Protocol returns ProtocolOpenAI.
func (a *OpenAIAdapter) Protocol() Protocol {
	return ProtocolOpenAI
}

// Detect inspects an HTTP request and returns whether it looks like an OpenAI
// function calling request. It checks URL path and body structure.
func (a *OpenAIAdapter) Detect(r *http.Request, bodyPeek []byte) (bool, Confidence) {
	path := r.URL.Path

	// Definitive path match: standard OpenAI chat completions endpoint.
	if path == "/v1/chat/completions" || path == "/openai/v1/chat/completions" {
		return true, ConfidenceHigh
	}

	// Path prefix match.
	if strings.HasPrefix(path, "/openai/") || strings.HasPrefix(path, "/v1/chat/") {
		return true, ConfidenceHigh
	}

	// Body structure match: look for tool_calls, function_call, or functions.
	if len(bodyPeek) > 0 && hasOpenAIStructure(bodyPeek) {
		return true, ConfidenceHigh
	}

	return false, ConfidenceLow
}

// openaiRequest represents the relevant parts of an OpenAI chat completions
// request body for tool call extraction.
type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

// openaiMessage represents a single message in the OpenAI messages array.
type openaiMessage struct {
	Role      string           `json:"role"`
	Content   json.RawMessage  `json:"content,omitempty"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`

	// Legacy function_call format (deprecated but still in use).
	FunctionCall *openaiFunctionCall `json:"function_call,omitempty"`

	// Tool result message fields.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// openaiToolCall represents a tool_call entry in an assistant message.
type openaiToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openaiFunctionCall `json:"function"`
}

// openaiFunctionCall represents a function invocation.
type openaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Decode parses an OpenAI-format request and extracts the first tool call
// into a CanonicalRequest. For requests with multiple tool_calls, the first
// one is decoded and the remaining IDs are stored in metadata for the caller
// to handle (e.g., via multiple sequential calls or batching).
func (a *OpenAIAdapter) Decode(r *http.Request) (*CanonicalRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("openai decode: read body: %w", err)
	}

	var req openaiRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("openai decode: parse json: %w", err)
	}

	// Find the last assistant message with tool_calls.
	toolCall, err := extractToolCall(req.Messages)
	if err != nil {
		return nil, err
	}

	// Parse the function arguments JSON string into a map.
	args, err := parseArguments(toolCall.Function.Arguments)
	if err != nil {
		return nil, fmt.Errorf("openai decode: parse arguments for %q: %w", toolCall.Function.Name, err)
	}

	canonical := &CanonicalRequest{
		Protocol:  ProtocolOpenAI,
		ToolName:  toolCall.Function.Name,
		Arguments: args,
		RequestID: toolCall.ID,
		Metadata:  map[string]string{},
	}

	// Store model name in metadata if present.
	if req.Model != "" {
		canonical.Metadata["openai_model"] = req.Model
	}

	// Extract bearer token if present.
	if auth := r.Header.Get("Authorization"); auth != "" {
		if token, ok := strings.CutPrefix(auth, "Bearer "); ok {
			canonical.Auth = &AuthContext{Token: token}
		}
	}

	return canonical, nil
}

// extractToolCall finds the first tool call from the messages.
// It searches for assistant messages with tool_calls (preferred) or
// the legacy function_call field.
func extractToolCall(messages []openaiMessage) (*openaiToolCall, error) {
	// Search from the end — the latest assistant message is most relevant.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}

		// Modern tool_calls format.
		if len(msg.ToolCalls) > 0 {
			return &msg.ToolCalls[0], nil
		}

		// Legacy function_call format: convert to tool_call shape.
		if msg.FunctionCall != nil {
			return &openaiToolCall{
				ID:   "legacy_fc",
				Type: "function",
				Function: openaiFunctionCall{
					Name:      msg.FunctionCall.Name,
					Arguments: msg.FunctionCall.Arguments,
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("openai decode: no tool_calls found in assistant messages")
}

// parseArguments parses a JSON arguments string into a map.
// OpenAI sends arguments as a JSON string, not a raw object.
func parseArguments(argsJSON string) (map[string]any, error) {
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" || argsJSON == "{}" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("invalid arguments json: %w", err)
	}
	return args, nil
}

// openaiToolResultMessage represents a tool result in OpenAI format.
type openaiToolResultMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

// openaiChatCompletion represents a minimal OpenAI chat completion response
// wrapping a tool result message.
type openaiChatCompletion struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
}

// openaiChoice is a single choice in a chat completion response.
type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// Encode wraps a CanonicalResponse as an OpenAI-format tool result message
// embedded in a chat completion response.
func (a *OpenAIAdapter) Encode(resp *CanonicalResponse) ([]byte, string, error) {
	// Build the content string from canonical content parts.
	content := buildContentString(resp)

	// Build the response as a chat completion with a tool message.
	toolCallID := resp.RequestID
	if toolCallID == "" {
		toolCallID = "call_unknown"
	}

	completion := openaiChatCompletion{
		ID:     "chatcmpl-" + toolCallID,
		Object: "chat.completion",
		Model:  a.modelName,
		Choices: []openaiChoice{
			{
				Index: 0,
				Message: openaiMessage{
					Role:       "tool",
					Content:    marshalContent(content),
					ToolCallID: toolCallID,
				},
				FinishReason: "stop",
			},
		},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(completion); err != nil {
		return nil, "", fmt.Errorf("openai encode: %w", err)
	}

	return buf.Bytes(), "application/json", nil
}

// buildContentString concatenates canonical content parts into a single string
// for the OpenAI tool result message content field.
func buildContentString(resp *CanonicalResponse) string {
	if !resp.Success && resp.Error != nil {
		return fmt.Sprintf("Error: %s", resp.Error.Message)
	}

	var parts []string
	for _, c := range resp.Content {
		switch c.Type {
		case ContentTypeText:
			parts = append(parts, c.Text)
		case ContentTypeJSON:
			if c.JSON != nil {
				if b, err := json.Marshal(c.JSON); err == nil {
					parts = append(parts, string(b))
				}
			}
		case ContentTypeData:
			parts = append(parts, fmt.Sprintf("[binary data: %d bytes, type: %s]", len(c.Data), c.MimeType))
		case ContentTypeImage:
			parts = append(parts, fmt.Sprintf("[image: %s]", c.MimeType))
		default:
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// marshalContent converts the content string to a json.RawMessage suitable
// for the openaiMessage Content field.
func marshalContent(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return json.RawMessage(b)
}
