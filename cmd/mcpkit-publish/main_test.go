package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/discovery"
)

func TestRunPublishCLIValidateOnly(t *testing.T) {
	card := writePublishCLICard(t, validCLIMetadata())
	var stdout, stderr bytes.Buffer

	code := runPublishCLI([]string{"-card", card, "-validate-only"}, &stdout, &stderr, func(string) string { return "" })

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	var result discovery.PublishWorkflowResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !result.Skipped || !result.Validation.Valid {
		t.Fatalf("result = %#v, want skipped valid result", result)
	}
}

func TestRunPublishCLIRequiresCard(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runPublishCLI(nil, &stdout, &stderr, func(string) string { return "" })

	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "-card is required") {
		t.Fatalf("stderr = %q, want card error", stderr.String())
	}
}

func TestRunPublishCLIRegister(t *testing.T) {
	card := writePublishCLICard(t, validCLIMetadata())
	var method, path, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(discovery.ServerMetadata{ID: "srv-cli", Name: "published"})
	}))
	defer srv.Close()
	var stdout, stderr bytes.Buffer

	code := runPublishCLI([]string{
		"-card", card,
		"-registry-url", srv.URL,
		"-token", "tok",
	}, &stdout, &stderr, func(string) string { return "" })

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if method != http.MethodPost || path != "/v1/servers" {
		t.Fatalf("request = %s %s, want POST /v1/servers", method, path)
	}
	if auth != "Bearer tok" {
		t.Fatalf("auth = %q, want Bearer tok", auth)
	}
}

func TestRunPublishCLIUsesTokenEnv(t *testing.T) {
	card := writePublishCLICard(t, validCLIMetadata())
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(discovery.ServerMetadata{ID: "srv-env", Name: "published"})
	}))
	defer srv.Close()
	var stdout, stderr bytes.Buffer

	code := runPublishCLI([]string{
		"-card", card,
		"-registry-url", srv.URL,
	}, &stdout, &stderr, func(key string) string {
		if key == "MCP_REGISTRY_TOKEN" {
			return "env-token"
		}
		return ""
	})

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if auth != "Bearer env-token" {
		t.Fatalf("auth = %q, want Bearer env-token", auth)
	}
}

func TestRunPublishCLIOAuthClientCredentials(t *testing.T) {
	card := writePublishCLICard(t, validCLIMetadata())
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "client_credentials" {
			t.Fatalf("grant_type = %q, want client_credentials", got)
		}
		if got := r.Form.Get("client_id"); got != "client-id" {
			t.Fatalf("client_id = %q, want client-id", got)
		}
		if got := r.Form.Get("scope"); got != "registry.publish registry.delete" {
			t.Fatalf("scope = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "oauth-cli-token",
			"token_type":   "Bearer",
		})
	}))
	defer tokenServer.Close()
	var auth string
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(discovery.ServerMetadata{ID: "srv-oauth", Name: "published"})
	}))
	defer registryServer.Close()
	var stdout, stderr bytes.Buffer

	code := runPublishCLI([]string{
		"-card", card,
		"-registry-url", registryServer.URL,
		"-token-url", tokenServer.URL,
		"-client-id", "client-id",
		"-client-secret", "client-secret",
		"-scopes", "registry.publish, registry.delete",
	}, &stdout, &stderr, func(string) string { return "" })

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if auth != "Bearer oauth-cli-token" {
		t.Fatalf("auth = %q, want OAuth bearer", auth)
	}
}

func writePublishCLICard(t *testing.T, meta discovery.ServerMetadata) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write card: %v", err)
	}
	return path
}

func validCLIMetadata() discovery.ServerMetadata {
	return discovery.ServerMetadata{
		Name:         "cli-server",
		Description:  "CLI test server",
		Version:      "1.0.0",
		Organization: "Example",
		Repository:   "https://github.com/example/cli-server",
		Transports: []discovery.TransportInfo{{
			Type: "streamable-http",
			URL:  "https://example.com/mcp",
		}},
		Tools: []discovery.ToolSummary{{
			Name:        "echo",
			Description: "Echo text",
		}},
		Auth: &discovery.AuthRequirement{Type: "none"},
	}
}
