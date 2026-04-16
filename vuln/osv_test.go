package vuln

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOSVClient_Query_Found(t *testing.T) {
	pub := time.Date(2026, 2, 26, 18, 24, 17, 0, time.UTC)
	mod := time.Date(2026, 3, 3, 1, 29, 33, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/query") {
			http.NotFound(w, r)
			return
		}
		// Verify request body.
		var req osvQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Package.Ecosystem != "Go" {
			http.Error(w, "wrong ecosystem", http.StatusBadRequest)
			return
		}

		resp := osvAPIResponse{
			Vulns: []osvEntry{
				{
					ID:        "GO-2026-4559",
					Summary:   "Sending certain HTTP/2 frames can cause a server to panic in golang.org/x/net",
					Details:   "Due to missing nil check, sending HTTP/2 frames will cause a running server to panic",
					Aliases:   []string{"CVE-2026-27141"},
					Published: pub,
					Modified:  mod,
					References: []osvRef{
						{Type: "ADVISORY", URL: "https://nvd.nist.gov/vuln/detail/CVE-2026-27141"},
						{Type: "FIX", URL: "https://go.dev/cl/746180"},
					},
					Affected: []affected{
						{
							Pkg: osvPackage{Name: "golang.org/x/net", Ecosystem: "Go"},
							Ranges: []osvRange{
								{
									Type: "SEMVER",
									Events: []osvEvent{
										{Introduced: "0.50.0"},
										{Fixed: "0.51.0"},
									},
								},
							},
						},
					},
					DatabaseSpecific: &struct {
						URL string `json:"url"`
					}{URL: "https://pkg.go.dev/vuln/GO-2026-4559"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewOSVClient(OSVClientConfig{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})

	result, err := c.Query(context.Background(), "golang.org/x/net", "0.50.0")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Module != "golang.org/x/net" {
		t.Errorf("Module = %q", result.Module)
	}
	if len(result.Vulnerabilities) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(result.Vulnerabilities))
	}
	v := result.Vulnerabilities[0]
	if v.ID != "GO-2026-4559" {
		t.Errorf("ID = %q", v.ID)
	}
	if v.FixedVersion != "v0.51.0" {
		t.Errorf("FixedVersion = %q, want v0.51.0", v.FixedVersion)
	}
	if len(v.References) != 2 {
		t.Errorf("References = %d, want 2", len(v.References))
	}
	if v.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want HIGH", v.Severity)
	}
	if v.AdvisoryURL == "" {
		t.Error("AdvisoryURL should be set")
	}
	if !strings.Contains(result.Summary, "1 vulnerability") {
		t.Errorf("Summary = %q, want '1 vulnerability'", result.Summary)
	}
}

func TestOSVClient_Query_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(osvAPIResponse{})
	}))
	defer srv.Close()

	c := NewOSVClient(OSVClientConfig{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})

	result, err := c.Query(context.Background(), "example.com/clean", "v1.0.0")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Vulnerabilities) != 0 {
		t.Errorf("expected 0 vulns, got %d", len(result.Vulnerabilities))
	}
	if !strings.Contains(result.Summary, "No vulnerabilities") {
		t.Errorf("Summary = %q", result.Summary)
	}
}

func TestOSVClient_Query_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewOSVClient(OSVClientConfig{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})

	_, err := c.Query(context.Background(), "example.com/pkg", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention 503, got %q", err.Error())
	}
}

func TestOSVClient_Query_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	c := NewOSVClient(OSVClientConfig{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})

	_, err := c.Query(context.Background(), "example.com/pkg", "v1.0.0")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestOSVClient_Query_VersionVPrefix(t *testing.T) {
	// The module.go strips "v" from the version before calling OSVClient.Query,
	// but OSVClient.Query itself should work with or without the prefix.
	var capturedReq osvQueryRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(osvAPIResponse{})
	}))
	defer srv.Close()

	c := NewOSVClient(OSVClientConfig{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})

	_, err := c.Query(context.Background(), "example.com/pkg", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if capturedReq.Version != "1.0.0" {
		t.Errorf("version sent = %q, want 1.0.0", capturedReq.Version)
	}
}

func TestNewOSVClient_Defaults(t *testing.T) {
	c := NewOSVClient()
	if c.baseURL != osvAPIBase {
		t.Errorf("baseURL = %q, want %q", c.baseURL, osvAPIBase)
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestBuildOSVSummary_NoVulns(t *testing.T) {
	r := OSVQueryResult{Module: "example.com/m", Version: "1.0.0"}
	s := buildOSVSummary(r)
	if !strings.Contains(s, "No vulnerabilities") {
		t.Errorf("summary = %q", s)
	}
}

func TestBuildOSVSummary_MultipleVulns(t *testing.T) {
	r := OSVQueryResult{
		Module: "example.com/m",
		Vulnerabilities: []Vulnerability{
			{ID: "GO-A"},
			{ID: "GO-B"},
			{ID: "GO-C"},
		},
	}
	s := buildOSVSummary(r)
	if !strings.Contains(s, "3 vulnerabilities") {
		t.Errorf("summary = %q, want '3 vulnerabilities'", s)
	}
}
