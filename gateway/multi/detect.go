package multi

import (
	"bytes"
	"net/http"
	"strings"
)

// DetectProtocol determines the protocol of an incoming HTTP request by
// inspecting headers, URL path, and body structure. It returns the best-match
// protocol and the confidence level.
//
// Detection proceeds through a priority-ordered chain:
//  1. Custom headers (MCP-Protocol-Version, MCP-Session-Id) -> definitive MCP
//  2. URL path (/.well-known/agent-card.json) -> definitive A2A
//  3. URL path prefixes (/mcp, /a2a, /openai) -> high confidence
//  4. JSON-RPC method in body peek (tools/call -> MCP, a2a.* -> A2A) -> definitive
//  5. Body structure (tool_calls array -> OpenAI) -> high confidence
//  6. Fallback -> unknown with low confidence
func DetectProtocol(r *http.Request, bodyPeek []byte) (Protocol, Confidence) {
	// Phase 1: Header-based detection (no body read, cheapest check)
	if r.Header.Get("MCP-Protocol-Version") != "" || r.Header.Get("Mcp-Session-Id") != "" {
		return ProtocolMCP, ConfidenceDefinitive
	}

	// Phase 2: Path-based detection
	path := r.URL.Path
	if path == "/.well-known/agent-card.json" || path == "/agent-card:extended" {
		return ProtocolA2A, ConfidenceDefinitive
	}

	if proto, ok := detectByPathPrefix(path); ok {
		return proto, ConfidenceHigh
	}

	// Phase 3: Body peek analysis (only for POST with JSON content)
	if r.Method == http.MethodPost && hasJSONContentType(r) && len(bodyPeek) > 0 {
		// JSON-RPC method inspection — definitive when matched
		method := extractJSONRPCMethod(bodyPeek)
		if method != "" {
			if isMCPMethod(method) {
				return ProtocolMCP, ConfidenceDefinitive
			}
			if isA2AMethod(method) {
				return ProtocolA2A, ConfidenceDefinitive
			}
		}

		// OpenAI function calling structure detection
		if hasOpenAIStructure(bodyPeek) {
			return ProtocolOpenAI, ConfidenceHigh
		}
	}

	// Phase 4: Fallback
	return ProtocolUnknown, ConfidenceLow
}

// detectByPathPrefix matches URL path prefixes to protocols.
func detectByPathPrefix(path string) (Protocol, bool) {
	normalized := strings.TrimSuffix(path, "/")
	switch {
	case normalized == "/mcp" || strings.HasPrefix(path, "/mcp/"):
		return ProtocolMCP, true
	case normalized == "/a2a" || strings.HasPrefix(path, "/a2a/"):
		return ProtocolA2A, true
	case strings.HasPrefix(path, "/openai/") || strings.HasPrefix(path, "/v1/chat/"):
		return ProtocolOpenAI, true
	default:
		return ProtocolUnknown, false
	}
}

// hasJSONContentType returns true if the request Content-Type is JSON.
func hasJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "application/json") ||
		strings.HasPrefix(ct, "application/jsonrpc")
}

// extractJSONRPCMethod extracts the "method" field from a JSON-RPC body peek.
// Uses byte scanning instead of full JSON parsing for speed.
// Returns empty string if no method is found.
func extractJSONRPCMethod(peek []byte) string {
	// Look for "method" key in JSON. This is intentionally simple:
	// we search for "method":" or "method": " and extract the value.
	idx := bytes.Index(peek, []byte(`"method"`))
	if idx < 0 {
		return ""
	}

	// Skip past "method" and find the colon
	rest := peek[idx+8:] // len(`"method"`) == 8
	rest = bytes.TrimLeft(rest, " \t\n\r")
	if len(rest) == 0 || rest[0] != ':' {
		return ""
	}
	rest = rest[1:]
	rest = bytes.TrimLeft(rest, " \t\n\r")

	// Extract string value
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	end := bytes.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return string(rest[:end])
}

// isMCPMethod returns true if the method string is a known MCP JSON-RPC method.
func isMCPMethod(method string) bool {
	switch method {
	case "initialize", "ping",
		"tools/list", "tools/call",
		"resources/list", "resources/read", "resources/templates/list",
		"resources/subscribe", "resources/unsubscribe",
		"prompts/list", "prompts/get",
		"logging/setLevel", "sampling/createMessage", "roots/list",
		"completion/complete", "elicitation/create",
		"notifications/initialized", "notifications/cancelled",
		"notifications/progress", "notifications/message",
		"notifications/resources/updated", "notifications/resources/list_changed",
		"notifications/tools/list_changed", "notifications/prompts/list_changed",
		"notifications/roots/list_changed":
		return true
	default:
		return false
	}
}

// isA2AMethod returns true if the method string is a known A2A JSON-RPC method.
func isA2AMethod(method string) bool {
	switch method {
	case "a2a.sendMessage", "a2a.sendStreamingMessage",
		"a2a.getTask", "a2a.cancelTask", "a2a.listTasks",
		"a2a.getExtendedAgentCard", "a2a.subscribeToTask",
		"a2a.createPushNotificationConfig", "a2a.getPushNotificationConfig",
		"a2a.listPushNotificationConfigs", "a2a.deletePushNotificationConfig":
		return true
	default:
		return false
	}
}

// hasOpenAIStructure returns true if the body peek looks like an OpenAI
// function calling request (contains "tool_calls" or "function_call" fields).
func hasOpenAIStructure(peek []byte) bool {
	return bytes.Contains(peek, []byte(`"tool_calls"`)) ||
		bytes.Contains(peek, []byte(`"function_call"`)) ||
		bytes.Contains(peek, []byte(`"functions"`))
}
