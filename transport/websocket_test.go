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

func TestWebSocketTransport_ContextCancellation(t *testing.T) {
	t.Parallel()

	// blockingWSConn blocks on ReadMessage until context is cancelled.
	conn := &blockingWSConn{block: make(chan struct{})}
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	ctx, cancel := context.WithCancel(context.Background())

	if err := ws.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel the context — readLoop should exit via ctx.Done().
	cancel()

	// The receive channel should close.
	ch := ws.Receive()
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to close on context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close after context cancellation")
	}

	ws.Close()
}

// blockingWSConn blocks ReadMessage until Close or context cancellation.
type blockingWSConn struct {
	mu     sync.Mutex
	block  chan struct{}
	closed bool
}

func (b *blockingWSConn) ReadMessage(ctx context.Context) (int, []byte, error) {
	select {
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case <-b.block:
		return 0, nil, errors.New("closed")
	}
}

func (b *blockingWSConn) WriteMessage(_ context.Context, _ int, _ []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("closed")
	}
	return nil
}

func (b *blockingWSConn) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.block)
	}
	return nil
}

func TestWebSocketTransport_EmptyMessage(t *testing.T) {
	t.Parallel()

	// Queue an empty message followed by a real message.
	conn := &mockWSConn{
		messages: [][]byte{nil, []byte(`{"id":99}`)},
	}
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ws.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// The empty message should be skipped; we should receive the real one.
	ch := ws.Receive()
	select {
	case msg := <-ch:
		if string(msg.Body) != `{"id":99}` {
			t.Errorf("expected {\"id\":99}, got %q", msg.Body)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for message after empty skip")
	}

	ws.Close()
}

func TestWebSocketTransport_DoneChannelDuringDelivery(t *testing.T) {
	t.Parallel()

	// Slow consumer: fill the recv channel buffer (64), then close via done.
	msgs := make([][]byte, 70)
	for i := range msgs {
		msgs[i] = []byte(`{"i":1}`)
	}
	conn := &mockWSConn{messages: msgs}
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ws.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let the readLoop fill the buffer, then close.
	time.Sleep(50 * time.Millisecond)
	ws.Close()

	// Drain whatever is in the channel.
	for range ws.Receive() {
	}
}

func TestWebSocketTransport_DoneBeforeRead(t *testing.T) {
	t.Parallel()

	// slowWSConn reads a message then delays, giving time for Close() to
	// fire the done channel before the next ReadMessage call.
	conn := newSlowWSConn([][]byte{[]byte(`{"first":1}`)}, 100*time.Millisecond)
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ws.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Read the first message.
	ch := ws.Receive()
	select {
	case msg := <-ch:
		if string(msg.Body) != `{"first":1}` {
			t.Errorf("unexpected message: %q", msg.Body)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for first message")
	}

	// Close the transport — the readLoop should hit t.done in the top select.
	ws.Close()

	// Receive channel should close.
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to close after Close()")
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for channel close")
	}
}

// slowWSConn returns messages with a delay between reads.
type slowWSConn struct {
	mu       sync.Mutex
	messages [][]byte
	readIdx  int
	delay    time.Duration
	done     chan struct{}
}

func newSlowWSConn(messages [][]byte, delay time.Duration) *slowWSConn {
	return &slowWSConn{
		messages: messages,
		delay:    delay,
		done:     make(chan struct{}),
	}
}

func (s *slowWSConn) ReadMessage(ctx context.Context) (int, []byte, error) {
	s.mu.Lock()
	idx := s.readIdx
	s.readIdx++
	s.mu.Unlock()

	if idx > 0 && s.delay > 0 {
		time.Sleep(s.delay)
	}

	s.mu.Lock()
	if idx >= len(s.messages) {
		s.mu.Unlock()
		// Block until closed or context done.
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case <-s.done:
			return 0, nil, errors.New("closed")
		}
	}
	data := s.messages[idx]
	s.mu.Unlock()
	return 1, data, nil
}

func (s *slowWSConn) WriteMessage(_ context.Context, _ int, _ []byte) error {
	return nil
}

func (s *slowWSConn) Close() error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return nil
}

func TestWebSocketTransport_CtxDoneDuringDelivery(t *testing.T) {
	t.Parallel()

	// Produce many messages to fill recv buffer, then cancel context.
	msgs := make([][]byte, 70)
	for i := range msgs {
		msgs[i] = []byte(`{"fill":true}`)
	}
	conn := &mockWSConn{messages: msgs}
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	ws.SetConn(conn)

	ctx, cancel := context.WithCancel(context.Background())

	if err := ws.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let the buffer fill, then cancel context.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Drain.
	for range ws.Receive() {
	}

	ws.Close()
}
