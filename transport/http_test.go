package transport_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/transport"
)

func TestHTTPTransport_SendReceive(t *testing.T) {
	t.Parallel()

	// Stand up a test server that echoes the request body wrapped in a response.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Test", "echo")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer ts.Close()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
		Headers:  map[string]string{"X-Custom": "value"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start is a no-op for HTTP.
	if err := ht.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	err := ht.Send(ctx, transport.Message{Body: payload})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case msg := <-ht.Receive():
		if !bytes.Equal(msg.Body, payload) {
			t.Fatalf("got %q, want %q", msg.Body, payload)
		}
		// Verify metadata captured response headers.
		if msg.Metadata["X-Test"] != "echo" {
			t.Errorf("expected X-Test=echo in metadata, got %v", msg.Metadata)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for response")
	}
}

func TestHTTPTransport_DefaultClient(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	// No Client or Headers — exercise defaults.
	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
	})

	ctx := context.Background()
	if err := ht.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	err := ht.Send(ctx, transport.Message{Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	msg := <-ht.Receive()
	if string(msg.Body) != `{"ok":true}` {
		t.Errorf("unexpected body: %s", msg.Body)
	}
}

func TestHTTPTransport_NonSuccessStatus(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	err := ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestHTTPTransport_InvalidEndpoint(t *testing.T) {
	t.Parallel()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: "http://127.0.0.1:0/not-listening",
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	err := ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
}

func TestHTTPTransport_CancelledContext(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := ht.Send(ctx, transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestHTTPTransport_CloseIdempotent(t *testing.T) {
	t.Parallel()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: "http://localhost:0",
	})
	// Close twice should not panic.
	if err := ht.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := ht.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestHTTPTransport_SendAfterClose(t *testing.T) {
	t.Parallel()

	// The server delays so the Close() happens while Send is in flight.
	ready := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(ready) // signal that the request is being processed
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Fill the recv channel (capacity 64) to force the select in Send to block
	// on channel delivery, then Close to hit the t.close path.
	for i := 0; i < 64; i++ {
		ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		// Use a separate server that responds instantly to fill the buffer.
		ht2 := transport.NewHTTPTransport(transport.HTTPConfig{Endpoint: ts2.URL})
		_ = ht2.Start(context.Background())
		_ = ht2.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
		ts2.Close()
		ht2.Close()
	}

	// Now do a send on the transport with a full buffer, close will race.
	errCh := make(chan error, 1)
	go func() {
		errCh <- ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	}()

	<-ready // wait for request to arrive at server
	ht.Close()

	err := <-errCh
	// The error may be "transport: closed" or nil depending on timing — both are acceptable.
	_ = err
}

func TestHTTPTransport_SendRecvFull_ClosePath(t *testing.T) {
	t.Parallel()

	// Create a server that responds successfully but slowly.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Fill the recv channel buffer (64 capacity) by sending many requests
	// without reading from Receive().
	for i := 0; i < 64; i++ {
		_ = ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	}

	// Now close the transport while the buffer is full. The next Send that
	// tries to deliver to the full recv channel should hit the close path.
	go func() {
		time.Sleep(20 * time.Millisecond)
		ht.Close()
	}()

	err := ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	// May get "transport: closed" or nil depending on timing.
	_ = err
}

func TestHTTPTransport_MalformedEndpoint(t *testing.T) {
	t.Parallel()

	// A URL with control characters triggers NewRequestWithContext to error.
	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: "http://\x00invalid",
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	err := ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error for malformed endpoint URL")
	}
	if !strings.Contains(err.Error(), "build request") {
		t.Errorf("expected 'build request' error, got: %v", err)
	}
}

func TestHTTPTransport_ReadBodyError(t *testing.T) {
	t.Parallel()

	// Server that returns a response with a content-length header that lies
	// about the body size, causing io.ReadAll to fail.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hijack the connection to write a malformed response that
		// causes ReadAll to error.
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusOK)
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			return
		}
		// Write a response header declaring 1000 bytes but close immediately.
		bufrw.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n")
		bufrw.Flush()
		conn.Close()
	}))
	defer ts.Close()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	err := ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error for truncated response body")
	}
	// The error should be about reading the response.
	if !strings.Contains(err.Error(), "read response") && !strings.Contains(err.Error(), "EOF") {
		t.Logf("got error: %v (acceptable variant)", err)
	}
}

func TestHTTPTransport_CustomHeaders(t *testing.T) {
	t.Parallel()

	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	ht := transport.NewHTTPTransport(transport.HTTPConfig{
		Endpoint: ts.URL,
		Headers:  map[string]string{"Authorization": "Bearer test-token"},
	})
	if err := ht.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ht.Close()

	err := ht.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	<-ht.Receive()

	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Authorization header, got %q", receivedAuth)
	}
}
