package transport_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/transport"
)

// --------------------------------------------------------------------------
// ReadWriteTransport
// --------------------------------------------------------------------------

func TestReadWriteTransport_SendReceive(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	// Use a pipe so writes on one end appear as reads on the other.
	pr, pw := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	// We write the payload into the read buffer so the transport can read it.
	pr.Write(payload)

	rt := transport.NewReadWriteTransport(&rw{r: pr, w: pw})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rt.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer rt.Close()

	select {
	case msg := <-rt.Receive():
		if !bytes.Equal(msg.Body, payload) {
			t.Fatalf("got %q, want %q", msg.Body, payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for message")
	}
}

func TestReadWriteTransport_Send(t *testing.T) {
	wBuf := bytes.NewBuffer(nil)
	rt := transport.NewReadWriteTransport(&rw{r: strings.NewReader(""), w: wBuf})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer rt.Close()

	msg := transport.Message{Body: []byte(`{"hello":"world"}`)}
	if err := rt.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !bytes.Equal(wBuf.Bytes(), msg.Body) {
		t.Fatalf("written %q, want %q", wBuf.Bytes(), msg.Body)
	}
}

// --------------------------------------------------------------------------
// StdioTransport
// --------------------------------------------------------------------------

func TestStdioTransport_FromRW(t *testing.T) {
	payload := []byte(`{"method":"test"}`)
	rBuf := bytes.NewBuffer(payload)
	wBuf := bytes.NewBuffer(nil)

	st := transport.NewStdioTransportFromRW(rBuf, wBuf)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := st.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer st.Close()

	select {
	case msg := <-st.Receive():
		if !bytes.Equal(msg.Body, payload) {
			t.Fatalf("got %q, want %q", msg.Body, payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}

// --------------------------------------------------------------------------
// WebSocketTransport stub
// --------------------------------------------------------------------------

func TestWebSocketTransport_Stub(t *testing.T) {
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	if ws.URL() != "ws://localhost:8080" {
		t.Fatalf("unexpected URL: %s", ws.URL())
	}

	err := ws.Start(context.Background())
	if err == nil {
		t.Fatal("expected error from stub Start")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should not panic.
	if err := ws.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Double-close should be safe.
	if err := ws.Close(); err != nil {
		t.Fatalf("double Close: %v", err)
	}
}

// --------------------------------------------------------------------------
// Chain / middleware
// --------------------------------------------------------------------------

func TestChain_Identity(t *testing.T) {
	payload := []byte(`{"id":2}`)
	rBuf := bytes.NewBuffer(payload)
	wBuf := bytes.NewBuffer(nil)

	base := transport.NewReadWriteTransport(&rw{r: rBuf, w: wBuf})
	chained := transport.Chain(base) // no middleware → identity

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := chained.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer chained.Close()

	select {
	case msg := <-chained.Receive():
		if !bytes.Equal(msg.Body, payload) {
			t.Fatalf("got %q, want %q", msg.Body, payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	payload := []byte(`{"method":"log"}`)
	rBuf := bytes.NewBuffer(payload)
	wBuf := bytes.NewBuffer(nil)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	base := transport.NewReadWriteTransport(&rw{r: rBuf, w: wBuf})
	chained := transport.Chain(base, transport.LoggingMiddleware(logger))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := chained.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer chained.Close()

	// Send a message and verify it passes through.
	if err := chained.Send(ctx, transport.Message{Body: []byte(`{"out":1}`)}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case msg := <-chained.Receive():
		if !bytes.Equal(msg.Body, payload) {
			t.Fatalf("got %q, want %q", msg.Body, payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// rw implements io.ReadWriter using separate Reader and Writer.
type rw struct {
	r interface{ Read([]byte) (int, error) }
	w interface{ Write([]byte) (int, error) }
}

func (x *rw) Read(p []byte) (int, error)  { return x.r.Read(p) }
func (x *rw) Write(p []byte) (int, error) { return x.w.Write(p) }
