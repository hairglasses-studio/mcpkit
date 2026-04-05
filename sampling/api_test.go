//go:build !official_sdk

package sampling

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// redirectTransport rewrites all requests to point at the test server URL,
// then delegates to the test server's default transport. This lets us test
// doRequest (which hardcodes the Anthropic API URL) against a local httptest server.
type redirectTransport struct {
	targetURL string
	inner     http.RoundTripper
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	// Parse the test server URL to get host.
	req.URL.Host = strings.TrimPrefix(rt.targetURL, "http://")
	return rt.inner.RoundTrip(req)
}

// testHTTPClient returns an *http.Client that redirects all requests to the
// given httptest.Server.
func testHTTPClient(ts *httptest.Server) *http.Client {
	return &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			inner:     http.DefaultTransport,
		},
		Timeout: 10 * time.Second,
	}
}

func TestAPISamplingClient_CreateMessage_Success(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers.
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key=test-key, got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("content-type") != "application/json" {
			t.Errorf("expected content-type=application/json, got %q", r.Header.Get("content-type"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version, got %q", r.Header.Get("anthropic-version"))
		}

		// Decode the request to verify structure.
		var req apiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Model != "claude-sonnet-4-6" {
			t.Errorf("expected default model claude-sonnet-4-6, got %q", req.Model)
		}
		// CompletionRequest sets default of 1024, so that's what arrives.
		if req.MaxTokens != 1024 {
			t.Errorf("expected MaxTokens 1024, got %d", req.MaxTokens)
		}

		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "Hello from test"}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
			Usage:      apiUsage{InputTokens: 10, OutputTokens: 5},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	msgs := []SamplingMessage{TextMessage("user", "test")}
	req := CompletionRequest(msgs)

	result, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if result.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %q", result.Model)
	}
	if result.StopReason != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %q", result.StopReason)
	}
	if string(result.Role) != "assistant" {
		t.Errorf("expected role assistant, got %q", result.Role)
	}
}

func TestAPISamplingClient_CreateMessage_CustomModel(t *testing.T) {
	t.Parallel()

	var receivedModel string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req apiRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "ok"}},
			Model:      req.Model,
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:       "test-key",
		DefaultModel: "claude-opus-4",
		HTTPClient:   testHTTPClient(ts),
	}

	msgs := []SamplingMessage{TextMessage("user", "test")}
	req := CompletionRequest(msgs)

	_, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if receivedModel != "claude-opus-4" {
		t.Errorf("expected model claude-opus-4, got %q", receivedModel)
	}
}

func TestAPISamplingClient_CreateMessage_ModelFromMetadata(t *testing.T) {
	t.Parallel()

	var receivedModel string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req apiRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "ok"}},
			Model:      req.Model,
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:       "test-key",
		DefaultModel: "should-be-overridden",
		HTTPClient:   testHTTPClient(ts),
	}

	msgs := []SamplingMessage{TextMessage("user", "test")}
	req := CompletionRequest(msgs, WithModel("claude-3-5-haiku"))

	_, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if receivedModel != "claude-3-5-haiku" {
		t.Errorf("expected model claude-3-5-haiku, got %q", receivedModel)
	}
}

func TestAPISamplingClient_CreateMessage_SystemPrompt(t *testing.T) {
	t.Parallel()

	var receivedSystem string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req apiRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedSystem = req.System
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "ok"}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	msgs := []SamplingMessage{TextMessage("user", "hello")}
	req := CompletionRequest(msgs, WithSystemPrompt("You are a test assistant."))

	_, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if receivedSystem != "You are a test assistant." {
		t.Errorf("expected system prompt, got %q", receivedSystem)
	}
}

func TestAPISamplingClient_CreateMessage_MaxTokensDefault(t *testing.T) {
	t.Parallel()

	var receivedMaxTokens int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req apiRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedMaxTokens = req.MaxTokens
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "ok"}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	// maxTokens 0 should be clamped to 4096.
	req := CreateMessageRequest{}
	req.MaxTokens = 0
	req.Messages = []SamplingMessage{TextMessage("user", "hi")}

	_, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if receivedMaxTokens != 4096 {
		t.Errorf("expected maxTokens clamped to 4096, got %d", receivedMaxTokens)
	}
}

func TestAPISamplingClient_CreateMessage_MultipleMessages(t *testing.T) {
	t.Parallel()

	var receivedMsgs []apiMessage
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req apiRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedMsgs = req.Messages
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "ok"}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	msgs := []SamplingMessage{
		TextMessage("user", "What is Go?"),
		TextMessage("assistant", "Go is a programming language."),
		TextMessage("user", "Tell me more."),
	}
	req := CompletionRequest(msgs)

	_, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if len(receivedMsgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(receivedMsgs))
	}
	if receivedMsgs[0].Role != "user" || receivedMsgs[1].Role != "assistant" {
		t.Errorf("unexpected roles: %v", receivedMsgs)
	}
}

func TestAPISamplingClient_HttpClient_Default(t *testing.T) {
	t.Parallel()

	client := &APISamplingClient{
		APIKey: "test",
	}
	hc := client.httpClient()
	if hc == nil {
		t.Fatal("expected non-nil default HTTP client")
	}
	if hc.Timeout != 5*time.Minute {
		t.Errorf("expected 5m timeout, got %v", hc.Timeout)
	}
}

