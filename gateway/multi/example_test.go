package multi_test

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/hairglasses-studio/mcpkit/gateway/multi"
)

// ExampleDetectProtocol demonstrates the multi-protocol gateway's
// auto-detection chain. The detector inspects (in priority order):
//  1. Custom headers (MCP-Protocol-Version, MCP-Session-Id) → definitive MCP
//  2. URL path well-knowns (/.well-known/agent-card.json) → definitive A2A
//  3. URL path prefixes (/mcp, /a2a, /openai) → high confidence
//  4. JSON-RPC method in body peek (tools/call, a2a.*) → definitive
//  5. OpenAI-shaped body (tool_calls array) → high confidence
//
// The detection result lets the gateway route the request to the
// appropriate per-protocol adapter without forcing the client to
// disambiguate via separate endpoints.
func ExampleDetectProtocol() {
	// Case 1: MCP-tagged header — definitive MCP route.
	req1, _ := http.NewRequest("POST", "/api", bytes.NewReader([]byte(`{"jsonrpc":"2.0"}`)))
	req1.Header.Set("MCP-Protocol-Version", "2025-11-25")
	protocol, conf := multi.DetectProtocol(req1, nil)
	fmt.Printf("case 1: %s confidence=%d\n", protocol, conf)

	// Case 2: A2A well-known path — definitive A2A route.
	req2, _ := http.NewRequest("GET", "/.well-known/agent-card.json", nil)
	protocol, conf = multi.DetectProtocol(req2, nil)
	fmt.Printf("case 2: %s confidence=%d\n", protocol, conf)

	// Case 3: Generic /api path with no telltale headers or body — unknown.
	req3, _ := http.NewRequest("POST", "/api", nil)
	protocol, conf = multi.DetectProtocol(req3, nil)
	fmt.Printf("case 3: %s confidence=%d\n", protocol, conf)

	// Output:
	// case 1: mcp confidence=3
	// case 2: a2a confidence=3
	// case 3: unknown confidence=0
}

// Example_protocolConstants shows the canonical Protocol constants. These are
// the values DetectProtocol can return and that downstream adapters
// switch on.
func Example_protocolConstants() {
	fmt.Println(multi.ProtocolMCP)
	fmt.Println(multi.ProtocolA2A)
	fmt.Println(multi.ProtocolOpenAI)
	fmt.Println(multi.ProtocolUnknown)

	// Output:
	// mcp
	// a2a
	// openai
	// unknown
}
