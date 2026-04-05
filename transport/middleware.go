package transport

import (
	"context"
	"log/slog"
)

// LoggingMiddleware returns a Middleware that logs each message send/receive
// using the provided slog.Logger.
func LoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next Transport) Transport {
		return &loggingTransport{next: next, logger: logger}
	}
}

type loggingTransport struct {
	next    Transport
	logger  *slog.Logger
	recv    chan Message
	started bool
}

func (t *loggingTransport) Start(ctx context.Context) error {
	if err := t.next.Start(ctx); err != nil {
		return err
	}
	t.recv = make(chan Message, 64)
	t.started = true
	go t.fanOut()
	return nil
}

func (t *loggingTransport) fanOut() {
	for msg := range t.next.Receive() {
		t.logger.Debug("transport: received message", "bytes", len(msg.Body))
		t.recv <- msg
	}
	close(t.recv)
}

func (t *loggingTransport) Send(ctx context.Context, msg Message) error {
	t.logger.Debug("transport: sending message", "bytes", len(msg.Body))
	return t.next.Send(ctx, msg)
}

func (t *loggingTransport) Receive() <-chan Message {
	if t.recv != nil {
		return t.recv
	}
	return t.next.Receive()
}

func (t *loggingTransport) Close() error {
	return t.next.Close()
}

// MetricsMiddleware returns a Middleware that records message counts using
// simple in-memory counters accessible via Snapshot.
func MetricsMiddleware() Middleware {
	return func(next Transport) Transport {
		return &metricsTransport{next: next}
	}
}

// MetricsSnapshot holds transport message counts.
type MetricsSnapshot struct {
	Sent     int64
	Received int64
	Errors   int64
}

type metricsTransport struct {
	next Transport
	snap MetricsSnapshot
	recv chan Message
}

func (t *metricsTransport) Start(ctx context.Context) error {
	if err := t.next.Start(ctx); err != nil {
		return err
	}
	t.recv = make(chan Message, 64)
	go t.fanOut()
	return nil
}

func (t *metricsTransport) fanOut() {
	for msg := range t.next.Receive() {
		t.snap.Received++
		t.recv <- msg
	}
	close(t.recv)
}

func (t *metricsTransport) Send(ctx context.Context, msg Message) error {
	err := t.next.Send(ctx, msg)
	if err != nil {
		t.snap.Errors++
	} else {
		t.snap.Sent++
	}
	return err
}

func (t *metricsTransport) Receive() <-chan Message {
	if t.recv != nil {
		return t.recv
	}
	return t.next.Receive()
}

func (t *metricsTransport) Close() error {
	return t.next.Close()
}

// Snapshot returns a point-in-time copy of the transport metrics.
func (t *metricsTransport) Snapshot() MetricsSnapshot {
	return t.snap
}
