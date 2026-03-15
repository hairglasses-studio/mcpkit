package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestGCPMetadataProvider_GetToken(t *testing.T) {
	t.Parallel()

	const mockToken = "gcp-identity-token-abc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.Header.Get("Metadata-Flavor") != "Google" {
			t.Errorf("Metadata-Flavor = %q, want Google", r.Header.Get("Metadata-Flavor"))
		}
		if r.URL.Path != "/computeMetadata/v1/instance/service-accounts/default/identity" {
			t.Errorf("path = %q, unexpected", r.URL.Path)
		}
		if r.URL.Query().Get("audience") != "test-audience" {
			t.Errorf("audience = %q, want test-audience", r.URL.Query().Get("audience"))
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, mockToken)
	}))
	defer srv.Close()

	p := NewGCPMetadataProvider("test-audience")
	// Override the base URL by replacing the HTTPClient transport with one that
	// rewrites requests to our test server.
	p.HTTPClient = srv.Client()

	// Directly build the URL using the test server address instead of the real metadata server.
	// We test via a provider that targets the mock server directly.
	p2 := &GCPMetadataProvider{
		Audience:   "test-audience",
		HTTPClient: &http.Client{},
	}
	p2.HTTPClient.Transport = rewriteTransport{target: srv.URL}

	token, err := p2.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if token != mockToken {
		t.Errorf("token = %q, want %q", token, mockToken)
	}
}

func TestGCPMetadataProvider_GetToken_Error(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := &GCPMetadataProvider{
		Audience:   "test-audience",
		HTTPClient: &http.Client{Transport: rewriteTransport{target: srv.URL}},
	}

	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestGCPMetadataProvider_Name(t *testing.T) {
	t.Parallel()

	p := NewGCPMetadataProvider("aud")
	if p.Name() != "gcp" {
		t.Errorf("Name() = %q, want gcp", p.Name())
	}
}

func TestAWSIMDSProvider_GetToken(t *testing.T) {
	t.Parallel()

	const mockIMDSToken = "mock-imds-token"
	const mockRole = "test-role"
	const mockSessionToken = "aws-session-token-xyz"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			if r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds") != "21600" {
				t.Errorf("X-aws-ec2-metadata-token-ttl-seconds = %q, want 21600",
					r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, mockIMDSToken)

		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/":
			if r.Header.Get("X-aws-ec2-metadata-token") != mockIMDSToken {
				t.Errorf("X-aws-ec2-metadata-token = %q, want %q",
					r.Header.Get("X-aws-ec2-metadata-token"), mockIMDSToken)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, mockRole)

		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/"+mockRole:
			if r.Header.Get("X-aws-ec2-metadata-token") != mockIMDSToken {
				t.Errorf("X-aws-ec2-metadata-token = %q, want %q",
					r.Header.Get("X-aws-ec2-metadata-token"), mockIMDSToken)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"AccessKeyId":     "AKIA...",
				"SecretAccessKey": "secret",
				"Token":           mockSessionToken,
			})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := &AWSIMDSProvider{
		HTTPClient: &http.Client{Transport: rewriteTransport{target: srv.URL}},
	}

	token, err := p.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if token != mockSessionToken {
		t.Errorf("token = %q, want %q", token, mockSessionToken)
	}
}

func TestAWSIMDSProvider_GetToken_TokenFetchError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := &AWSIMDSProvider{
		HTTPClient: &http.Client{Transport: rewriteTransport{target: srv.URL}},
	}

	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 IMDS token response")
	}
}

func TestAWSIMDSProvider_Name(t *testing.T) {
	t.Parallel()

	p := NewAWSIMDSProvider()
	if p.Name() != "aws" {
		t.Errorf("Name() = %q, want aws", p.Name())
	}
}

// TestAutoDetectNone verifies that AutoDetect returns an error when no
// workload identity providers are reachable (normal in unit test environments).
func TestAutoDetectNone(t *testing.T) {
	t.Parallel()

	// In a unit test environment neither GCP metadata server nor AWS IMDS
	// are reachable, so AutoDetect should return an error.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := AutoDetect(ctx)
	if err == nil {
		// We may be running inside GCP or AWS — skip rather than fail.
		t.Skip("AutoDetect returned a provider; may be running in a cloud environment")
	}
}

func TestWorkloadMiddleware(t *testing.T) {
	t.Parallel()

	const fixedToken = "workload-token-abc"

	mock := &mockWorkloadProvider{token: fixedToken, name: "testcloud"}

	var capturedCtx context.Context
	inner := registry.ToolHandlerFunc(func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		capturedCtx = ctx
		return registry.MakeTextResult("ok"), nil
	})

	mw := WorkloadMiddleware(mock)
	handler := mw("test-tool", registry.ToolDefinition{}, inner)

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if registry.IsResultError(result) {
		t.Fatalf("handler returned error result")
	}

	if got := WorkloadToken(capturedCtx); got != fixedToken {
		t.Errorf("WorkloadToken = %q, want %q", got, fixedToken)
	}
	if got := Subject(capturedCtx); got != "workload:testcloud" {
		t.Errorf("Subject = %q, want workload:testcloud", got)
	}
}

func TestWorkloadMiddleware_ProviderError(t *testing.T) {
	t.Parallel()

	mock := &mockWorkloadProvider{err: fmt.Errorf("metadata unreachable"), name: "badcloud"}

	inner := registry.ToolHandlerFunc(func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) {
		t.Error("inner handler should not be called when provider fails")
		return registry.MakeTextResult("should not reach here"), nil
	})

	mw := WorkloadMiddleware(mock)
	handler := mw("test-tool", registry.ToolDefinition{}, inner)

	result, err := handler(context.Background(), registry.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !registry.IsResultError(result) {
		t.Fatal("expected error result when provider fails")
	}
}

func TestWorkloadContextHelpers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Empty context returns empty string.
	if got := WorkloadToken(ctx); got != "" {
		t.Errorf("WorkloadToken on empty context = %q, want empty", got)
	}

	const want = "my-workload-token"
	ctx = WithWorkloadToken(ctx, want)
	if got := WorkloadToken(ctx); got != want {
		t.Errorf("WorkloadToken = %q, want %q", got, want)
	}
}

// --- helpers ---

// rewriteTransport rewrites all requests so the host points to target,
// allowing real HTTP client code to be tested against a local httptest.Server.
type rewriteTransport struct {
	target string // e.g., "http://127.0.0.1:PORT"
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	// Strip scheme+host from target to get just host:port.
	targetURL, err := http.NewRequest(http.MethodGet, rt.target, nil)
	if err != nil {
		return nil, err
	}
	cloned.URL.Host = targetURL.URL.Host
	cloned.Host = targetURL.URL.Host
	return http.DefaultTransport.RoundTrip(cloned)
}

// mockWorkloadProvider is a WorkloadIdentityProvider for testing.
type mockWorkloadProvider struct {
	token string
	name  string
	err   error
}

func (m *mockWorkloadProvider) GetToken(_ context.Context) (string, error) {
	return m.token, m.err
}

func (m *mockWorkloadProvider) Name() string { return m.name }
