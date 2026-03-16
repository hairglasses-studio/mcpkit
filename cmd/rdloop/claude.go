//go:build !official_sdk

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// ClaudeClient implements sampling.SamplingClient by calling the Anthropic Messages API.
type ClaudeClient struct {
	apiKey     string
	httpClient *http.Client
	model      string
}

// NewClaudeClient creates a client for the Anthropic Messages API.
func NewClaudeClient(apiKey, model string) *ClaudeClient {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &ClaudeClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		model: model,
	}
}

// claudeRequest is the request body for POST /v1/messages.
type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeResponse is the response body from POST /v1/messages.
type claudeResponse struct {
	Content    []claudeContent `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	Usage      claudeUsage     `json:"usage"`
	Error      *claudeError    `json:"error,omitempty"`
}

type claudeContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// CreateMessage sends a sampling request to the Anthropic Messages API.
func (c *ClaudeClient) CreateMessage(ctx context.Context, req sampling.CreateMessageRequest) (*sampling.CreateMessageResult, error) {
	// Determine model: check metadata preferredModel override, then default.
	model := c.model
	if md, ok := req.Metadata.(map[string]any); ok {
		if pm, ok := md["preferredModel"].(string); ok && pm != "" {
			model = pm
		}
	}

	// Extract text from each SamplingMessage.
	var msgs []claudeMessage
	for _, m := range req.Messages {
		text := ""
		if content, ok := m.Content.(registry.Content); ok {
			if t, ok := registry.ExtractTextContent(content); ok {
				text = t
			}
		}
		if text == "" {
			// Try string content directly.
			if s, ok := m.Content.(string); ok {
				text = s
			}
		}
		msgs = append(msgs, claudeMessage{
			Role:    string(m.Role),
			Content: text,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	body := claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  msgs,
	}

	// Retry with exponential backoff on 429.
	var resp *claudeResponse
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, lastErr = c.doRequest(ctx, body)
		if lastErr == nil {
			break
		}
		// Only retry on rate limit errors.
		if !isRateLimitError(lastErr) {
			return nil, lastErr
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("claude: all retries exhausted: %w", lastErr)
	}

	// Extract text from response.
	text := ""
	for _, c := range resp.Content {
		if c.Type == "text" {
			text = c.Text
			break
		}
	}

	result := &sampling.CreateMessageResult{
		Model:      resp.Model,
		StopReason: resp.StopReason,
	}
	result.Role = "assistant"
	result.Content = registry.MakeTextContent(text)

	return result, nil
}

func (c *ClaudeClient) doRequest(ctx context.Context, body claudeRequest) (*claudeResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("claude: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("claude: create request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude: http error: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("claude: read response: %w", err)
	}

	if httpResp.StatusCode == http.StatusTooManyRequests {
		return nil, &rateLimitError{status: httpResp.StatusCode, body: string(respBody)}
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude: API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var resp claudeResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("claude: unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("claude: API error: %s: %s", resp.Error.Type, resp.Error.Message)
	}

	return &resp, nil
}

type rateLimitError struct {
	status int
	body   string
}

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("claude: rate limited (status %d): %s", e.status, e.body)
}

func isRateLimitError(err error) bool {
	_, ok := err.(*rateLimitError)
	return ok
}
