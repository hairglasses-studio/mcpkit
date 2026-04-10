//go:build !official_sdk

package sampling

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeOllamaClientListModels(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("path = %q, want /api/tags", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer ollama" {
			t.Fatalf("authorization = %q, want Bearer ollama", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "code-primary", "model": "code-primary", "digest": "sha256:1"},
			},
		})
	}))
	defer ts.Close()

	client := &NativeOllamaClient{BaseURL: ts.URL}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].Name != "code-primary" {
		t.Fatalf("ListModels() = %#v, want one code-primary model", models)
	}
}

func TestNativeOllamaClientShowModelUsesExplicitAPIKey(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" {
			t.Fatalf("path = %q, want /api/show", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer explicit-key" {
			t.Fatalf("authorization = %q, want Bearer explicit-key", got)
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["name"] != "code-primary" {
			t.Fatalf("name = %q, want code-primary", req["name"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"details": map[string]any{"family": "qwen"},
		})
	}))
	defer ts.Close()

	client := &NativeOllamaClient{
		BaseURL: ts.URL,
		APIKey:  "explicit-key",
	}
	info, err := client.ShowModel(context.Background(), "code-primary")
	if err != nil {
		t.Fatalf("ShowModel() error = %v", err)
	}
	details, ok := info["details"].(map[string]any)
	if !ok || details["family"] != "qwen" {
		t.Fatalf("ShowModel() = %#v, want details.family=qwen", info)
	}
}
