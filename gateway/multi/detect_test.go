package multi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetectProtocol_MCPHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	proto, conf := DetectProtocol(req, nil)
	if proto != ProtocolMCP {
		t.Errorf("protocol = %q, want mcp", proto)
	}
	if conf != ConfidenceDefinitive {
		t.Errorf("confidence = %v, want definitive", conf)
	}
}

func TestDetectProtocol_MCPSessionHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Mcp-Session-Id", "sess-abc-123")

	proto, conf := DetectProtocol(req, nil)
	if proto != ProtocolMCP {
		t.Errorf("protocol = %q, want mcp", proto)
	}
	if conf != ConfidenceDefinitive {
		t.Errorf("confidence = %v, want definitive", conf)
	}
}

func TestDetectProtocol_A2AWellKnown(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)

	proto, conf := DetectProtocol(req, nil)
	if proto != ProtocolA2A {
		t.Errorf("protocol = %q, want a2a", proto)
	}
	if conf != ConfidenceDefinitive {
		t.Errorf("confidence = %v, want definitive", conf)
	}
}

func TestDetectProtocol_A2AExtendedCard(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/agent-card:extended", nil)

	proto, conf := DetectProtocol(req, nil)
	if proto != ProtocolA2A {
		t.Errorf("protocol = %q, want a2a", proto)
	}
	if conf != ConfidenceDefinitive {
		t.Errorf("confidence = %v, want definitive", conf)
	}
}

func TestDetectProtocol_PathPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		wantProt Protocol
	}{
		{"mcp root", "/mcp", ProtocolMCP},
		{"mcp subpath", "/mcp/ws", ProtocolMCP},
		{"a2a root", "/a2a", ProtocolA2A},
		{"a2a subpath", "/a2a/stream", ProtocolA2A},
		{"openai path", "/openai/v1/chat/completions", ProtocolOpenAI},
		{"v1 chat path", "/v1/chat/completions", ProtocolOpenAI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			proto, conf := DetectProtocol(req, nil)
			if proto != tt.wantProt {
				t.Errorf("path %q: protocol = %q, want %q", tt.path, proto, tt.wantProt)
			}
			if conf != ConfidenceHigh {
				t.Errorf("path %q: confidence = %v, want high", tt.path, conf)
			}
		})
	}
}

func TestDetectProtocol_MCPMethod(t *testing.T) {
	t.Parallel()

	methods := []string{"tools/call", "tools/list", "initialize", "ping", "resources/list"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			body := `{"jsonrpc":"2.0","method":"` + method + `","id":1}`
			req := httptest.NewRequest(http.MethodPost, "/",
				strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			proto, conf := DetectProtocol(req, []byte(body))
			if proto != ProtocolMCP {
				t.Errorf("method %q: protocol = %q, want mcp", method, proto)
			}
			if conf != ConfidenceDefinitive {
				t.Errorf("method %q: confidence = %v, want definitive", method, conf)
			}
		})
	}
}

func TestDetectProtocol_A2AMethod(t *testing.T) {
	t.Parallel()

	methods := []string{"a2a.sendMessage", "a2a.getTask", "a2a.cancelTask", "a2a.listTasks"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			body := `{"jsonrpc":"2.0","method":"` + method + `","id":1}`
			req := httptest.NewRequest(http.MethodPost, "/",
				strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			proto, conf := DetectProtocol(req, []byte(body))
			if proto != ProtocolA2A {
				t.Errorf("method %q: protocol = %q, want a2a", method, proto)
			}
			if conf != ConfidenceDefinitive {
				t.Errorf("method %q: confidence = %v, want definitive", method, conf)
			}
		})
	}
}

func TestDetectProtocol_OpenAIStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{
			"tool_calls array",
			`{"model":"gpt-4","messages":[],"tool_calls":[{"function":{"name":"search"}}]}`,
		},
		{
			"function_call field",
			`{"model":"gpt-4","function_call":{"name":"search","arguments":"{}"}}`,
		},
		{
			"functions array",
			`{"model":"gpt-4","functions":[{"name":"search","parameters":{}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			proto, conf := DetectProtocol(req, []byte(tt.body))
			if proto != ProtocolOpenAI {
				t.Errorf("protocol = %q, want openai", proto)
			}
			if conf != ConfidenceHigh {
				t.Errorf("confidence = %v, want high", conf)
			}
		})
	}
}

func TestDetectProtocol_Unknown(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/some/random/path", nil)

	proto, conf := DetectProtocol(req, nil)
	if proto != ProtocolUnknown {
		t.Errorf("protocol = %q, want unknown", proto)
	}
	if conf != ConfidenceLow {
		t.Errorf("confidence = %v, want low", conf)
	}
}

func TestDetectProtocol_AmbiguousJSONRPC(t *testing.T) {
	t.Parallel()

	// A JSON-RPC request with an unrecognized method should fall through.
	body := `{"jsonrpc":"2.0","method":"custom.doSomething","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	proto, conf := DetectProtocol(req, []byte(body))
	if proto != ProtocolUnknown {
		t.Errorf("protocol = %q, want unknown for unrecognized method", proto)
	}
	if conf != ConfidenceLow {
		t.Errorf("confidence = %v, want low", conf)
	}
}

func TestDetectProtocol_HeaderTakesPriorityOverPath(t *testing.T) {
	t.Parallel()

	// MCP header on an A2A path: header wins.
	req := httptest.NewRequest(http.MethodPost, "/a2a", nil)
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")

	proto, _ := DetectProtocol(req, nil)
	if proto != ProtocolMCP {
		t.Errorf("protocol = %q, want mcp (header should beat path)", proto)
	}
}

func TestDetectProtocol_EmptyBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")

	proto, conf := DetectProtocol(req, []byte{})
	if proto != ProtocolUnknown {
		t.Errorf("protocol = %q, want unknown for empty body", proto)
	}
	if conf != ConfidenceLow {
		t.Errorf("confidence = %v, want low", conf)
	}
}

func TestExtractJSONRPCMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{"standard", `{"jsonrpc":"2.0","method":"tools/call","id":1}`, "tools/call"},
		{"method first", `{"method":"initialize","jsonrpc":"2.0"}`, "initialize"},
		{"extra spaces", `{"method" : "ping" , "id": 1}`, "ping"},
		{"no method", `{"jsonrpc":"2.0","id":1}`, ""},
		{"empty string", ``, ""},
		{"numeric method", `{"method":42}`, ""},
		{"nested method first match", `{"params":{"method":"inner"},"method":"outer"}`, "inner"},
		{"top-level method", `{"method":"outer","params":{"method":"inner"}}`, "outer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractJSONRPCMethod([]byte(tt.body))
			if got != tt.want {
				t.Errorf("extractJSONRPCMethod(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestHasJSONContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ct   string
		want bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/jsonrpc", true},
		{"text/plain", false},
		{"text/html", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ct, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.ct != "" {
				req.Header.Set("Content-Type", tt.ct)
			}
			if got := hasJSONContentType(req); got != tt.want {
				t.Errorf("hasJSONContentType(ct=%q) = %v, want %v", tt.ct, got, tt.want)
			}
		})
	}
}

func TestIsMCPMethod(t *testing.T) {
	t.Parallel()

	mcpMethods := []string{
		"initialize", "ping", "tools/list", "tools/call",
		"resources/list", "resources/read", "prompts/list", "prompts/get",
		"logging/setLevel", "sampling/createMessage", "completion/complete",
	}
	for _, m := range mcpMethods {
		if !isMCPMethod(m) {
			t.Errorf("isMCPMethod(%q) = false, want true", m)
		}
	}

	nonMCP := []string{"a2a.sendMessage", "custom.method", "", "tools"}
	for _, m := range nonMCP {
		if isMCPMethod(m) {
			t.Errorf("isMCPMethod(%q) = true, want false", m)
		}
	}
}

func TestIsA2AMethod(t *testing.T) {
	t.Parallel()

	a2aMethods := []string{
		"a2a.sendMessage", "a2a.getTask", "a2a.cancelTask",
		"a2a.sendStreamingMessage", "a2a.listTasks",
	}
	for _, m := range a2aMethods {
		if !isA2AMethod(m) {
			t.Errorf("isA2AMethod(%q) = false, want true", m)
		}
	}

	nonA2A := []string{"tools/call", "custom.method", "", "a2a"}
	for _, m := range nonA2A {
		if isA2AMethod(m) {
			t.Errorf("isA2AMethod(%q) = true, want false", m)
		}
	}
}

func TestHasOpenAIStructure(t *testing.T) {
	t.Parallel()

	positive := []string{
		`{"tool_calls":[]}`,
		`{"function_call":{"name":"x"}}`,
		`{"functions":[{"name":"x"}]}`,
	}
	for _, body := range positive {
		if !hasOpenAIStructure([]byte(body)) {
			t.Errorf("hasOpenAIStructure(%q) = false, want true", body)
		}
	}

	negative := []string{
		`{"method":"tools/call"}`,
		`{"messages":[]}`,
		`{}`,
	}
	for _, body := range negative {
		if hasOpenAIStructure([]byte(body)) {
			t.Errorf("hasOpenAIStructure(%q) = true, want false", body)
		}
	}
}

func TestDetectByPathPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		wantProt Protocol
		wantOK   bool
	}{
		{"/mcp", ProtocolMCP, true},
		{"/mcp/", ProtocolMCP, true},
		{"/mcp/ws", ProtocolMCP, true},
		{"/a2a", ProtocolA2A, true},
		{"/a2a/stream", ProtocolA2A, true},
		{"/openai/v1/chat", ProtocolOpenAI, true},
		{"/v1/chat/completions", ProtocolOpenAI, true},
		{"/other", ProtocolUnknown, false},
		{"/", ProtocolUnknown, false},
		{"/mcpx", ProtocolUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			proto, ok := detectByPathPrefix(tt.path)
			if ok != tt.wantOK {
				t.Errorf("path %q: ok = %v, want %v", tt.path, ok, tt.wantOK)
			}
			if proto != tt.wantProt {
				t.Errorf("path %q: protocol = %q, want %q", tt.path, proto, tt.wantProt)
			}
		})
	}
}
