package adapter

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// Protocol identifies a protocol type.
type Protocol string

const (
	ProtocolMCP     Protocol = "mcp"
	ProtocolA2A     Protocol = "a2a"
	ProtocolGRPC    Protocol = "grpc"
	ProtocolOpenAPI Protocol = "openapi"
)

// ProtocolAdapter abstracts protocol-specific connection, discovery, and
// tool invocation. The gateway uses this interface to treat all upstreams
// uniformly regardless of their native protocol.
//
// Contract:
//   - Connect must be called before DiscoverTools or CallTool
//   - CallTool must return (*mcp.CallToolResult, nil) per MCP protocol
//   - Healthy must complete quickly (< 10s timeout recommended)
//   - Close must be idempotent
type ProtocolAdapter interface {
	// Protocol returns the adapter's protocol identifier.
	Protocol() Protocol

	// Connect establishes a connection to the backend.
	Connect(ctx context.Context) error

	// DiscoverTools lists all tools available from the backend as MCP tools.
	// The gateway applies AllowedTools filtering after this call.
	DiscoverTools(ctx context.Context) ([]mcp.Tool, error)

	// CallTool invokes a tool on the backend. Takes the original (non-namespaced)
	// tool name and arguments. Returns MCP result — errors go in content, not Go error.
	CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error)

	// Healthy returns nil if the backend is reachable. Called periodically
	// by the gateway's health loop.
	Healthy(ctx context.Context) error

	// Close tears down the connection. Must be idempotent.
	Close() error
}

// Factory creates a ProtocolAdapter from configuration.
type Factory func(ctx context.Context, cfg Config) (ProtocolAdapter, error)

// Config holds protocol-agnostic adapter configuration.
type Config struct {
	// Protocol specifies which adapter to use.
	Protocol Protocol `json:"protocol"`

	// Name is the namespace prefix for tools from this upstream.
	Name string `json:"name"`

	// URL is the endpoint to connect to.
	URL string `json:"url"`

	// AllowedTools limits the upstream surface to an explicit subset.
	// Empty means all tools are allowed.
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// Auth holds authentication configuration.
	Auth *AuthConfig `json:"auth,omitempty"`

	// Params holds protocol-specific configuration.
	Params map[string]interface{} `json:"params,omitempty"`
}

// AuthConfig specifies how to authenticate with the backend.
type AuthConfig struct {
	// Type: "none", "bearer", "api_key", "oauth2", "mtls"
	Type string `json:"type"`

	// Token for bearer auth.
	Token string `json:"token,omitempty"`

	// Header name for API key auth (default: "X-API-Key").
	Header string `json:"header,omitempty"`

	// OAuth2 fields.
	TokenEndpoint string   `json:"token_endpoint,omitempty"`
	ClientID      string   `json:"client_id,omitempty"`
	ClientSecret  string   `json:"client_secret,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
}

// Registry maps protocol names to adapter factories.
type Registry struct {
	factories map[Protocol]Factory
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[Protocol]Factory),
	}
}

// Register adds a factory for a protocol.
func (r *Registry) Register(protocol Protocol, factory Factory) {
	r.factories[protocol] = factory
}

// Create instantiates an adapter for the given config.
func (r *Registry) Create(ctx context.Context, cfg Config) (ProtocolAdapter, error) {
	factory, ok := r.factories[cfg.Protocol]
	if !ok {
		return nil, &UnsupportedProtocolError{Protocol: cfg.Protocol}
	}
	return factory(ctx, cfg)
}

// Has returns true if a factory is registered for the protocol.
func (r *Registry) Has(protocol Protocol) bool {
	_, ok := r.factories[protocol]
	return ok
}

// Protocols returns all registered protocol names.
func (r *Registry) Protocols() []Protocol {
	protocols := make([]Protocol, 0, len(r.factories))
	for p := range r.factories {
		protocols = append(protocols, p)
	}
	return protocols
}

// UnsupportedProtocolError is returned when no factory exists for a protocol.
type UnsupportedProtocolError struct {
	Protocol Protocol
}

func (e *UnsupportedProtocolError) Error() string {
	return "unsupported protocol: " + string(e.Protocol)
}
