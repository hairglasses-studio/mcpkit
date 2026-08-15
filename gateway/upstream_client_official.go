//go:build official_sdk

package gateway

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

type officialUpstreamClient struct {
	session *mcp.ClientSession
}

func newUpstreamClient(ctx context.Context, endpoint, implementationName, authToken string) (upstreamClient, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		DisableStandaloneSSE: true,
	}
	if authToken != "" {
		transport.HTTPClient = &http.Client{Transport: bearerRoundTripper{
			base:  http.DefaultTransport,
			token: authToken,
		}}
	}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    implementationName,
		Version: "1.0.0",
	}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return &officialUpstreamClient{session: session}, nil
}

func (c *officialUpstreamClient) listTools(ctx context.Context) ([]registry.Tool, error) {
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	tools := make([]registry.Tool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		if tool != nil {
			tools = append(tools, *tool)
		}
	}
	return tools, nil
}

func (c *officialUpstreamClient) callTool(ctx context.Context, name string, args map[string]any) (*registry.CallToolResult, error) {
	return c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

func (c *officialUpstreamClient) ping(ctx context.Context) error {
	// MCP 2026-07-28 removed ping. Use a bounded, read-only request on the
	// negotiated session so the health loop still verifies protocol liveness.
	_, err := c.session.ListTools(ctx, nil)
	return err
}

func (c *officialUpstreamClient) close() error {
	return c.session.Close()
}

func requestToolName(req registry.CallToolRequest) string {
	if req.Params == nil {
		return ""
	}
	return req.Params.Name
}

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (rt bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+rt.token)
	return rt.base.RoundTrip(clone)
}
