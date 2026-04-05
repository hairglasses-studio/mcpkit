//go:build !official_sdk

package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// newTestServer creates an httptest.Server that simulates a Petstore API.
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /pets", func(w http.ResponseWriter, r *http.Request) {
		limit := r.URL.Query().Get("limit")
		pets := []map[string]any{
			{"id": "1", "name": "Fido"},
			{"id": "2", "name": "Whiskers"},
		}
		if limit == "1" {
			pets = pets[:1]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pets)
	})

	mux.HandleFunc("POST /pets", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		body["id"] = "3"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(body)
	})

	mux.HandleFunc("GET /pets/{petId}", func(w http.ResponseWriter, r *http.Request) {
		petId := r.PathValue("petId")
		if petId == "404" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   petId,
			"name": "Fido",
		})
	})

	mux.HandleFunc("DELETE /pets/{petId}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /pets/{petId}/vaccinations", func(w http.ResponseWriter, r *http.Request) {
		reqId := r.Header.Get("X-Request-Id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"petId":      r.PathValue("petId"),
			"requestId":  reqId,
			"vaccines":   []string{"rabies", "distemper"},
		})
	})

	return httptest.NewServer(mux)
}

// makeBridgeWithServer creates a Bridge pointed at the test server.
func makeBridgeWithServer(t *testing.T, ts *httptest.Server) *Bridge {
	t.Helper()
	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{
		BaseURL: ts.URL,
		Client:  ts.Client(),
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if err := b.RegisterTools(); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}
	return b
}

func callTool(t *testing.T, b *Bridge, name string, args map[string]any) *registry.CallToolResult {
	t.Helper()
	td, ok := b.registry.GetTool(name)
	if !ok {
		t.Fatalf("tool %q not found", name)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s handler error: %v", name, err)
	}
	return result
}

func TestHandler_ListPets(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	result := callTool(t, b, "listPets", nil)
	if registry.IsResultError(result) {
		t.Fatalf("listPets returned error: %v", result)
	}

	text, ok := registry.ExtractTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	if text == "" {
		t.Error("expected non-empty response")
	}

	// Should contain both pets.
	var pets []map[string]any
	if err := json.Unmarshal([]byte(text), &pets); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(pets) != 2 {
		t.Errorf("expected 2 pets, got %d", len(pets))
	}
}

func TestHandler_ListPets_WithQuery(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	result := callTool(t, b, "listPets", map[string]any{"limit": "1"})
	if registry.IsResultError(result) {
		t.Fatalf("listPets returned error: %v", result)
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	var pets []map[string]any
	if err := json.Unmarshal([]byte(text), &pets); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(pets) != 1 {
		t.Errorf("expected 1 pet with limit=1, got %d", len(pets))
	}
}

func TestHandler_GetPet_PathParam(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	result := callTool(t, b, "getPet", map[string]any{"petId": "42"})
	if registry.IsResultError(result) {
		t.Fatalf("getPet returned error: %v", result)
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	var pet map[string]any
	if err := json.Unmarshal([]byte(text), &pet); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if pet["id"] != "42" {
		t.Errorf("pet id = %v, want 42", pet["id"])
	}
}

func TestHandler_GetPet_NotFound(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	result := callTool(t, b, "getPet", map[string]any{"petId": "404"})
	if !registry.IsResultError(result) {
		t.Error("expected error result for 404 response")
	}
}

func TestHandler_CreatePet_Body(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	body := map[string]any{"name": "Rex", "tag": "dog"}
	result := callTool(t, b, "createPet", map[string]any{"body": body})
	if registry.IsResultError(result) {
		t.Fatalf("createPet returned error: %v", result)
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	var created map[string]any
	if err := json.Unmarshal([]byte(text), &created); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if created["name"] != "Rex" {
		t.Errorf("created name = %v, want Rex", created["name"])
	}
	if created["id"] != "3" {
		t.Errorf("created id = %v, want 3", created["id"])
	}
}

func TestHandler_CreatePet_StringBody(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	result := callTool(t, b, "createPet", map[string]any{
		"body": `{"name":"Luna","tag":"cat"}`,
	})
	if registry.IsResultError(result) {
		t.Fatalf("createPet returned error: %v", result)
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	var created map[string]any
	if err := json.Unmarshal([]byte(text), &created); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if created["name"] != "Luna" {
		t.Errorf("created name = %v, want Luna", created["name"])
	}
}

func TestHandler_DeletePet(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	result := callTool(t, b, "deletePet", map[string]any{"petId": "1"})
	if registry.IsResultError(result) {
		t.Fatalf("deletePet returned error: %v", result)
	}
}

func TestHandler_HeaderParam(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := makeBridgeWithServer(t, ts)

	// The vaccinations endpoint has no operationId; use the fallback name.
	toolName := "get_pets_petId_vaccinations"
	result := callTool(t, b, toolName, map[string]any{
		"petId":        "7",
		"X-Request-Id": "trace-123",
	})
	if registry.IsResultError(result) {
		t.Fatalf("%s returned error: %v", toolName, result)
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["petId"] != "7" {
		t.Errorf("petId = %v, want 7", resp["petId"])
	}
	if resp["requestId"] != "trace-123" {
		t.Errorf("requestId = %v, want trace-123", resp["requestId"])
	}
}

func TestHandler_Auth(t *testing.T) {
	// Create a server that checks for auth header.
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /pets", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	reg := registry.NewToolRegistry()
	b, err := NewBridge(testdataPath("petstore.json"), reg, BridgeConfig{
		BaseURL:    ts.URL,
		Client:     ts.Client(),
		AuthHeader: "Authorization",
		AuthToken:  "Bearer test-token",
	})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if err := b.RegisterTools(); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}

	callTool(t, b, "listPets", nil)
	if gotAuth != "Bearer test-token" {
		t.Errorf("auth header = %q, want %q", gotAuth, "Bearer test-token")
	}
}
