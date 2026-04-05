package transport

import (
	"context"
	"fmt"
	"sync"
)

// WebSocketConn is a thin interface representing a WebSocket connection.
// Users inject their preferred library (nhooyr.io/websocket, gorilla/websocket,
// etc.) by implementing this interface. This avoids forcing a transitive
// dependency on all mcpkit consumers.
type WebSocketConn interface {
	// ReadMessage blocks until a message is received or the connection closes.
	// messageType is 1 for text, 2 for binary (matching standard WebSocket opcodes).
	ReadMessage(ctx context.Context) (messageType int, data []byte, err error)
	// WriteMessage sends a message. messageType should be 1 (text) or 2 (binary).
	WriteMessage(ctx context.Context, messageType int, data []byte) error
	// Close sends a close frame and closes the connection.
	Close() error
}

// WebSocketTransport is a Transport backed by a WebSocket connection.
// It requires a WebSocketConn provided by the caller.
type WebSocketTransport struct {
	url  string
	conn WebSocketConn
	recv chan Message
	done chan struct{}

	closeOnce sync.Once
	writeMu   sync.Mutex
}

// NewWebSocketTransport creates a WebSocketTransport for the given URL.
// Pass a nil conn to create a placeholder; call SetConn before Start.
func NewWebSocketTransport(url string) *WebSocketTransport {
	return &WebSocketTransport{
		url:  url,
		recv: make(chan Message, 64),
		done: make(chan struct{}),
	}
}

// SetConn sets the WebSocket connection. Must be called before Start.
func (t *WebSocketTransport) SetConn(conn WebSocketConn) {
	t.conn = conn
}

// Start begins reading messages from the WebSocket in a goroutine.
func (t *WebSocketTransport) Start(ctx context.Context) error {
	if t.conn == nil {
		return fmt.Errorf("transport: WebSocket connection not set — call SetConn before Start")
	}
	go t.readLoop(ctx)
	return nil
}

func (t *WebSocketTransport) readLoop(ctx context.Context) {
	defer close(t.recv)
	for {
		select {
		case <-t.done:
			return
		case <-ctx.Done():
			return
		default:
		}
		_, data, err := t.conn.ReadMessage(ctx)
		if err != nil {
			return
		}
		if len(data) == 0 {
			continue
		}
		select {
		case t.recv <- Message{Body: data}:
		case <-t.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Send writes a text message to the WebSocket.
func (t *WebSocketTransport) Send(ctx context.Context, msg Message) error {
	if t.conn == nil {
		return fmt.Errorf("transport: WebSocket connection not set")
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return t.conn.WriteMessage(ctx, 1, msg.Body) // 1 = text
}

// Receive returns the incoming message channel.
func (t *WebSocketTransport) Receive() <-chan Message {
	return t.recv
}

// Close closes the transport and the underlying connection.
func (t *WebSocketTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.done)
		if t.conn != nil {
			err = t.conn.Close()
		}
	})
	return err
}

// URL returns the WebSocket URL this transport was configured for.
func (t *WebSocketTransport) URL() string {
	return t.url
}
