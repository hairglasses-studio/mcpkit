package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// maxResponseBody is the maximum number of bytes read from upstream responses.
const maxResponseBody = 1 << 20 // 1 MB

// makeHandler creates an MCP tool handler that proxies requests to the upstream
// REST API for the given OpenAPI operation.
//
// The handler:
//  1. Extracts parameters from the MCP tool call arguments
//  2. Builds an HTTP request (path params, query params, headers, body)
//  3. Sends the request to the upstream API
//  4. Translates the HTTP response into an MCP CallToolResult
func (b *Bridge) makeHandler(
	path, method string,
	op *openapi3.Operation,
	params openapi3.Parameters,
) registry.ToolHandlerFunc {
	return func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		args := registry.ExtractArguments(req)
		if args == nil {
			args = make(map[string]any)
		}

		// Build URL with path parameter substitution.
		urlPath := path
		for _, pRef := range params {
			p := pRef.Value
			if p == nil || p.In != "path" {
				continue
			}
			if val, ok := args[p.Name]; ok {
				urlPath = strings.ReplaceAll(urlPath, "{"+p.Name+"}", fmt.Sprint(val))
			}
		}

		fullURL := b.baseURL + urlPath

		// Append query parameters.
		var queryParts []string
		for _, pRef := range params {
			p := pRef.Value
			if p == nil || p.In != "query" {
				continue
			}
			if val, ok := args[p.Name]; ok {
				queryParts = append(queryParts, fmt.Sprintf("%s=%v", p.Name, val))
			}
		}
		if len(queryParts) > 0 {
			fullURL += "?" + strings.Join(queryParts, "&")
		}

		// Build request body.
		var bodyReader io.Reader
		if bodyVal, ok := args["body"]; ok {
			switch v := bodyVal.(type) {
			case string:
				bodyReader = strings.NewReader(v)
			default:
				encoded, err := json.Marshal(v)
				if err != nil {
					return handler.CodedErrorResult(handler.ErrInvalidParam,
						fmt.Errorf("marshal body: %w", err)), nil
				}
				bodyReader = bytes.NewReader(encoded)
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return handler.CodedErrorResult(handler.ErrClientInit,
				fmt.Errorf("build request: %w", err)), nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")

		// Set auth header.
		if b.config.AuthHeader != "" && b.config.AuthToken != "" {
			httpReq.Header.Set(b.config.AuthHeader, b.config.AuthToken)
		}

		// Set header parameters from arguments.
		for _, pRef := range params {
			p := pRef.Value
			if p == nil || p.In != "header" {
				continue
			}
			if val, ok := args[p.Name]; ok {
				httpReq.Header.Set(p.Name, fmt.Sprint(val))
			}
		}

		resp, err := b.client.Do(httpReq)
		if err != nil {
			return handler.CodedErrorResult(handler.ErrAPIError,
				fmt.Errorf("http request: %w", err)), nil
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		if err != nil {
			return handler.CodedErrorResult(handler.ErrAPIError,
				fmt.Errorf("read response: %w", err)), nil
		}

		// Non-2xx responses are returned as MCP errors.
		if resp.StatusCode >= 400 {
			truncated := body
			if len(truncated) > 500 {
				truncated = truncated[:500]
			}
			return handler.CodedErrorResult(handler.ErrAPIError,
				fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(truncated))), nil
		}

		// Pretty-print JSON if possible.
		var prettyJSON bytes.Buffer
		if json.Indent(&prettyJSON, body, "", "  ") == nil {
			body = prettyJSON.Bytes()
		}

		return handler.TextResult(string(body)), nil
	}
}
