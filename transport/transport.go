package transport

import (
	"context"
	"io"
)

// Message represents a raw transport message with optional metadata.
type Message struct {
	// Body is the raw message bytes (typically JSON-encoded JSON-RPC).
	Body []byte
	// Metadata holds transport-specific metadata (e.g. HTTP headers).
	Metadata map[string]string
}

// Transport is the interface that all transport adapters implement.
// A Transport manages the low-level sending and receiving of messages.
type Transport interface {
	// Start begins accepting or initiating connections.
	// It blocks until the transport is ready to send/receive.
	Start(ctx context.Context) error

	// Send sends a message to the remote peer.
	Send(ctx context.Context, msg Message) error

	// Receive returns a channel that yields incoming messages.
	// The channel is closed when the transport is stopped.
	Receive() <-chan Message

	// Close shuts down the transport and releases resources.
	Close() error
}

// Middleware is a function that wraps a Transport to add behaviour.
type Middleware func(Transport) Transport

// Chain applies a sequence of middleware to a base Transport.
// Middleware is applied left-to-right: Chain(t, m1, m2) produces m1(m2(t)).
func Chain(base Transport, middleware ...Middleware) Transport {
	for i := len(middleware) - 1; i >= 0; i-- {
		base = middleware[i](base)
	}
	return base
}

// ReadWriteTransport is a Transport backed by an io.ReadWriter.
// It is the common base for stdio and in-process transports.
type ReadWriteTransport struct {
	rw     io.ReadWriter
	recv   chan Message
	close  chan struct{}
	closed bool
}

// NewReadWriteTransport creates a Transport that reads/writes from rw.
func NewReadWriteTransport(rw io.ReadWriter) *ReadWriteTransport {
	return &ReadWriteTransport{
		rw:    rw,
		recv:  make(chan Message, 64),
		close: make(chan struct{}),
	}
}

// Start begins the read loop in a goroutine.
func (t *ReadWriteTransport) Start(_ context.Context) error {
	go t.readLoop()
	return nil
}

func (t *ReadWriteTransport) readLoop() {
	defer close(t.recv)
	buf := make([]byte, 4096)
	for {
		select {
		case <-t.close:
			return
		default:
		}
		n, err := t.rw.Read(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		body := make([]byte, n)
		copy(body, buf[:n])
		select {
		case t.recv <- Message{Body: body}:
		case <-t.close:
			return
		}
	}
}

// Send writes the message body to the underlying ReadWriter.
func (t *ReadWriteTransport) Send(_ context.Context, msg Message) error {
	_, err := t.rw.Write(msg.Body)
	return err
}

// Receive returns the incoming message channel.
func (t *ReadWriteTransport) Receive() <-chan Message {
	return t.recv
}

// Close stops the transport.
func (t *ReadWriteTransport) Close() error {
	if !t.closed {
		t.closed = true
		close(t.close)
	}
	return nil
}
