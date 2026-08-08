// portable_server_test.go exercises NewPortableEverythingServer end-to-end
// over real streamable HTTP (initialize -> tools/list -> tools/call ->
// resources/read -> prompts/get), driven entirely through
// encoding/json + net/http with no SDK-specific imports. That is what lets
// this single file (no build tag) run unmodified under both mcp-go and
// official_sdk: it is the actual "runs REAL tests under -tags official_sdk"
// verification for the P52.6/P52.7 conformance-port canary unblocker,
// exercising every portable module (PortableToolsModule,
// PortableResourcesModule, PortablePromptsModule) through the real
// initialize -> request -> response protocol path, not just direct Go calls
// into each module's methods.
package conformance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func newPortableConformanceServer(t *testing.T) string {
	t.Helper()
	s := NewPortableEverythingServer(DefaultConfig())
	handler := registry.NewStreamableHTTPHandler(s, registry.HTTPServerOptions{
		EndpointPath: "/mcp",
		Stateless:    true,
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts.URL + "/mcp"
}

func conformancePost(t *testing.T, endpoint, body string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var payload []byte
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(string(raw), "\n") {
			if after, ok := strings.CutPrefix(line, "data: "); ok {
				payload = []byte(after)
			}
		}
		if payload == nil {
			t.Fatalf("no data line in SSE response: %s", string(raw))
		}
	} else {
		payload = raw
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal response %q (status %d): %v", string(payload), resp.StatusCode, err)
	}
	if errVal, ok := result["error"]; ok && errVal != nil {
		t.Fatalf("RPC error for request %s: %v", body, errVal)
	}
	return result
}

func conformanceInitialize(t *testing.T, endpoint string) {
	t.Helper()
	conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"conformance-test","version":"1.0.0"}}}`)
}

func TestPortableEverythingServer_ToolsList(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	result, _ := resp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	names := map[string]bool{}
	for _, tl := range tools {
		if m, ok := tl.(map[string]any); ok {
			names[m["name"].(string)] = true
		}
	}
	if !names["echo"] || !names["add"] {
		t.Errorf("expected echo and add in tools/list, got %v", names)
	}
}

func TestPortableEverythingServer_ToolsCall_Echo(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hi there"}}}`)
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("empty content: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "hi there") {
		t.Errorf("text = %q, want substring 'hi there'", text)
	}
}

func TestPortableEverythingServer_ToolsCall_Add(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"add","arguments":{"a":2,"b":3}}}`)
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("empty content: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "5") {
		t.Errorf("text = %q, want substring '5'", text)
	}
}

func TestPortableEverythingServer_ResourcesList(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":5,"method":"resources/list"}`)
	result, _ := resp["result"].(map[string]any)
	res, _ := result["resources"].([]any)
	uris := map[string]bool{}
	for _, r := range res {
		if m, ok := r.(map[string]any); ok {
			uris[m["uri"].(string)] = true
		}
	}
	if !uris["test://static-text"] || !uris["test://static-binary"] {
		t.Errorf("expected static-text and static-binary in resources/list, got %v", uris)
	}
}

func TestPortableEverythingServer_ResourcesRead_StaticText(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"test://static-text"}}`)
	result, _ := resp["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("empty contents: %v", result)
	}
	first, _ := contents[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "static text resource") {
		t.Errorf("text = %q, want substring 'static text resource'", text)
	}
}

func TestPortableEverythingServer_ResourcesRead_StaticBinary(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":7,"method":"resources/read","params":{"uri":"test://static-binary"}}`)
	result, _ := resp["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("empty contents: %v", result)
	}
	first, _ := contents[0].(map[string]any)
	if first["mimeType"] != "image/png" {
		t.Errorf("mimeType = %v, want image/png", first["mimeType"])
	}
	blob, _ := first["blob"].(string)
	if blob == "" {
		t.Error("expected non-empty base64 blob")
	}
}

func TestPortableEverythingServer_ResourcesRead_DynamicTemplate(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":8,"method":"resources/read","params":{"uri":"test://dynamic/myname"}}`)
	result, _ := resp["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("empty contents: %v", result)
	}
	first, _ := contents[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "test://dynamic/myname") {
		t.Errorf("text = %q, want substring 'test://dynamic/myname'", text)
	}
}

func TestPortableEverythingServer_ResourcesRead_TemplateJSON(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":9,"method":"resources/read","params":{"uri":"test://template/abc123/data"}}`)
	result, _ := resp["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("empty contents: %v", result)
	}
	first, _ := contents[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "abc123") {
		t.Errorf("text = %q, want substring 'abc123'", text)
	}
}

func TestPortableEverythingServer_PromptsList(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":10,"method":"prompts/list"}`)
	result, _ := resp["result"].(map[string]any)
	prompts, _ := result["prompts"].([]any)
	names := map[string]bool{}
	for _, p := range prompts {
		if m, ok := p.(map[string]any); ok {
			names[m["name"].(string)] = true
		}
	}
	for _, want := range []string{"simple_prompt", "complex_prompt", "resource_prompt", "image_prompt",
		"test_simple_prompt", "test_prompt_with_arguments", "test_prompt_with_embedded_resource", "test_prompt_with_image"} {
		if !names[want] {
			t.Errorf("expected %q in prompts/list, got %v", want, names)
		}
	}
}

func TestPortableEverythingServer_PromptsGet_Simple(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":11,"method":"prompts/get","params":{"name":"simple_prompt"}}`)
	result, _ := resp["result"].(map[string]any)
	messages, _ := result["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d: %v", len(messages), messages)
	}
}

func TestPortableEverythingServer_PromptsGet_ComplexDefaultStyle(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":12,"method":"prompts/get","params":{"name":"complex_prompt","arguments":{"name":"Bob"}}}`)
	result, _ := resp["result"].(map[string]any)
	messages, _ := result["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	m, _ := messages[0].(map[string]any)
	content, _ := m["content"].(map[string]any)
	text, _ := content["text"].(string)
	if !strings.Contains(text, "Bob") || !strings.Contains(text, "formal") {
		t.Errorf("text = %q, want 'Bob' and default 'formal'", text)
	}
}

func TestPortableEverythingServer_PromptsGet_ResourceEmbedded(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":13,"method":"prompts/get","params":{"name":"resource_prompt"}}`)
	result, _ := resp["result"].(map[string]any)
	messages, _ := result["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d: %v", len(messages), messages)
	}
	resultStr, _ := json.Marshal(result)
	if !strings.Contains(string(resultStr), "static text resource") {
		t.Errorf("expected embedded resource text in result, got: %s", resultStr)
	}
}

func TestPortableEverythingServer_PromptsGet_Image(t *testing.T) {
	endpoint := newPortableConformanceServer(t)
	conformanceInitialize(t, endpoint)

	resp := conformancePost(t, endpoint, `{"jsonrpc":"2.0","id":14,"method":"prompts/get","params":{"name":"image_prompt"}}`)
	result, _ := resp["result"].(map[string]any)
	messages, _ := result["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d: %v", len(messages), messages)
	}
	resultStr, _ := json.Marshal(result)
	if !strings.Contains(string(resultStr), "image/png") {
		t.Errorf("expected image/png in result, got: %s", resultStr)
	}
}
