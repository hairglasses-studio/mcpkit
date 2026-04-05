package transport_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/transport"
)

// mockWSConn implements WebSocketConn for testing.
type mockWSConn struct {
	mu       sync.Mutex
	messages [][]byte // queued incoming messages
	sent     [][]byte // captured outgoing messages
	closed   bool
	readIdx  int
	readErr  error // injected read error
}

func (m *mockWSConn) ReadMessage(_ context.Context) (int, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return 0, nil, m.readErr
	}
	if m.readIdx >= len(m.messages) {
		// Block until closed — simulate a real connection
		return 0, nil, errors.New("connection closed")
	}
	data := m.messages[m.readIdx]
	m.readIdx++
	return 1, data, nil
}

func (m *mockWSConn) WriteMessage(_ context.Context, _ int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("connection closed")
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.sent = append(m.sent, cp)
	return nil
}

func (m *mockWSConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func TestWebSocketTransport_StartWithoutConn(t *testing.T) {
	t.Parallel()
	ws := transport.NewWebSocketTransport("ws://localhost:9999")
	err := ws.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when starting without conn")
	}
	if !strings.Contains(err.Error(), "not set") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWebSocketTransport_SendWithoutConn(t *testing.T) {
	t.Parallel()
	ws := transport.NewWebSocketTransport("ws://localhost:9999")
	err := ws.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error when sending without conn")
	}
}

func TestWebSocketTransport_SendAndReceive(t *testing.T) {
	t.Parallel()
	conn := &mockWSConn{
		messages: [][]byte{[]byte(`{"id":1}`), []byte(`{"id":2}`)},
	}
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ws.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Receive 2 messages
	ch := ws.Receive()
	msg1 := <-ch
	if string(msg1.Body) != `{"id":1}` {
		t.Errorf("msg1 = %q, want {\"id\":1}", msg1.Body)
	}
	msg2 := <-ch
	if string(msg2.Body) != `{"id":2}` {
		t.Errorf("msg2 = %q, want {\"id\":2}", msg2.Body)
	}

	// Send a message
	err := ws.Send(ctx, transport.Message{Body: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	conn.mu.Lock()
	if len(conn.sent) != 1 || string(conn.sent[0]) != `{"ok":true}` {
		t.Errorf("sent = %v, want [{\"ok\":true}]", conn.sent)
	}
	conn.mu.Unlock()

	ws.Close()
}

func TestWebSocketTransport_Close(t *testing.T) {
	t.Parallel()
	conn := &mockWSConn{}
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	if err := ws.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	// Double close should be safe
	if err := ws.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}

	conn.mu.Lock()
	if !conn.closed {
		t.Error("expected conn to be closed")
	}
	conn.mu.Unlock()
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

func TestWebSocketTransport_ReadError(t *testing.T) {
	t.Parallel()
	conn := &mockWSConn{
		readErr: errors.New("network error"),
	}
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ws.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Channel should close when read error occurs
	ch := ws.Receive()
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to close on read error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}
