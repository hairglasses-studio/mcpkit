//go:build official_sdk

package resources

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hairglasses-studio/mcpkit/sanitize"
)

// uriTestModule implements ResourceModule for the official_sdk uri_middleware
// tests. It is a minimal, local stand-in for resources_test.go's testModule,
// which is !official_sdk-only (it also exercises DynamicRegistry, which has
// no official_sdk port).
type uriTestModule struct {
	name      string
	resources []ResourceDefinition
}

func (m *uriTestModule) Name() string                    { return m.name }
func (m *uriTestModule) Description() string             { return "uri test module" }
func (m *uriTestModule) Resources() []ResourceDefinition { return m.resources }
func (m *uriTestModule) Templates() []TemplateDefinition { return nil }

// okHandler returns a handler that always succeeds with a text resource.
func okHandler(text string) ResourceHandlerFunc {
	return func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "test://", Text: text},
			},
		}, nil
	}
}

func makeRequest(uri string) *mcp.ReadResourceRequest {
	return &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: uri}}
}

func TestURIValidationMiddleware_ValidURI(t *testing.T) {
	mw := URIValidationMiddleware(sanitize.DefaultURIPolicy())

	rd := ResourceDefinition{
		Resource: mcp.Resource{URI: "https://example.com/data", Name: "Data"},
		Handler:  okHandler("content"),
	}

	wrapped := mw("https://example.com/data", rd, rd.Handler)
	result, err := wrapped(context.Background(), makeRequest("https://example.com/data"))
	if err != nil {
		t.Fatalf("unexpected error for valid URI: %v", err)
	}
	if result == nil || len(result.Contents) != 1 {
		t.Fatalf("expected 1 content item, got %v", result)
	}
}

func TestURIValidationMiddleware_PathTraversal(t *testing.T) {
	mw := URIValidationMiddleware(sanitize.DefaultURIPolicy())

	rd := ResourceDefinition{
		Resource: mcp.Resource{URI: "https://example.com/data", Name: "Data"},
		Handler:  okHandler("content"),
	}

	wrapped := mw("https://example.com/data", rd, rd.Handler)
	_, err := wrapped(context.Background(), makeRequest("https://example.com/../../../etc/passwd"))
	if err == nil {
		t.Fatal("expected error for path traversal URI")
	}
	if !strings.Contains(err.Error(), "URI validation failed") {
		t.Errorf("error should mention validation failed, got: %v", err)
	}
}

func TestURIValidationMiddleware_SSRFBlocked(t *testing.T) {
	mw := URIValidationMiddleware(sanitize.DefaultURIPolicy())

	rd := ResourceDefinition{
		Resource: mcp.Resource{URI: "https://example.com/data", Name: "Data"},
		Handler:  okHandler("should not reach"),
	}

	ssrfURIs := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://localhost/admin",
		"http://127.0.0.1/internal",
	}

	wrapped := mw("https://example.com/data", rd, rd.Handler)
	for _, uri := range ssrfURIs {
		t.Run(uri, func(t *testing.T) {
			_, err := wrapped(context.Background(), makeRequest(uri))
			if err == nil {
				t.Errorf("expected SSRF to be blocked for %q", uri)
			}
		})
	}
}

func TestURIValidationMiddleware_BlockedScheme(t *testing.T) {
	mw := URIValidationMiddleware(sanitize.DefaultURIPolicy())

	rd := ResourceDefinition{
		Resource: mcp.Resource{URI: "https://example.com/data", Name: "Data"},
		Handler:  okHandler("content"),
	}

	wrapped := mw("https://example.com/data", rd, rd.Handler)
	_, err := wrapped(context.Background(), makeRequest("ftp://example.com/file.txt"))
	if err == nil {
		t.Fatal("expected error for disallowed scheme ftp://")
	}
}

func TestURIValidationMiddleware_CustomPolicy(t *testing.T) {
	policy := sanitize.URIPolicy{
		AllowedSchemes: []string{"myapp"},
		MaxLength:      256,
	}
	mw := URIValidationMiddleware(policy)

	rd := ResourceDefinition{
		Resource: mcp.Resource{URI: "myapp://resource/123", Name: "My Resource"},
		Handler:  okHandler("custom content"),
	}

	t.Run("custom scheme passes", func(t *testing.T) {
		wrapped := mw("myapp://resource/123", rd, rd.Handler)
		_, err := wrapped(context.Background(), makeRequest("myapp://resource/123"))
		if err != nil {
			t.Errorf("custom scheme should pass, got: %v", err)
		}
	})

	t.Run("https rejected by custom policy", func(t *testing.T) {
		wrapped := mw("myapp://resource/123", rd, rd.Handler)
		_, err := wrapped(context.Background(), makeRequest("https://example.com/"))
		if err == nil {
			t.Error("https should be rejected when not in custom allowed schemes")
		}
	})
}

func TestURIValidationMiddleware_FallbackToRegisteredURI(t *testing.T) {
	// When the request URI is empty, the middleware falls back to the registered URI.
	mw := URIValidationMiddleware(sanitize.DefaultURIPolicy())

	rd := ResourceDefinition{
		Resource: mcp.Resource{URI: "https://example.com/resource", Name: "Resource"},
		Handler:  okHandler("fallback"),
	}

	wrapped := mw("https://example.com/resource", rd, rd.Handler)
	// Send request with empty URI.
	emptyReq := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{}}
	_, err := wrapped(context.Background(), emptyReq)
	if err != nil {
		t.Fatalf("fallback to registered URI should pass, got: %v", err)
	}
}

func TestURIValidationMiddleware_ChainedWithRegistry(t *testing.T) {
	// Integration: register middleware through Config and verify it runs.
	reg := NewResourceRegistry(Config{
		Middleware: []Middleware{
			URIValidationMiddleware(sanitize.DefaultURIPolicy()),
		},
	})

	var handlerCalled bool
	reg.RegisterModule(&uriTestModule{
		name: "uritest",
		resources: []ResourceDefinition{
			{
				Resource: mcp.Resource{URI: "https://example.com/api", Name: "API"},
				Handler: func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
					handlerCalled = true
					return &mcp.ReadResourceResult{
						Contents: []*mcp.ResourceContents{
							{URI: req.Params.URI, Text: "ok"},
						},
					}, nil
				},
			},
		},
	})

	rd := reg.resources["https://example.com/api"]
	wrapped := reg.wrapHandler("https://example.com/api", rd)

	t.Run("valid URI reaches handler", func(t *testing.T) {
		handlerCalled = false
		_, err := wrapped(context.Background(), makeRequest("https://example.com/api"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !handlerCalled {
			t.Error("handler should have been called for valid URI")
		}
	})

	t.Run("invalid URI blocked before handler", func(t *testing.T) {
		handlerCalled = false
		_, err := wrapped(context.Background(), makeRequest("http://localhost/steal"))
		if err == nil {
			t.Fatal("expected error for SSRF URI")
		}
		if handlerCalled {
			t.Error("handler should NOT have been called for invalid URI")
		}
		var target interface{ Error() string }
		_ = errors.As(err, &target)
	})
}

func TestURIValidationMiddleware_NullByte(t *testing.T) {
	mw := URIValidationMiddleware(sanitize.DefaultURIPolicy())

	rd := ResourceDefinition{
		Resource: mcp.Resource{URI: "https://example.com/", Name: "Root"},
		Handler:  okHandler("ok"),
	}

	wrapped := mw("https://example.com/", rd, rd.Handler)
	_, err := wrapped(context.Background(), makeRequest("https://example.com/path\x00inject"))
	if err == nil {
		t.Fatal("expected error for null byte in URI")
	}
}
