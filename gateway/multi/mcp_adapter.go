package multi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// mcpJSONRPCRequest represents an incoming MCP JSON-RPC 2.0 request.
type mcpJSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	ID      json.RawMessage `json:"id"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpToolsCallParams holds the params for a tools/call request.
type mcpToolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Meta      *mcpMeta       `json:"_meta,omitempty"`
}

// mcpMeta holds MCP request metadata.
type mcpMeta struct {
	ProgressToken json.RawMessage `json:"progressToken,omitempty"`
}

// mcpJSONRPCResponse represents an outgoing MCP JSON-RPC 2.0 response.
type mcpJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

// mcpRPCError is a JSON-RPC 2.0 error object.
type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// mcpCallToolResult is the MCP wire format for a tools/call result.
type mcpCallToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// mcpContent is a single content block in an MCP result.
type mcpContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	jsonRPCParseError     = -32700
	jsonRPCInvalidRequest = -32600
	jsonRPCMethodNotFound = -32601
	jsonRPCInvalidParams  = -32602
	jsonRPCInternalError  = -32603
)

// MCPAdapter implements the Adapter interface for the Model Context Protocol.
// It handles MCP JSON-RPC requests (tools/call, tools/list, initialize, ping)
// by translating them to/from the gateway's canonical data model.
//
// MCPAdapter is safe for concurrent use.
type MCPAdapter struct{}

// NewMCPAdapter creates a new MCP protocol adapter.
func NewMCPAdapter() *MCPAdapter {
	return &MCPAdapter{}
}

// Protocol returns ProtocolMCP.
func (a *MCPAdapter) Protocol() Protocol {
	return ProtocolMCP
}

// Detect inspects the HTTP request and body peek to determine whether this is
// an MCP request. It checks for MCP-specific headers and JSON-RPC methods.
//
// Detection signals (in priority order):
//   - MCP-Protocol-Version or Mcp-Session-Id headers -> definitive
//   - /mcp path prefix -> high
//   - JSON-RPC method matching MCP methods -> definitive
func (a *MCPAdapter) Detect(r *http.Request, bodyPeek []byte) (bool, Confidence) {
	// Phase 1: MCP-specific headers (cheapest check).
	if r.Header.Get("MCP-Protocol-Version") != "" || r.Header.Get("Mcp-Session-Id") != "" {
		return true, ConfidenceDefinitive
	}

	// Phase 2: Path prefix.
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/") {
		return true, ConfidenceHigh
	}

	// Phase 3: JSON-RPC method in body peek.
	if r.Method == http.MethodPost && len(bodyPeek) > 0 {
		method := extractJSONRPCMethod(bodyPeek)
		if method != "" && isMCPMethod(method) {
			return true, ConfidenceDefinitive
		}
	}

	return false, ConfidenceLow
}

// Decode translates an MCP JSON-RPC request into a CanonicalRequest.
// It supports tools/call (producing a tool invocation), and returns an error
// for methods that cannot be mapped to tool calls (initialize, ping, tools/list).
//
// For tools/call, the JSON-RPC params.name becomes CanonicalRequest.ToolName
// and params.arguments becomes CanonicalRequest.Arguments. The JSON-RPC id
// is preserved as the RequestID.
func (a *MCPAdapter) Decode(r *http.Request) (*CanonicalRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	var rpcReq mcpJSONRPCRequest
	if err := json.Unmarshal(body, &rpcReq); err != nil {
		return nil, fmt.Errorf("parse JSON-RPC request: %w", err)
	}

	if rpcReq.JSONRPC != "2.0" {
		return nil, fmt.Errorf("unsupported JSON-RPC version: %q", rpcReq.JSONRPC)
	}

	requestID := formatJSONRPCID(rpcReq.ID)

	// Build metadata from the raw method for round-trip fidelity.
	metadata := map[string]string{
		"jsonrpc.method": rpcReq.Method,
	}

	switch rpcReq.Method {
	case "tools/call":
		return a.decodeToolsCall(rpcReq.Params, requestID, metadata)
	case "initialize", "ping", "tools/list",
		"resources/list", "resources/read", "resources/templates/list",
		"prompts/list", "prompts/get",
		"logging/setLevel", "completion/complete":
		// These are valid MCP methods but don't map to tool calls.
		// Store the method and raw params so the router/gateway can handle them.
		return &CanonicalRequest{
			Protocol:  ProtocolMCP,
			ToolName:  "", // No tool invocation for lifecycle methods.
			RequestID: requestID,
			Metadata:  metadata,
		}, nil
	default:
		// Notifications and unknown methods.
		if strings.HasPrefix(rpcReq.Method, "notifications/") {
			return &CanonicalRequest{
				Protocol:  ProtocolMCP,
				ToolName:  "",
				RequestID: requestID,
				Metadata:  metadata,
			}, nil
		}
		return nil, fmt.Errorf("unsupported MCP method: %q", rpcReq.Method)
	}
}

// decodeToolsCall extracts tool name and arguments from a tools/call params payload.
func (a *MCPAdapter) decodeToolsCall(params json.RawMessage, requestID string, metadata map[string]string) (*CanonicalRequest, error) {
	if params == nil {
		return nil, fmt.Errorf("tools/call missing params")
	}

	var callParams mcpToolsCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, fmt.Errorf("parse tools/call params: %w", err)
	}

	if callParams.Name == "" {
		return nil, fmt.Errorf("tools/call params missing tool name")
	}

	// Carry progress token in metadata if present.
	if callParams.Meta != nil && len(callParams.Meta.ProgressToken) > 0 {
		metadata["mcp.progressToken"] = string(callParams.Meta.ProgressToken)
	}

	return &CanonicalRequest{
		Protocol:  ProtocolMCP,
		ToolName:  callParams.Name,
		Arguments: callParams.Arguments,
		RequestID: requestID,
		Metadata:  metadata,
	}, nil
}

// Encode translates a CanonicalResponse into MCP JSON-RPC 2.0 wire format.
// Success responses produce a tools/call result object. Error responses produce
// either a JSON-RPC error (for protocol-level failures) or an MCP result with
// isError=true (for tool-level failures).
func (a *MCPAdapter) Encode(resp *CanonicalResponse) ([]byte, string, error) {
	id := marshalRequestID(resp.RequestID)

	if resp.Success {
		return a.encodeSuccess(resp, id)
	}
	return a.encodeError(resp, id)
}

// encodeSuccess wraps a successful canonical response in an MCP tools/call result.
func (a *MCPAdapter) encodeSuccess(resp *CanonicalResponse, id json.RawMessage) ([]byte, string, error) {
	result := mcpCallToolResult{
		Content: canonicalToMCPContent(resp.Content),
	}

	rpcResp := mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	body, err := json.Marshal(rpcResp)
	if err != nil {
		return nil, "", fmt.Errorf("marshal MCP response: %w", err)
	}
	return body, "application/json", nil
}

// encodeError wraps an error canonical response in MCP format.
// Tool-level errors use the MCP isError convention (result with isError=true).
// Protocol-level errors use JSON-RPC error responses.
func (a *MCPAdapter) encodeError(resp *CanonicalResponse, id json.RawMessage) ([]byte, string, error) {
	if resp.Error == nil {
		// No error details: produce a generic JSON-RPC error.
		return a.encodeJSONRPCError(id, jsonRPCInternalError, "unknown error", nil)
	}

	// Map canonical error codes to appropriate response format.
	switch resp.Error.Code {
	case ErrInvalidParams:
		return a.encodeJSONRPCError(id, jsonRPCInvalidParams, resp.Error.Message, nil)
	case ErrNotFound:
		return a.encodeJSONRPCError(id, jsonRPCMethodNotFound, resp.Error.Message, nil)
	default:
		// Tool-level errors: return an MCP result with isError=true.
		content := []mcpContent{
			{Type: "text", Text: resp.Error.Message},
		}
		// Include any additional content parts.
		if len(resp.Content) > 0 {
			content = canonicalToMCPContent(resp.Content)
			// Ensure the error message is present.
			if len(content) == 0 || content[0].Text != resp.Error.Message {
				content = append([]mcpContent{{Type: "text", Text: resp.Error.Message}}, content...)
			}
		}

		result := mcpCallToolResult{
			Content: content,
			IsError: true,
		}

		rpcResp := mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  result,
		}

		body, err := json.Marshal(rpcResp)
		if err != nil {
			return nil, "", fmt.Errorf("marshal MCP error response: %w", err)
		}
		return body, "application/json", nil
	}
}

// encodeJSONRPCError produces a JSON-RPC 2.0 error response.
func (a *MCPAdapter) encodeJSONRPCError(id json.RawMessage, code int, message string, data any) ([]byte, string, error) {
	rpcResp := mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	body, err := json.Marshal(rpcResp)
	if err != nil {
		return nil, "", fmt.Errorf("marshal JSON-RPC error: %w", err)
	}
	return body, "application/json", nil
}

// canonicalToMCPContent converts canonical ContentParts to MCP content blocks.
func canonicalToMCPContent(parts []ContentPart) []mcpContent {
	if len(parts) == 0 {
		return []mcpContent{}
	}

	result := make([]mcpContent, 0, len(parts))
	for _, p := range parts {
		switch p.Type {
		case ContentTypeText:
			result = append(result, mcpContent{
				Type: "text",
				Text: p.Text,
			})
		case ContentTypeJSON:
			// Serialize JSON content as text.
			data, err := json.Marshal(p.JSON)
			if err != nil {
				result = append(result, mcpContent{
					Type: "text",
					Text: fmt.Sprintf("[JSON marshal error: %v]", err),
				})
				continue
			}
			result = append(result, mcpContent{
				Type: "text",
				Text: string(data),
			})
		case ContentTypeImage:
			result = append(result, mcpContent{
				Type:     "image",
				MimeType: p.MimeType,
				Data:     string(p.Data),
			})
		default:
			// Data and other types: emit as text.
			if p.Text != "" {
				result = append(result, mcpContent{
					Type: "text",
					Text: p.Text,
				})
			}
		}
	}
	return result
}

// formatJSONRPCID converts a raw JSON id value to a stable string for RequestID.
func formatJSONRPCID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try as string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try as number — preserve the raw representation.
	return strings.TrimSpace(string(raw))
}

// marshalRequestID converts a RequestID back to a JSON-RPC id value.
func marshalRequestID(id string) json.RawMessage {
	if id == "" {
		return json.RawMessage("null")
	}
	// If the id looks like a number, return it as-is (e.g., "1", "42").
	if len(id) > 0 && id[0] >= '0' && id[0] <= '9' {
		// Validate it's actually a number.
		var n json.Number
		if err := json.Unmarshal([]byte(id), &n); err == nil {
			return json.RawMessage(id)
		}
	}
	// Otherwise, return as a JSON string.
	data, _ := json.Marshal(id)
	return json.RawMessage(data)
}
