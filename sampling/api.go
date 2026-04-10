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
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/finops"
	"github.com/hairglasses-studio/mcpkit/registry"
)

const defaultAnthropicBaseURL = "https://api.anthropic.com"

// APISamplingClient implements SamplingClient by calling an Anthropic-compatible
// Messages API directly. The default target is Anthropic's hosted API, but the
// client can also point at local-compatible backends such as Ollama.
type APISamplingClient struct {
	APIKey             string
	DefaultModel       string
	BaseURL            string
	AuthHeaderStrategy string
	HTTPClient         *http.Client
}

func (c *APISamplingClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (c *APISamplingClient) baseURL() string {
	baseURL := strings.TrimSpace(c.BaseURL)
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func (c *APISamplingClient) messagesURL() string {
	baseURL := c.baseURL()
	switch {
	case strings.HasSuffix(baseURL, "/messages"):
		return baseURL
	case strings.HasSuffix(baseURL, "/v1"):
		return baseURL + "/messages"
	default:
		return baseURL + "/v1/messages"
	}
}

func (c *APISamplingClient) resolvedAPIKey() string {
	if apiKey := strings.TrimSpace(c.APIKey); apiKey != "" {
		return apiKey
	}
	if isLikelyOllamaBaseURL(c.baseURL()) {
		if apiKey := strings.TrimSpace(os.Getenv("OLLAMA_API_KEY")); apiKey != "" {
			return apiKey
		}
		return "ollama"
	}
	return ""
}

func (c *APISamplingClient) authHeaderStrategy() string {
	if strategy := strings.TrimSpace(c.AuthHeaderStrategy); strategy != "" {
		return strings.ToLower(strategy)
	}
	if isLikelyOllamaBaseURL(c.baseURL()) {
		return "both"
	}
	return "anthropic"
}

func isLikelyOllamaBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return true
	}
	if strings.Contains(host, "ollama") {
		return true
	}
	return parsed.Port() == "11434"
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

// CreateMessage sends a sampling request to the configured Messages API.
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

	ctx, span := startLLMSpan(ctx, llmSpanConfig{
		System:    c.genAISystem(),
		Operation: "chat",
		Model:     model,
		BaseURL:   c.baseURL(),
	})
	var (
		usage      finops.TokenUsage
		stopReason string
		attempts   int
		callErr    error
	)
	defer func() {
		finishLLMSpan(ctx, span, usage, stopReason, attempts, callErr)
	}()

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
		attempts = attempt + 1
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				callErr = ctx.Err()
				return nil, callErr
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
		callErr = fmt.Errorf("api sampler: all retries exhausted: %w", lastErr)
		return nil, callErr
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
	stopReason = resp.StopReason
	usage = finops.TokenUsage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		Model:        resp.Model,
	}

	return result, nil
}

func (c *APISamplingClient) genAISystem() string {
	if isLikelyOllamaBaseURL(c.baseURL()) {
		return "ollama"
	}
	return "anthropic"
}

func (c *APISamplingClient) doRequest(ctx context.Context, body apiRequest) (*apiResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("api sampler: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("api sampler: create request: %w", err)
	}
	apiKey := c.resolvedAPIKey()
	switch c.authHeaderStrategy() {
	case "authorization", "bearer":
		if apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}
	case "both":
		if apiKey != "" {
			httpReq.Header.Set("x-api-key", apiKey)
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}
	case "none":
		// Deliberately no auth header.
	default:
		if apiKey != "" {
			httpReq.Header.Set("x-api-key", apiKey)
		}
	}
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
