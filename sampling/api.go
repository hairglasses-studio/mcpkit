//go:build !official_sdk

package sampling

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
)

// APISamplingClient implements SamplingClient by calling the Anthropic Messages API directly.
// Use this when the MCP client doesn't support sampling (e.g., Claude Code).
type APISamplingClient struct {
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

func (c *APISamplingClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiResponse struct {
	Content    []apiContent `json:"content"`
	Model      string       `json:"model"`
	StopReason string       `json:"stop_reason"`
	Usage      apiUsage     `json:"usage"`
	Error      *apiError    `json:"error,omitempty"`
}

type apiContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// CreateMessage sends a sampling request to the Anthropic Messages API.
// It validates the request before sending; returns an error if validation fails.
func (c *APISamplingClient) CreateMessage(ctx context.Context, req CreateMessageRequest) (*CreateMessageResult, error) {
	if err := ValidateRequest(req); err != nil {
		return nil, fmt.Errorf("api sampler: invalid request: %w", err)
	}

	model := c.DefaultModel
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	if md, ok := req.Metadata.(map[string]any); ok {
		if pm, ok := md["preferredModel"].(string); ok && pm != "" {
			model = pm
		}
	}

	var msgs []apiMessage
	for _, m := range req.Messages {
		text := ""
		if content, ok := m.Content.(registry.Content); ok {
			if t, ok := registry.ExtractTextContent(content); ok {
				text = t
			}
		}
		if text == "" {
			if s, ok := m.Content.(string); ok {
				text = s
			}
		}
		msgs = append(msgs, apiMessage{
			Role:    string(m.Role),
			Content: text,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	body := apiRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  msgs,
	}

	var resp *apiResponse
	var lastErr error
	for attempt := range 4 {
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
		if !isAPIRateLimitError(lastErr) {
			return nil, lastErr
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("api sampler: all retries exhausted: %w", lastErr)
	}

	text := ""
	for _, ct := range resp.Content {
		if ct.Type == "text" {
			text = ct.Text
			break
		}
	}

	result := &CreateMessageResult{
		Model:      resp.Model,
		StopReason: resp.StopReason,
	}
	result.Role = "assistant"
	result.Content = registry.MakeTextContent(text)

	return result, nil
}

func (c *APISamplingClient) doRequest(ctx context.Context, body apiRequest) (*apiResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("api sampler: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("api sampler: create request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	httpResp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api sampler: http error: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("api sampler: read response: %w", err)
	}

	if httpResp.StatusCode == http.StatusTooManyRequests {
		return nil, &apiRateLimitError{status: httpResp.StatusCode, body: string(respBody)}
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api sampler: API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var resp apiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("api sampler: unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("api sampler: API error: %s: %s", resp.Error.Type, resp.Error.Message)
	}

	return &resp, nil
}

type apiRateLimitError struct {
	status int
	body   string
}

func (e *apiRateLimitError) Error() string {
	return fmt.Sprintf("api sampler: rate limited (status %d): %s", e.status, e.body)
}

func isAPIRateLimitError(err error) bool {
	_, ok := err.(*apiRateLimitError)
	return ok
}
