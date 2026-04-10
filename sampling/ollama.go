//go:build !official_sdk

package sampling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultOllamaBaseURL = "http://127.0.0.1:11434"

// NativeOllamaClient provides direct access to Ollama's native /api surface.
type NativeOllamaClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NativeOllamaModelDetails describes shared model metadata from /api/tags, /api/ps, and /api/show.
type NativeOllamaModelDetails struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// NativeOllamaModel describes an installed model returned by /api/tags.
type NativeOllamaModel struct {
	Name       string                   `json:"name"`
	Model      string                   `json:"model"`
	ModifiedAt time.Time                `json:"modified_at"`
	Size       int64                    `json:"size"`
	Digest     string                   `json:"digest"`
	Details    NativeOllamaModelDetails `json:"details"`
}

// NativeOllamaRunningModel describes a loaded model returned by /api/ps.
type NativeOllamaRunningModel struct {
	Name          string                   `json:"name"`
	Model         string                   `json:"model"`
	Size          int64                    `json:"size"`
	Digest        string                   `json:"digest"`
	Details       NativeOllamaModelDetails `json:"details"`
	ExpiresAt     string                   `json:"expires_at,omitempty"`
	SizeVRAM      int64                    `json:"size_vram,omitempty"`
	ContextLength int                      `json:"context_length,omitempty"`
}

// NativeOllamaToolFunction describes a tool definition or tool call.
type NativeOllamaToolFunction struct {
	Index       int                    `json:"index,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Arguments   map[string]interface{} `json:"arguments,omitempty"`
}

// NativeOllamaTool describes a tool schema or tool call.
type NativeOllamaTool struct {
	Type     string                   `json:"type"`
	Function NativeOllamaToolFunction `json:"function"`
}

// NativeOllamaGenerateRequest represents a native /api/generate request.
type NativeOllamaGenerateRequest struct {
	Model     string                 `json:"model"`
	Prompt    string                 `json:"prompt"`
	System    string                 `json:"system,omitempty"`
	Stream    bool                   `json:"stream"`
	KeepAlive string                 `json:"keep_alive,omitempty"`
	Format    interface{}            `json:"format,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

// NativeOllamaGenerateResponse represents a native /api/generate response.
type NativeOllamaGenerateResponse struct {
	Model              string `json:"model"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration,omitempty"`
	LoadDuration       int64  `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64  `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int64  `json:"eval_duration,omitempty"`
}

// NativeOllamaChatMessage represents one native /api/chat message.
type NativeOllamaChatMessage struct {
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	ToolName  string             `json:"tool_name,omitempty"`
	ToolCalls []NativeOllamaTool `json:"tool_calls,omitempty"`
}

// NativeOllamaChatRequest represents a native /api/chat request.
type NativeOllamaChatRequest struct {
	Model     string                    `json:"model"`
	Messages  []NativeOllamaChatMessage `json:"messages"`
	Stream    bool                      `json:"stream"`
	KeepAlive string                    `json:"keep_alive,omitempty"`
	Format    interface{}               `json:"format,omitempty"`
	Tools     []NativeOllamaTool        `json:"tools,omitempty"`
	Options   map[string]interface{}    `json:"options,omitempty"`
}

// NativeOllamaChatResponse represents a native /api/chat response.
type NativeOllamaChatResponse struct {
	Model              string                  `json:"model"`
	Message            NativeOllamaChatMessage `json:"message"`
	Done               bool                    `json:"done"`
	CreatedAt          time.Time               `json:"created_at"`
	TotalDuration      int64                   `json:"total_duration,omitempty"`
	LoadDuration       int64                   `json:"load_duration,omitempty"`
	PromptEvalCount    int                     `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64                   `json:"prompt_eval_duration,omitempty"`
	EvalCount          int                     `json:"eval_count,omitempty"`
	EvalDuration       int64                   `json:"eval_duration,omitempty"`
}

func (c *NativeOllamaClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (c *NativeOllamaClient) baseURL() string {
	baseURL := strings.TrimSpace(c.BaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	return baseURL
}

func (c *NativeOllamaClient) resolvedAPIKey() string {
	if apiKey := strings.TrimSpace(c.APIKey); apiKey != "" {
		return apiKey
	}
	if apiKey := strings.TrimSpace(os.Getenv("OLLAMA_API_KEY")); apiKey != "" {
		return apiKey
	}
	if isLikelyOllamaBaseURL(c.baseURL()) {
		return "ollama"
	}
	return ""
}

func (c *NativeOllamaClient) doJSON(ctx context.Context, method, path string, requestBody, responseBody interface{}) error {
	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("native ollama client: marshal %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, body)
	if err != nil {
		return fmt.Errorf("native ollama client: build %s %s: %w", method, path, err)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey := c.resolvedAPIKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("native ollama client: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("native ollama client: %s %s returned HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if responseBody == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
		return fmt.Errorf("native ollama client: decode %s %s: %w", method, path, err)
	}
	return nil
}

// ListModels returns the installed models from /api/tags.
func (c *NativeOllamaClient) ListModels(ctx context.Context) ([]NativeOllamaModel, error) {
	var response struct {
		Models []NativeOllamaModel `json:"models"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/tags", nil, &response); err != nil {
		return nil, err
	}
	return response.Models, nil
}

// ListRunningModels returns the loaded models from /api/ps.
func (c *NativeOllamaClient) ListRunningModels(ctx context.Context) ([]NativeOllamaRunningModel, error) {
	var response struct {
		Models []NativeOllamaRunningModel `json:"models"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/ps", nil, &response); err != nil {
		return nil, err
	}
	return response.Models, nil
}

// ShowModel returns model metadata from /api/show.
func (c *NativeOllamaClient) ShowModel(ctx context.Context, model string) (map[string]interface{}, error) {
	var response map[string]interface{}
	if err := c.doJSON(ctx, http.MethodPost, "/api/show", map[string]string{"name": model}, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// Generate calls /api/generate with native Ollama options.
func (c *NativeOllamaClient) Generate(ctx context.Context, req NativeOllamaGenerateRequest) (*NativeOllamaGenerateResponse, error) {
	req.Stream = false
	var response NativeOllamaGenerateResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/generate", req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// Chat calls /api/chat with native Ollama options.
func (c *NativeOllamaClient) Chat(ctx context.Context, req NativeOllamaChatRequest) (*NativeOllamaChatResponse, error) {
	req.Stream = false
	var response NativeOllamaChatResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/chat", req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
