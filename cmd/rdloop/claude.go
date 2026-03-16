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
	"strconv"
	"strings"
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
			Timeout: 10 * time.Minute, // generous for long implement prompts
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

	// Retry with exponential backoff on transient errors (429, 5xx, network).
	var resp *claudeResponse
	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			// Cap backoff at 60s to avoid long waits.
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			// Use Retry-After header if available.
			if rle, ok := lastErr.(*transientError); ok && rle.retryAfter > 0 {
				backoff = rle.retryAfter
			}
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
		// Only retry on transient errors (429, 5xx, network timeouts).
		if !isTransientError(lastErr) {
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

	// Transient: 429 (rate limit), 500, 502, 503, 529 (overloaded).
	if httpResp.StatusCode == http.StatusTooManyRequests ||
		httpResp.StatusCode >= 500 {
		retryAfter := parseRetryAfter(httpResp.Header.Get("Retry-After"))
		return nil, &transientError{
			status:     httpResp.StatusCode,
			body:       string(respBody),
			retryAfter: retryAfter,
		}
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

// transientError represents a retryable API error (429, 5xx, network).
type transientError struct {
	status     int
	body       string
	retryAfter time.Duration
}

func (e *transientError) Error() string {
	return fmt.Sprintf("claude: transient error (status %d): %s", e.status, e.body)
}

func isTransientError(err error) bool {
	if _, ok := err.(*transientError); ok {
		return true
	}
	// Also retry on context-independent network errors (connection refused, reset, etc.)
	// but NOT on context.Canceled or context.DeadlineExceeded.
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	// Network errors from http.Client (DNS, TCP, TLS) are wrapped — retry them.
	errStr := err.Error()
	for _, sub := range []string{"connection refused", "connection reset", "EOF", "i/o timeout", "no such host"} {
		if strings.Contains(errStr, sub) {
			return true
		}
	}
	return false
}

// parseRetryAfter parses the Retry-After header value (seconds).
func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 0
	}
	secs, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	d := time.Duration(secs) * time.Second
	// Cap at 2 minutes to avoid extreme waits.
	if d > 2*time.Minute {
		d = 2 * time.Minute
	}
	return d
}