func TestAPISamplingClient_HttpClient_Custom(t *testing.T) {
	t.Parallel()

	custom := &http.Client{Timeout: 30 * time.Second}
	client := &APISamplingClient{
		APIKey:     "test",
		HTTPClient: custom,
	}
	if client.httpClient() != custom {
		t.Error("expected custom HTTP client to be returned")
	}
}

func TestAPISamplingClient_RateLimitRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "success after retry"}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	body := apiRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  []apiMessage{{Role: "user", Content: "test"}},
	}

	// First call: rate limited.
	resp1, err1 := client.doRequest(context.Background(), body)
	if resp1 != nil {
		t.Error("expected nil response on rate limit")
	}
	if err1 == nil {
		t.Fatal("expected rate limit error")
	}
	if !isAPIRateLimitError(err1) {
		t.Errorf("expected rate limit error type, got %T: %v", err1, err1)
	}

	// Second call: also rate limited.
	resp2, err2 := client.doRequest(context.Background(), body)
	if resp2 != nil {
		t.Error("expected nil response on rate limit")
	}
	if !isAPIRateLimitError(err2) {
		t.Error("expected rate limit error on second attempt")
	}

	// Third call: succeeds.
	resp3, err3 := client.doRequest(context.Background(), body)
	if err3 != nil {
		t.Fatalf("expected success on third attempt, got: %v", err3)
	}
	if resp3 == nil {
		t.Fatal("expected non-nil response")
	}

	totalAttempts := attempts.Load()
	if totalAttempts != 3 {
		t.Errorf("expected 3 total attempts, got %d", totalAttempts)
	}
}

func TestAPISamplingClient_NonRateLimitError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	body := apiRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  []apiMessage{{Role: "user", Content: "test"}},
	}

	resp, err := client.doRequest(context.Background(), body)
	if resp != nil {
		t.Error("expected nil response on error")
	}
	if err == nil {
		t.Fatal("expected error for 400 status")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error to mention 400, got: %v", err)
	}
	if isAPIRateLimitError(err) {
		t.Error("400 error should not be classified as rate limit")
	}
}

func TestAPISamplingClient_APIResponseError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Error: &apiError{Type: "invalid_request_error", Message: "bad param"},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	body := apiRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  []apiMessage{{Role: "user", Content: "test"}},
	}

	resp, err := client.doRequest(context.Background(), body)
	if resp != nil {
		t.Error("expected nil response on API error")
	}
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "invalid_request_error") {
		t.Errorf("expected error to mention error type, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bad param") {
		t.Errorf("expected error to mention message, got: %v", err)
	}
}

func TestAPISamplingClient_DoRequest_Success(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "response text"}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
			Usage:      apiUsage{InputTokens: 10, OutputTokens: 20},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	body := apiRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  []apiMessage{{Role: "user", Content: "hello"}},
	}

	resp, err := client.doRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	if resp.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %q", resp.Model)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %q", resp.StopReason)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "response text" {
		t.Errorf("unexpected content: %v", resp.Content)
	}
}

func TestAPIRateLimitError_Error(t *testing.T) {
	t.Parallel()

	e := &apiRateLimitError{status: 429, body: "rate limited"}
	msg := e.Error()
	if !strings.Contains(msg, "429") {
		t.Errorf("expected status in error message, got: %s", msg)
	}
	if !strings.Contains(msg, "rate limited") {
		t.Errorf("expected body in error message, got: %s", msg)
	}
}

func TestIsAPIRateLimitError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"rate limit error", &apiRateLimitError{status: 429}, true},
		{"generic error", context.DeadlineExceeded, false},
		{"cancelled", context.Canceled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isAPIRateLimitError(tt.err); got != tt.expected {
				t.Errorf("isAPIRateLimitError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestAPISamplingClient_DoRequest_InvalidJSON(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	body := apiRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  []apiMessage{{Role: "user", Content: "test"}},
	}

	resp, err := client.doRequest(context.Background(), body)
	if resp != nil {
		t.Error("expected nil response for invalid JSON")
	}
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestAPISamplingClient_DoRequest_NoTextContent(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Content:    []apiContent{{Type: "image", Text: ""}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	body := apiRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  []apiMessage{{Role: "user", Content: "test"}},
	}

	resp, err := client.doRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestAPISamplingClient_DoRequest_EmptyContent(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Content:    []apiContent{},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	body := apiRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		Messages:  []apiMessage{{Role: "user", Content: "test"}},
	}

	resp, err := client.doRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Content) != 0 {
		t.Errorf("expected empty content, got %v", resp.Content)
	}
}

func TestAPISamplingClient_CreateMessage_StringContent(t *testing.T) {
	t.Parallel()

	var receivedMsgs []apiMessage
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req apiRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedMsgs = req.Messages
		resp := apiResponse{
			Content:    []apiContent{{Type: "text", Text: "ok"}},
			Model:      "claude-sonnet-4-6",
			StopReason: "end_turn",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &APISamplingClient{
		APIKey:     "test-key",
		HTTPClient: testHTTPClient(ts),
	}

	// Create a request with raw string content (exercises the string fallback path).
	req := CreateMessageRequest{}
	req.Messages = []SamplingMessage{
		{Role: "user", Content: "raw string content"},
	}
	req.MaxTokens = 256

	_, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if len(receivedMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(receivedMsgs))
	}
	if receivedMsgs[0].Content != "raw string content" {
		t.Errorf("expected raw string content, got %q", receivedMsgs[0].Content)
	}
}
