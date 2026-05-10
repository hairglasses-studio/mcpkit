package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client sends tasks to remote A2A agents.
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
}

// NewClient creates an A2A client for the given agent URL.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ClientOption configures the A2A client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithAuthToken sets a Bearer token for authentication.
func WithAuthToken(token string) ClientOption {
	return func(c *Client) { c.authToken = token }
}

// GetAgentCard fetches the agent card from /.well-known/agent.json.
func (c *Client) GetAgentCard(ctx context.Context) (*AgentCard, error) {
	url := c.baseURL + "/.well-known/agent.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("agent card: status %d: %s", resp.StatusCode, string(body))
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decode agent card: %w", err)
	}
	return &card, nil
}

// SendTask sends a task to the A2A agent and returns the response.
func (c *Client) SendTask(ctx context.Context, params TaskSendParams) (*Task, error) {
	return c.call(ctx, "tasks/send", params)
}

// GetTask queries the current state of a task.
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return c.call(ctx, "tasks/get", TaskQueryParams{ID: taskID})
}

// CancelTask requests cancellation of a task.
func (c *Client) CancelTask(ctx context.Context, taskID string) (*Task, error) {
	return c.call(ctx, "tasks/cancel", TaskQueryParams{ID: taskID})
}

// CreateTaskPushNotificationConfig creates a webhook notification config.
func (c *Client) CreateTaskPushNotificationConfig(ctx context.Context, config PushNotificationConfig) (*PushNotificationConfig, error) {
	var result PushNotificationConfig
	if err := c.callRPC(ctx, "CreateTaskPushNotificationConfig", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTaskPushNotificationConfig retrieves one webhook notification config.
func (c *Client) GetTaskPushNotificationConfig(ctx context.Context, taskID, configID string) (*PushNotificationConfig, error) {
	var result PushNotificationConfig
	err := c.callRPC(ctx, "GetTaskPushNotificationConfig", GetTaskPushNotificationConfigParams{
		TaskID: taskID,
		ID:     configID,
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTaskPushNotificationConfigs lists webhook notification configs for a task.
func (c *Client) ListTaskPushNotificationConfigs(ctx context.Context, taskID string) (*ListTaskPushNotificationConfigsResponse, error) {
	var result ListTaskPushNotificationConfigsResponse
	err := c.callRPC(ctx, "ListTaskPushNotificationConfigs", ListTaskPushNotificationConfigsParams{
		TaskID: taskID,
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteTaskPushNotificationConfig deletes one webhook notification config.
func (c *Client) DeleteTaskPushNotificationConfig(ctx context.Context, taskID, configID string) error {
	var result DeleteTaskPushNotificationConfigResult
	return c.callRPC(ctx, "DeleteTaskPushNotificationConfig", DeleteTaskPushNotificationConfigParams{
		TaskID: taskID,
		ID:     configID,
	}, &result)
}

// call sends a JSON-RPC request and decodes the Task from the result.
func (c *Client) call(ctx context.Context, method string, params interface{}) (*Task, error) {
	var task Task
	if err := c.callRPC(ctx, method, params, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (c *Client) callRPC(ctx context.Context, method string, params interface{}, result interface{}) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  paramsJSON,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return fmt.Errorf("decode response: %w (body: %s)", err, string(respBody[:min(len(respBody), 200)]))
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("A2A error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if result == nil {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, result); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
}
