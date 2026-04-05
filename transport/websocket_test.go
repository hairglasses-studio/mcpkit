package transport_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/transport"
)

func TestWebSocketTransport_Send_ReturnsError(t *testing.T) {
	t.Parallel()

	ws := transport.NewWebSocketTransport("ws://localhost:9999")
	err := ws.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error from stub Send")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWebSocketTransport_Receive_ReturnsChannel(t *testing.T) {
	t.Parallel()

	ws := transport.NewWebSocketTransport("ws://localhost:9999")
	ch := ws.Receive()
	if ch == nil {
		t.Fatal("expected non-nil channel from Receive")
	}
}

func TestWebSocketTransport_URL_Various(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"standard", "ws://localhost:8080"},
		{"secure", "wss://example.com/mcp"},
		{"with path", "ws://host:1234/api/v1"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ws := transport.NewWebSocketTransport(tt.url)
			if ws.URL() != tt.url {
				t.Errorf("expected URL %q, got %q", tt.url, ws.URL())
			}
		})
	}
}
