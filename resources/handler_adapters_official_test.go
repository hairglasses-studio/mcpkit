//go:build official_sdk

package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTextResourceHandler(t *testing.T) {
	h := TextResourceHandler(func(_ context.Context, uri string) (string, string, error) {
		return "text/markdown", "hello " + uri, nil
	})
	result, err := h(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "test://doc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content entry, got %d", len(result.Contents))
	}
	c := result.Contents[0]
	if c.URI != "test://doc" || c.MIMEType != "text/markdown" || c.Text != "hello test://doc" {
		t.Errorf("unexpected content: %+v", c)
	}
}

func TestTextResourceHandler_Error(t *testing.T) {
	wantErr := errors.New("boom")
	h := TextResourceHandler(func(_ context.Context, _ string) (string, string, error) {
		return "", "", wantErr
	})
	_, err := h(context.Background(), &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "test://doc"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped wantErr, got %v", err)
	}
}

func TestJSONResourceHandler(t *testing.T) {
	h := JSONResourceHandler(func(_ context.Context, uri string) (any, error) {
		return map[string]any{"uri": uri, "ok": true}, nil
	})
	result, err := h(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "test://json"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content entry, got %d", len(result.Contents))
	}
	c := result.Contents[0]
	if c.MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want application/json", c.MIMEType)
	}
	if c.Text != `{"ok":true,"uri":"test://json"}` {
		t.Errorf("Text = %q", c.Text)
	}
}

func TestJSONResourceHandler_Error(t *testing.T) {
	wantErr := errors.New("boom")
	h := JSONResourceHandler(func(_ context.Context, _ string) (any, error) {
		return nil, wantErr
	})
	_, err := h(context.Background(), &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "test://json"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped wantErr, got %v", err)
	}
}
