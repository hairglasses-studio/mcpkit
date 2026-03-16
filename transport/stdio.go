package transport

import (
	"context"
	"io"
	"os"
)

// StdioTransport is a Transport that communicates over stdin/stdout.
// This is the standard transport for MCP servers launched as child processes.
type StdioTransport struct {
	*ReadWriteTransport
}

// NewStdioTransport creates a StdioTransport using os.Stdin and os.Stdout.
func NewStdioTransport() *StdioTransport {
	return NewStdioTransportFromRW(os.Stdin, os.Stdout)
}

// NewStdioTransportFromRW creates a StdioTransport from arbitrary Reader/Writer.
// Useful for testing.
func NewStdioTransportFromRW(r io.Reader, w io.Writer) *StdioTransport {
	rw := &readWriter{r: r, w: w}
	return &StdioTransport{
		ReadWriteTransport: NewReadWriteTransport(rw),
	}
}

// readWriter combines a Reader and Writer into an io.ReadWriter.
type readWriter struct {
	r io.Reader
	w io.Writer
}

func (rw *readWriter) Read(p []byte) (int, error)  { return rw.r.Read(p) }
func (rw *readWriter) Write(p []byte) (int, error) { return rw.w.Write(p) }

// Start begins the stdio read loop.
func (t *StdioTransport) Start(ctx context.Context) error {
	return t.ReadWriteTransport.Start(ctx)
}
