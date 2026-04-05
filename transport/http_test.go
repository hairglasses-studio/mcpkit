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
