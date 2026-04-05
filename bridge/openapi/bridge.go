package openapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// DefaultTimeout for upstream HTTP requests.
const DefaultTimeout = 30 * time.Second

// BridgeConfig configures the OpenAPI-to-MCP bridge.
type BridgeConfig struct {
	// BaseURL overrides the spec's server URL for all requests.
	// If empty, the first server URL from the OpenAPI spec is used.
	BaseURL string

	// NameStyle controls tool name generation.
	// "operationId" (default): use the operation's operationId.
	// "path_method": generate from HTTP method + path (e.g., get_pets_id).
	NameStyle string

	// Timeout for upstream HTTP requests. Default: 30s.
	Timeout time.Duration

	// AuthHeader is the header name for authentication (e.g., "Authorization").
	AuthHeader string

	// AuthToken is the token value sent in AuthHeader (e.g., "Bearer sk-...").
	AuthToken string

	// Client is an optional HTTP client. If nil, a default client is created.
	Client *http.Client
}

// Bridge auto-generates MCP tools from an OpenAPI v3 specification and proxies
// tool calls to the upstream REST API.
type Bridge struct {
	spec     *openapi3.T
	registry *registry.ToolRegistry
	client   *http.Client
	baseURL  string
	config   BridgeConfig
}

// NewBridge creates a Bridge from an OpenAPI spec file path or URL.
// The registry must not be nil.
func NewBridge(specPath string, reg *registry.ToolRegistry, cfg BridgeConfig) (*Bridge, error) {
	if reg == nil {
		return nil, errors.New("openapi: registry must not be nil")
	}
	if specPath == "" {
		return nil, errors.New("openapi: specPath must not be empty")
	}

	loader := openapi3.NewLoader()
	var spec *openapi3.T
	var err error

	if strings.HasPrefix(specPath, "http://") || strings.HasPrefix(specPath, "https://") {
		u, parseErr := url.Parse(specPath)
		if parseErr != nil {
			return nil, fmt.Errorf("openapi: parse URL: %w", parseErr)
		}
		spec, err = loader.LoadFromURI(u)
	} else {
		spec, err = loader.LoadFromFile(specPath)
	}
	if err != nil {
		return nil, fmt.Errorf("openapi: load spec: %w", err)
	}

	return NewBridgeFromSpec(spec, reg, cfg)
}

// NewBridgeFromSpec creates a Bridge from a pre-parsed OpenAPI spec.
// This is useful for testing or when the spec is already loaded.
func NewBridgeFromSpec(spec *openapi3.T, reg *registry.ToolRegistry, cfg BridgeConfig) (*Bridge, error) {
	if reg == nil {
		return nil, errors.New("openapi: registry must not be nil")
	}
	if spec == nil {
		return nil, errors.New("openapi: spec must not be nil")
	}

	// Apply defaults.
	if cfg.NameStyle == "" {
		cfg.NameStyle = "operationId"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	// Determine base URL.
	baseURL := cfg.BaseURL
	if baseURL == "" && len(spec.Servers) > 0 && spec.Servers[0].URL != "" {
		baseURL = spec.Servers[0].URL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Bridge{
		spec:     spec,
		registry: reg,
		client:   client,
		baseURL:  baseURL,
		config:   cfg,
	}, nil
}

// RegisterTools iterates over every operation in the OpenAPI spec, converts
// each one to an MCP ToolDefinition, and registers them with the registry
// as a module named "openapi".
func (b *Bridge) RegisterTools() error {
	if b.spec.Paths == nil {
		return nil
	}

	module := &bridgeModule{
		tools: b.buildToolDefinitions(),
	}
	b.registry.RegisterModule(module)
	return nil
}

// ToolCount returns the number of operations that would be registered as tools.
func (b *Bridge) ToolCount() int {
	if b.spec.Paths == nil {
		return 0
	}
	count := 0
	for _, pathItem := range b.spec.Paths.Map() {
		for _, op := range operationsFromPathItem(pathItem) {
			if op != nil {
				count++
			}
		}
	}
	return count
}

// buildToolDefinitions creates ToolDefinitions for all operations in the spec.
func (b *Bridge) buildToolDefinitions() []registry.ToolDefinition {
	var defs []registry.ToolDefinition

	for path, pathItem := range b.spec.Paths.Map() {
		methods := methodsMap(pathItem)
		for method, op := range methods {
			if op == nil {
				continue
			}
			// Merge path-level parameters with operation-level parameters.
			allParams := mergeParams(pathItem.Parameters, op.Parameters)
			td := b.operationToTool(path, method, op, allParams)
			td.Handler = b.makeHandler(path, method, op, allParams)
			defs = append(defs, td)
		}
	}
	return defs
}

// methodsMap returns a map of HTTP method to Operation for a path item.
func methodsMap(item *openapi3.PathItem) map[string]*openapi3.Operation {
	return map[string]*openapi3.Operation{
		"GET":    item.Get,
		"POST":   item.Post,
		"PUT":    item.Put,
		"DELETE": item.Delete,
		"PATCH":  item.Patch,
	}
}

// operationsFromPathItem returns all non-nil operations.
func operationsFromPathItem(item *openapi3.PathItem) []*openapi3.Operation {
	var ops []*openapi3.Operation
	for _, op := range methodsMap(item) {
		if op != nil {
			ops = append(ops, op)
		}
	}
	return ops
}

// mergeParams combines path-level and operation-level parameters.
// Operation-level parameters take precedence (by name+in).
func mergeParams(pathParams, opParams openapi3.Parameters) openapi3.Parameters {
	seen := make(map[string]bool, len(opParams))
	for _, p := range opParams {
		if p.Value != nil {
			seen[p.Value.In+"::"+p.Value.Name] = true
		}
	}
	merged := make(openapi3.Parameters, 0, len(pathParams)+len(opParams))
	merged = append(merged, opParams...)
	for _, p := range pathParams {
		if p.Value != nil && !seen[p.Value.In+"::"+p.Value.Name] {
			merged = append(merged, p)
		}
	}
	return merged
}

// bridgeModule implements registry.ToolModule for the OpenAPI bridge.
type bridgeModule struct {
	tools []registry.ToolDefinition
}

func (m *bridgeModule) Name() string                    { return "openapi" }
func (m *bridgeModule) Description() string             { return "Auto-generated tools from OpenAPI spec" }
func (m *bridgeModule) Tools() []registry.ToolDefinition { return m.tools }
