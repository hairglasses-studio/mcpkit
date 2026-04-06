package transport_test

import (
	"bytes"
	"context"
	"io"
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

func TestWebSocketTransport_NoConn(t *testing.T) {
	ws := transport.NewWebSocketTransport("ws://localhost:8080")
	if ws.URL() != "ws://localhost:8080" {
		t.Fatalf("unexpected URL: %s", ws.URL())
	}

	err := ws.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when starting without conn")
	}
	if !strings.Contains(err.Error(), "not set") {
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
// ReadWriteTransport — close during read (readLoop close path)
// --------------------------------------------------------------------------

func TestReadWriteTransport_CloseStopsReadLoop(t *testing.T) {
	// Use a blocking reader so the readLoop blocks on Read, then close the
	// transport to exercise the close path in the select.
	blockReader := &blockingReader{block: make(chan struct{})}
	wBuf := bytes.NewBuffer(nil)

	rt := transport.NewReadWriteTransport(&rw{r: blockReader, w: wBuf})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rt.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Close the transport; readLoop should exit and close the recv channel.
	rt.Close()

	select {
	case _, ok := <-rt.Receive():
		if ok {
			t.Error("expected channel to be closed after Close")
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for recv channel close")
	}
}

func TestReadWriteTransport_DoubleClose(t *testing.T) {
	wBuf := bytes.NewBuffer(nil)
	rt := transport.NewReadWriteTransport(&rw{r: strings.NewReader(""), w: wBuf})

	// Double close should not panic.
	if err := rt.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// --------------------------------------------------------------------------
// StdioTransport — exercise readWriter.Write path
// --------------------------------------------------------------------------

func TestStdioTransport_SendExercisesWrite(t *testing.T) {
	rBuf := strings.NewReader("")
	wBuf := bytes.NewBuffer(nil)

	st := transport.NewStdioTransportFromRW(rBuf, wBuf)
	if err := st.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer st.Close()

	payload := []byte(`{"method":"write-test"}`)
	if err := st.Send(context.Background(), transport.Message{Body: payload}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !bytes.Equal(wBuf.Bytes(), payload) {
		t.Errorf("written %q, want %q", wBuf.Bytes(), payload)
	}
}

// --------------------------------------------------------------------------
// ReadWriteTransport — close during channel send (readLoop close path)
// --------------------------------------------------------------------------

func TestReadWriteTransport_CloseDuringChannelSend(t *testing.T) {
	// This test exercises the case where readLoop reads data but the recv
	// channel is full, so the select falls through to the close case.
	// We use a reader that produces many messages to fill the 64-capacity buffer.
	reader := &infiniteReader{}
	wBuf := bytes.NewBuffer(nil)

	rt := transport.NewReadWriteTransport(&rw{r: reader, w: wBuf})
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Let the reader fill up the buffer.
	time.Sleep(20 * time.Millisecond)

	// Close while buffer is full — readLoop should hit the close select case.
	rt.Close()

	// Drain whatever is left.
	for range rt.Receive() {
	}
}

// --------------------------------------------------------------------------
// ReadWriteTransport — zero-length read (n == 0, continue path)
// --------------------------------------------------------------------------

func TestReadWriteTransport_ZeroLengthRead(t *testing.T) {
	// zeroThenDataReader returns 0 bytes on the first read, then real data,
	// then EOF. This exercises the n == 0 continue path in readLoop.
	reader := &zeroThenDataReader{
		data: []byte(`{"zero":"test"}`),
	}
	wBuf := bytes.NewBuffer(nil)

	rt := transport.NewReadWriteTransport(&rw{r: reader, w: wBuf})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rt.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer rt.Close()

	// Despite the zero-read, we should still get the actual data.
	select {
	case msg := <-rt.Receive():
		if !bytes.Equal(msg.Body, reader.data) {
			t.Fatalf("got %q, want %q", msg.Body, reader.data)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for message after zero-length read")
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

// blockingReader blocks on Read until the block channel is closed.
type blockingReader struct {
	block chan struct{}
}

func (b *blockingReader) Read(_ []byte) (int, error) {
	<-b.block
	return 0, os.ErrClosed
}

// infiniteReader produces data on every Read call (never EOF).
type infiniteReader struct{}

func (r *infiniteReader) Read(p []byte) (int, error) {
	data := []byte(`{"infinite":true}`)
	n := copy(p, data)
	return n, nil
}

// zeroThenDataReader returns 0 bytes on first read, then the data, then EOF.
type zeroThenDataReader struct {
	data    []byte
	callNum int
}

func (z *zeroThenDataReader) Read(p []byte) (int, error) {
	z.callNum++
	switch z.callNum {
	case 1:
		return 0, nil // zero-length read
	case 2:
		n := copy(p, z.data)
		return n, nil
	default:
		return 0, io.EOF
	}
}
