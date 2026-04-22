package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// minimalSpec returns a minimal valid OpenAPI 3.0 spec with one GET operation.
func minimalSpec() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/items": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "listItems",
					"summary":     "List all items",
					"parameters": []interface{}{
						map[string]interface{}{
							"name":     "limit",
							"in":       "query",
							"required": false,
							"schema":   map[string]interface{}{"type": "integer"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
						},
					},
				},
			},
			"/items/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getItem",
					"summary":     "Get item by ID",
					"parameters": []interface{}{
						map[string]interface{}{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]interface{}{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
						},
					},
				},
			},
			"/items/create": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "createItem",
					"summary":     "Create an item",
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"type": "object"},
							},
						},
					},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{
							"description": "Created",
						},
					},
				},
			},
		},
	}
}

func newSpecServer(t *testing.T, spec map[string]interface{}) *httptest.Server {
	t.Helper()
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
}

func TestOpenAPIAdapter_Connect(t *testing.T) {
	t.Parallel()
	srv := newSpecServer(t, minimalSpec())
	defer srv.Close()

	adapter := &OpenAPIAdapter{
		cfg:        Config{URL: srv.URL, Protocol: ProtocolOpenAPI},
		httpClient: srv.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if len(adapter.operations) == 0 {
		t.Fatal("expected operations after connect")
	}
}

func TestOpenAPIAdapter_DiscoverTools(t *testing.T) {
	t.Parallel()
	srv := newSpecServer(t, minimalSpec())
	defer srv.Close()

	adapter := &OpenAPIAdapter{
		cfg:        Config{URL: srv.URL, Protocol: ProtocolOpenAPI},
		httpClient: srv.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	tools, err := adapter.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("DiscoverTools: %v", err)
	}

	// Should have 3 operations: GET /items, GET /items/{id}, POST /items/create
	if len(tools) != 3 {
		t.Errorf("tools = %d, want 3", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{"listItems", "getItem", "createItem"} {
		if !names[expected] {
			t.Errorf("missing tool %q, got %v", expected, names)
		}
	}
}

func TestOpenAPIAdapter_CallTool_GET(t *testing.T) {
	t.Parallel()

	// Backend that serves the spec and the API
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json", "/":
			data, _ := json.Marshal(minimalSpec())
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		case "/items":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]string{{"id": "1", "name": "test"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	spec := minimalSpec()
	spec["servers"] = []map[string]string{{"url": backend.URL}}

	specSrv := newSpecServer(t, spec)
	defer specSrv.Close()

	adapter := &OpenAPIAdapter{
		cfg:        Config{URL: specSrv.URL, Protocol: ProtocolOpenAPI},
		httpClient: backend.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	result, err := adapter.CallTool(context.Background(), "listItems", map[string]interface{}{
		"limit": "10",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result.Content)
	}
}

func TestOpenAPIAdapter_CallTool_PathParam(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/items/42" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "42", "name": "found"})
			return
		}
		data, _ := json.Marshal(minimalSpec())
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer backend.Close()

	spec := minimalSpec()
	spec["servers"] = []map[string]string{{"url": backend.URL}}

	specSrv := newSpecServer(t, spec)
	defer specSrv.Close()

	adapter := &OpenAPIAdapter{
		cfg:        Config{URL: specSrv.URL, Protocol: ProtocolOpenAPI},
		httpClient: backend.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := adapter.CallTool(context.Background(), "getItem", map[string]interface{}{
		"id": "42",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
}

func TestOpenAPIAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()

	adapter := &OpenAPIAdapter{
		operations: []openAPIOperation{},
	}
	result, err := adapter.CallTool(context.Background(), "nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected error result for unknown tool")
	}
}

func TestOpenAPIAdapter_CallTool_HTTPError(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/items" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		data, _ := json.Marshal(minimalSpec())
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer backend.Close()

	spec := minimalSpec()
	spec["servers"] = []map[string]string{{"url": backend.URL}}

	specSrv := newSpecServer(t, spec)
	defer specSrv.Close()

	adapter := &OpenAPIAdapter{
		cfg:        Config{URL: specSrv.URL, Protocol: ProtocolOpenAPI},
		httpClient: backend.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := adapter.CallTool(context.Background(), "listItems", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected error result for HTTP 500")
	}
}

func TestOpenAPIAdapter_CallTool_WithBody(t *testing.T) {
	t.Parallel()

	var receivedBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/items/create" {
			defer r.Body.Close()
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			receivedBody = string(buf[:n])
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"new"}`))
			return
		}
		data, _ := json.Marshal(minimalSpec())
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer backend.Close()

	spec := minimalSpec()
	spec["servers"] = []map[string]string{{"url": backend.URL}}

	specSrv := newSpecServer(t, spec)
	defer specSrv.Close()

	adapter := &OpenAPIAdapter{
		cfg:        Config{URL: specSrv.URL, Protocol: ProtocolOpenAPI},
		httpClient: backend.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	bodyJSON := `{"name":"new item"}`
	result, err := adapter.CallTool(context.Background(), "createItem", map[string]interface{}{
		"body": bodyJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if receivedBody != bodyJSON {
		t.Errorf("body = %q, want %q", receivedBody, bodyJSON)
	}
}

func TestOpenAPIAdapter_Auth_Bearer(t *testing.T) {
	t.Parallel()

	var receivedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		if r.URL.Path == "/items" {
			w.Write([]byte("[]"))
			return
		}
		data, _ := json.Marshal(minimalSpec())
		w.Write(data)
	}))
	defer backend.Close()

	spec := minimalSpec()
	spec["servers"] = []map[string]string{{"url": backend.URL}}

	specSrv := newSpecServer(t, spec)
	defer specSrv.Close()

	adapter := &OpenAPIAdapter{
		cfg: Config{
			URL:      specSrv.URL,
			Protocol: ProtocolOpenAPI,
			Auth:     &AuthConfig{Type: "bearer", Token: "test-token-123"},
		},
		httpClient: backend.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := adapter.CallTool(context.Background(), "listItems", nil)
	if err != nil {
		t.Fatal(err)
	}
	if receivedAuth != "Bearer test-token-123" {
		t.Errorf("auth = %q, want Bearer test-token-123", receivedAuth)
	}
}

func TestOpenAPIAdapter_Auth_APIKey(t *testing.T) {
	t.Parallel()

	var receivedKey string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("X-Custom-Key")
		if r.URL.Path == "/items" {
			w.Write([]byte("[]"))
			return
		}
		data, _ := json.Marshal(minimalSpec())
		w.Write(data)
	}))
	defer backend.Close()

	spec := minimalSpec()
	spec["servers"] = []map[string]string{{"url": backend.URL}}

	specSrv := newSpecServer(t, spec)
	defer specSrv.Close()

	adapter := &OpenAPIAdapter{
		cfg: Config{
			URL:      specSrv.URL,
			Protocol: ProtocolOpenAPI,
			Auth:     &AuthConfig{Type: "api_key", Token: "key-456", Header: "X-Custom-Key"},
		},
		httpClient: backend.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := adapter.CallTool(context.Background(), "listItems", nil)
	if err != nil {
		t.Fatal(err)
	}
	if receivedKey != "key-456" {
		t.Errorf("api key = %q, want key-456", receivedKey)
	}
}

func TestOpenAPIAdapter_Healthy(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	adapter := &OpenAPIAdapter{
		baseURL:    backend.URL,
		httpClient: backend.Client(),
	}
	if err := adapter.Healthy(context.Background()); err != nil {
		t.Fatalf("Healthy: %v", err)
	}
}

func TestOpenAPIAdapter_Protocol(t *testing.T) {
	t.Parallel()
	adapter := &OpenAPIAdapter{}
	if adapter.Protocol() != ProtocolOpenAPI {
		t.Errorf("Protocol = %q, want openapi", adapter.Protocol())
	}
}

func TestOpenAPIAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := &OpenAPIAdapter{}
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSanitizePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"/items", "items"},
		{"/items/{id}", "items_id"},
		{"/users/{user_id}/posts/{post_id}", "users_user_id_posts_post_id"},
		{"/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizePath(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOpenAPIAdapter_OperationWithoutID(t *testing.T) {
	t.Parallel()

	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0"},
		"paths": map[string]interface{}{
			"/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Health check",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK"},
					},
				},
			},
		},
	}

	srv := newSpecServer(t, spec)
	defer srv.Close()

	adapter := &OpenAPIAdapter{
		cfg:        Config{URL: srv.URL, Protocol: ProtocolOpenAPI},
		httpClient: srv.Client(),
	}
	if err := adapter.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	tools, _ := adapter.DiscoverTools(context.Background())
	if len(tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(tools))
	}
	// Without operationId, name should be generated from method + path
	if tools[0].Name != "get_health" {
		t.Errorf("tool name = %q, want get_health", tools[0].Name)
	}
}
