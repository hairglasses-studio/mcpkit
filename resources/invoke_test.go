package resources

import (
	"context"
	"errors"
	"testing"
)

// TestCallHandlerText exercises the pattern secretstudios-mcp's
// server_catalog_test.go/surface_modules_test.go/roadmap_wave_resource_test.go
// all use: invoke a resource handler directly and read back the first
// content entry's text. Built on the TextResourceHandler adapter (also this
// round's work) so the fixture itself is portable across both tags too.
func TestCallHandlerText(t *testing.T) {
	h := TextResourceHandler(func(_ context.Context, uri string) (string, string, error) {
		return "application/json", `{"uri":"` + uri + `"}`, nil
	})

	mimeType, text, err := CallHandlerText(context.Background(), h, "test://catalog")
	if err != nil {
		t.Fatalf("CallHandlerText: %v", err)
	}
	if mimeType != "application/json" {
		t.Errorf("mimeType = %q, want application/json", mimeType)
	}
	if text != `{"uri":"test://catalog"}` {
		t.Errorf("text = %q", text)
	}
}

func TestCallHandlerText_HandlerError(t *testing.T) {
	wantErr := errors.New("boom")
	h := TextResourceHandler(func(_ context.Context, _ string) (string, string, error) {
		return "", "", wantErr
	})
	_, _, err := CallHandlerText(context.Background(), h, "test://err")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped wantErr, got %v", err)
	}
}
