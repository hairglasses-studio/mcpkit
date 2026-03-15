package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/hairglasses-studio/mcpkit/client"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// PublisherConfig configures the registry publisher.
type PublisherConfig struct {
	// BaseURL is the registry API base URL. Default: DefaultRegistryURL.
	BaseURL string

	// Token is the Bearer token used for authenticated requests. Required.
	Token string

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
}

// Publisher registers and manages server metadata in the MCP Registry.
type Publisher struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewPublisher creates a new Publisher. Returns an error if Token is empty.
func NewPublisher(cfg PublisherConfig) (*Publisher, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discovery: publisher token is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultRegistryURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = client.Standard()
	}
	return &Publisher{
		baseURL:    cfg.BaseURL,
		token:      cfg.Token,
		httpClient: cfg.HTTPClient,
	}, nil
}

// Register publishes new server metadata to the registry via POST /v1/servers.
func (p *Publisher) Register(ctx context.Context, meta ServerMetadata) (ServerMetadata, error) {
	return p.doJSON(ctx, http.MethodPost, p.baseURL+"/v1/servers", meta)
}

// Update updates existing server metadata in the registry via PUT /v1/servers/{id}.
func (p *Publisher) Update(ctx context.Context, id string, meta ServerMetadata) (ServerMetadata, error) {
	reqURL := p.baseURL + "/v1/servers/" + url.PathEscape(id)
	return p.doJSON(ctx, http.MethodPut, reqURL, meta)
}

// Deregister removes a server from the registry via DELETE /v1/servers/{id}.
func (p *Publisher) Deregister(ctx context.Context, id string) error {
	reqURL := p.baseURL + "/v1/servers/" + url.PathEscape(id)

	body, err := json.Marshal(struct{}{})
	if err != nil {
		return fmt.Errorf("discovery: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discovery: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("discovery: http request: %w", err)
	}
	defer resp.Body.Close()

	return mapStatusError(resp.StatusCode, reqURL)
}

// doJSON encodes payload as JSON, sends it with method to reqURL, and decodes
// the response body into a ServerMetadata. Sets the Authorization header.
func (p *Publisher) doJSON(ctx context.Context, method, reqURL string, payload any) (ServerMetadata, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ServerMetadata{}, fmt.Errorf("discovery: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(encoded))
	if err != nil {
		return ServerMetadata{}, fmt.Errorf("discovery: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ServerMetadata{}, fmt.Errorf("discovery: http request: %w", err)
	}
	defer resp.Body.Close()

	if err := mapStatusError(resp.StatusCode, reqURL); err != nil {
		return ServerMetadata{}, err
	}

	var result ServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ServerMetadata{}, fmt.Errorf("discovery: decode response: %w", err)
	}
	return result, nil
}

// Publish is a convenience function that creates a Publisher and registers the
// provided ServerMetadata in one call. It is equivalent to:
//
//	p, err := NewPublisher(PublisherConfig{BaseURL: registryURL, Token: token})
//	if err != nil { return ServerMetadata{}, err }
//	return p.Register(ctx, meta)
func Publish(ctx context.Context, registryURL, token string, meta ServerMetadata) (ServerMetadata, error) {
	p, err := NewPublisher(PublisherConfig{BaseURL: registryURL, Token: token})
	if err != nil {
		return ServerMetadata{}, err
	}
	return p.Register(ctx, meta)
}

// Unpublish is a convenience function that creates a Publisher and removes the
// server with the given ID from the registry in one call. It is equivalent to:
//
//	p, err := NewPublisher(PublisherConfig{BaseURL: registryURL, Token: token})
//	if err != nil { return err }
//	return p.Deregister(ctx, serverID)
func Unpublish(ctx context.Context, registryURL, token string, serverID string) error {
	p, err := NewPublisher(PublisherConfig{BaseURL: registryURL, Token: token})
	if err != nil {
		return err
	}
	return p.Deregister(ctx, serverID)
}

// MetadataFromRegistry builds a ServerMetadata from a ToolRegistry and the
// provided server-level fields. It extracts tool names and descriptions from
// all registered tools in the registry.
func MetadataFromRegistry(name, desc string, reg *registry.ToolRegistry, transports []TransportInfo) ServerMetadata {
	defs := reg.GetAllToolDefinitions()

	// Sort for deterministic output.
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Tool.Name < defs[j].Tool.Name
	})

	tools := make([]ToolSummary, 0, len(defs))
	for _, td := range defs {
		tools = append(tools, ToolSummary{
			Name:        td.Tool.Name,
			Description: td.Tool.Description,
		})
	}

	return ServerMetadata{
		Name:        name,
		Description: desc,
		Tools:       tools,
		Transports:  transports,
	}
}
