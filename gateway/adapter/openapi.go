package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"
)

// OpenAPIAdapter connects to a REST API via its OpenAPI spec and exposes
// endpoints as MCP tools. Each OpenAPI operation becomes one tool.
type OpenAPIAdapter struct {
	cfg        Config
	httpClient *http.Client
	spec       *openapi3.T
	baseURL    string
	operations []openAPIOperation
}

type openAPIOperation struct {
	toolName    string
	method      string
	path        string
	description string
	parameters  []*openapi3.ParameterRef
	requestBody *openapi3.RequestBodyRef
}

// NewOpenAPIAdapter creates an OpenAPI adapter from config.
func NewOpenAPIAdapter(ctx context.Context, cfg Config) (ProtocolAdapter, error) {
	adapter := &OpenAPIAdapter{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	if err := adapter.Connect(ctx); err != nil {
		return nil, err
	}
	return adapter, nil
}

func (o *OpenAPIAdapter) Protocol() Protocol { return ProtocolOpenAPI }

func (o *OpenAPIAdapter) Connect(ctx context.Context) error {
	loader := openapi3.NewLoader()
	loader.Context = ctx

	var spec *openapi3.T
	var err error

	if strings.HasPrefix(o.cfg.URL, "http://") || strings.HasPrefix(o.cfg.URL, "https://") {
		u, parseErr := url.Parse(o.cfg.URL)
		if parseErr != nil {
			return fmt.Errorf("openapi connect: parse URL: %w", parseErr)
		}
		spec, err = loader.LoadFromURI(u)
		if err != nil {
			// Try fetching manually and parsing
			spec, err = o.fetchAndParse(ctx)
		}
	} else {
		spec, err = loader.LoadFromFile(o.cfg.URL)
	}
	if err != nil {
		return fmt.Errorf("openapi connect: load spec: %w", err)
	}

	o.spec = spec

	// Determine base URL from spec servers or config
	if len(spec.Servers) > 0 && spec.Servers[0].URL != "" {
		o.baseURL = spec.Servers[0].URL
	}
	// Config URL overrides spec servers if it's not a spec path
	if strings.HasPrefix(o.cfg.URL, "http") && o.baseURL == "" {
		o.baseURL = strings.TrimSuffix(o.cfg.URL, "/openapi.json")
	}

	// Extract operations
	o.operations = o.extractOperations()
	return nil
}

func (o *OpenAPIAdapter) DiscoverTools(ctx context.Context) ([]mcp.Tool, error) {
	tools := make([]mcp.Tool, 0, len(o.operations))
	for _, op := range o.operations {
		toolOpts := []mcp.ToolOption{mcp.WithDescription(op.description)}

		// Add parameters as tool input properties
		for _, p := range op.parameters {
			if p.Value == nil {
				continue
			}
			param := p.Value
			opts := []mcp.PropertyOption{mcp.Description(param.Description)}
			if param.Required {
				opts = append(opts, mcp.Required())
			}
			toolOpts = append(toolOpts, mcp.WithString(param.Name, opts...))
		}

		// Add request body as "body" parameter
		if op.requestBody != nil && op.requestBody.Value != nil {
			toolOpts = append(toolOpts, mcp.WithString("body",
				mcp.Description("Request body (JSON string)")))
		}

		tools = append(tools, mcp.NewTool(op.toolName, toolOpts...))
	}
	return tools, nil
}

func (o *OpenAPIAdapter) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Find operation
	var op *openAPIOperation
	for i := range o.operations {
		if o.operations[i].toolName == toolName {
			op = &o.operations[i]
			break
		}
	}
	if op == nil {
		return makeErrorResult(fmt.Sprintf("unknown tool: %s", toolName)), nil
	}

	// Build URL with path parameters
	urlPath := op.path
	for _, p := range op.parameters {
		if p.Value != nil && p.Value.In == "path" {
			if val, ok := arguments[p.Value.Name]; ok {
				urlPath = strings.ReplaceAll(urlPath, "{"+p.Value.Name+"}", fmt.Sprint(val))
			}
		}
	}

	fullURL := o.baseURL + urlPath

	// Build query parameters
	query := ""
	for _, p := range op.parameters {
		if p.Value != nil && p.Value.In == "query" {
			if val, ok := arguments[p.Value.Name]; ok {
				if query == "" {
					query = "?"
				} else {
					query += "&"
				}
				query += p.Value.Name + "=" + fmt.Sprint(val)
			}
		}
	}
	fullURL += query

	// Build request body
	var bodyReader io.Reader
	if bodyStr, ok := arguments["body"].(string); ok && bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, op.method, fullURL, bodyReader)
	if err != nil {
		return makeErrorResult(fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Auth
	if o.cfg.Auth != nil {
		switch o.cfg.Auth.Type {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+o.cfg.Auth.Token)
		case "api_key":
			header := o.cfg.Auth.Header
			if header == "" {
				header = "X-API-Key"
			}
			req.Header.Set(header, o.cfg.Auth.Token)
		}
	}

	// Header parameters
	for _, p := range op.parameters {
		if p.Value != nil && p.Value.In == "header" {
			if val, ok := arguments[p.Value.Name]; ok {
				req.Header.Set(p.Value.Name, fmt.Sprint(val))
			}
		}
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return makeErrorResult(fmt.Sprintf("http error: %v", err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return makeErrorResult(fmt.Sprintf("read response: %v", err)), nil
	}

	if resp.StatusCode >= 400 {
		return makeErrorResult(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 500)]))), nil
	}

	// Try to format as JSON for readability
	var prettyJSON bytes.Buffer
	if json.Indent(&prettyJSON, body, "", "  ") == nil {
		body = prettyJSON.Bytes()
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: string(body)},
		},
	}, nil
}

func (o *OpenAPIAdapter) Healthy(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", o.baseURL, nil)
	if err != nil {
		return err
	}
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (o *OpenAPIAdapter) Close() error { return nil }

func (o *OpenAPIAdapter) extractOperations() []openAPIOperation {
	var ops []openAPIOperation
	if o.spec.Paths == nil {
		return ops
	}

	for path, pathItem := range o.spec.Paths.Map() {
		methods := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}
		for method, operation := range methods {
			if operation == nil {
				continue
			}
			name := operation.OperationID
			if name == "" {
				name = strings.ToLower(method) + "_" + sanitizePath(path)
			}
			ops = append(ops, openAPIOperation{
				toolName:    name,
				method:      method,
				path:        path,
				description: operation.Summary,
				parameters:  append(pathItem.Parameters, operation.Parameters...),
				requestBody: operation.RequestBody,
			})
		}
	}
	return ops
}

func (o *OpenAPIAdapter) fetchAndParse(ctx context.Context) (*openapi3.T, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.cfg.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	return loader.LoadFromData(data)
}

func sanitizePath(path string) string {
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.Trim(path, "_")
	return path
}

