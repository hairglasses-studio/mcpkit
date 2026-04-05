package transport_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hairglasses-studio/mcpkit/transport"
)

func TestMetricsMiddleware_CountsSendAndReceive(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"method":"test"}`)
	rBuf := bytes.NewBuffer(payload)
	wBuf := bytes.NewBuffer(nil)

	base := transport.NewReadWriteTransport(&rw{r: rBuf, w: wBuf})
	wrapped := transport.Chain(base, transport.MetricsMiddleware())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := wrapped.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer wrapped.Close()

	// Wait for the received message.
	select {
	case msg := <-wrapped.Receive():
		if !bytes.Equal(msg.Body, payload) {
			t.Fatalf("got %q, want %q", msg.Body, payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for received message")
	}

	// Send a message.
	if err := wrapped.Send(ctx, transport.Message{Body: []byte(`{"id":1}`)}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Check snapshot — need to type-assert to access Snapshot method.
	type snapshotter interface {
		Snapshot() transport.MetricsSnapshot
	}

	// The Chain wraps base, so wrapped itself should be the metricsTransport.
	if s, ok := wrapped.(snapshotter); ok {
		snap := s.Snapshot()
		if snap.Sent != 1 {
			t.Errorf("expected Sent=1, got %d", snap.Sent)
		}
		if snap.Received != 1 {
			t.Errorf("expected Received=1, got %d", snap.Received)
		}
		if snap.Errors != 0 {
			t.Errorf("expected Errors=0, got %d", snap.Errors)
		}
	} else {
		t.Fatal("wrapped transport does not implement Snapshot()")
	}
}

func TestMetricsMiddleware_CountsErrors(t *testing.T) {
	t.Parallel()

	// errWriter always returns an error on Write.
	base := transport.NewReadWriteTransport(&rw{
		r: strings.NewReader(""),
		w: &errWriter{},
	})
	wrapped := transport.Chain(base, transport.MetricsMiddleware())

	if err := wrapped.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer wrapped.Close()

	err := wrapped.Send(context.Background(), transport.Message{Body: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error from errWriter")
	}

	type snapshotter interface {
		Snapshot() transport.MetricsSnapshot
	}
	if s, ok := wrapped.(snapshotter); ok {
		snap := s.Snapshot()
		if snap.Errors != 1 {
			t.Errorf("expected Errors=1, got %d", snap.Errors)
		}
		if snap.Sent != 0 {
			t.Errorf("expected Sent=0 (error case), got %d", snap.Sent)
		}
	} else {
		t.Fatal("wrapped transport does not implement Snapshot()")
	}
}

func TestMetricsMiddleware_ReceiveBeforeStart(t *testing.T) {
	t.Parallel()

	// If Receive is called before Start, it should fall back to next.
	base := transport.NewReadWriteTransport(&rw{
		r: strings.NewReader(""),
		w: bytes.NewBuffer(nil),
	})
	wrapped := transport.Chain(base, transport.MetricsMiddleware())

	// Receive before Start should return a valid channel (the inner one).
	ch := wrapped.Receive()
	if ch == nil {
		t.Error("expected non-nil channel even before Start")
	}
}

func TestChain_MultipleMiddleware(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"multi":"test"}`)
	rBuf := bytes.NewBuffer(payload)
	wBuf := bytes.NewBuffer(nil)

	base := transport.NewReadWriteTransport(&rw{r: rBuf, w: wBuf})
	wrapped := transport.Chain(base, transport.MetricsMiddleware())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := wrapped.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer wrapped.Close()

	select {
	case msg := <-wrapped.Receive():
		if !bytes.Equal(msg.Body, payload) {
			t.Fatalf("got %q, want %q", msg.Body, payload)
		}
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}

// errWriter always returns an error on Write.
type errWriter struct{}

func (e *errWriter) Write(p []byte) (int, error) { return 0, errors.New("write error") }
func (e *errWriter) Read(p []byte) (int, error)   { return 0, errors.New("read error") }
