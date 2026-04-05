package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// HTTPTransport is a Transport that sends messages over HTTP POST requests
// and receives messages via server-sent events or long-polling.
//
// For MCP use-cases this provides a stateless request/response adapter:
// each Send call posts a JSON-RPC request and the response body is
// delivered on the Receive channel.
type HTTPTransport struct {
	mu       sync.Mutex
	client   *http.Client
	endpoint string
	headers  map[string]string
	recv     chan Message
	close    chan struct{}
	closed   bool
}

// HTTPConfig configures an HTTPTransport.
type HTTPConfig struct {
	// Endpoint is the URL to POST messages to.
	Endpoint string
	// Headers are additional HTTP headers to set on each request.
	Headers map[string]string
	// Client is an optional HTTP client. Defaults to http.DefaultClient.
	Client *http.Client
}

// NewHTTPTransport creates an HTTPTransport from the given config.
func NewHTTPTransport(cfg HTTPConfig) *HTTPTransport {
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	headers := cfg.Headers
	if headers == nil {
		headers = make(map[string]string)
	}
	return &HTTPTransport{
		client:   client,
		endpoint: cfg.Endpoint,
		headers:  headers,
		recv:     make(chan Message, 64),
		close:    make(chan struct{}),
	}
}

// Start initialises the transport. For HTTP this is a no-op.
func (t *HTTPTransport) Start(_ context.Context) error {
	return nil
}

// Send POSTs msg.Body to the configured endpoint and places the response
// body on the Receive channel.
func (t *HTTPTransport) Send(ctx context.Context, msg Message) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(msg.Body))
	if err != nil {
		return fmt.Errorf("transport: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("transport: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("transport: http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("transport: read response: %w", err)
	}

	// Build metadata from response headers.
	meta := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		meta[k] = resp.Header.Get(k)
	}

	select {
	case t.recv <- Message{Body: body, Metadata: meta}:
	case <-t.close:
		return fmt.Errorf("transport: closed")
	}
	return nil
}

// Receive returns the incoming message channel.
func (t *HTTPTransport) Receive() <-chan Message {
	return t.recv
}

// Close shuts down the transport.
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		close(t.close)
	}
	return nil
}
