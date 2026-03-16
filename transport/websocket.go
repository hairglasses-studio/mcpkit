package transport

import (
	"context"
	"fmt"
)

// WebSocketTransport is a stub Transport for WebSocket communication.
// A full implementation will integrate with a WebSocket library in a
// future phase; this stub satisfies the Transport interface and allows
// packages that import transport to compile today.
type WebSocketTransport struct {
	url  string
	recv chan Message
	done chan struct{}
}

// NewWebSocketTransport creates a WebSocketTransport for the given URL.
// The transport is not yet connected; call Start to establish the connection.
func NewWebSocketTransport(url string) *WebSocketTransport {
	return &WebSocketTransport{
		url:  url,
		recv: make(chan Message, 64),
		done: make(chan struct{}),
	}
}

// Start is a stub that returns an error indicating WebSocket is not yet
// implemented. This is intentional – callers can detect the stub and fall
// back to HTTP or stdio.
func (t *WebSocketTransport) Start(_ context.Context) error {
	return fmt.Errorf("transport: WebSocket transport not yet implemented")
}

// Send is a stub that always returns an error.
func (t *WebSocketTransport) Send(_ context.Context, _ Message) error {
	return fmt.Errorf("transport: WebSocket transport not yet implemented")
}

// Receive returns the (empty) incoming message channel.
func (t *WebSocketTransport) Receive() <-chan Message {
	return t.recv
}

// Close closes the transport.
func (t *WebSocketTransport) Close() error {
	select {
	case <-t.done:
	default:
		close(t.done)
	}
	return nil
}

// URL returns the WebSocket URL this transport was configured for.
func (t *WebSocketTransport) URL() string {
	return t.url
}
