//go:build !official_sdk

package gateway

import (
	"context"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

type legacyUpstreamClient struct {
	client *client.Client
}

func newUpstreamClient(ctx context.Context, endpoint, implementationName, authToken string) (upstreamClient, error) {
	var opts []transport.StreamableHTTPCOption
	if authToken != "" {
		opts = append(opts, transport.WithHTTPHeaders(map[string]string{
			"Authorization": "Bearer " + authToken,
		}))
	}
	tp, err := transport.NewStreamableHTTP(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	c := client.NewClient(tp)
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    implementationName,
		Version: "1.0.0",
	}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &legacyUpstreamClient{client: c}, nil
}

func (c *legacyUpstreamClient) listTools(ctx context.Context) ([]registry.Tool, error) {
	result, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *legacyUpstreamClient) callTool(ctx context.Context, name string, args map[string]any) (*registry.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	return c.client.CallTool(ctx, req)
}

func (c *legacyUpstreamClient) ping(ctx context.Context) error {
	return c.client.Ping(ctx)
}

func (c *legacyUpstreamClient) close() error {
	return c.client.Close()
}

func requestToolName(req registry.CallToolRequest) string {
	return req.Params.Name
}
