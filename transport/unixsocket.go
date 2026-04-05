package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// UnixSocketServer wraps an MCP server to accept multiple concurrent connections
// on a Unix domain socket instead of a single stdio pair.
//
// Each connection gets its own goroutine that reads newline-delimited JSON-RPC
// messages from the socket and dispatches them to the shared MCPServer. This
// means one server process with one set of tool handlers in memory serves all
// connected clients. Connections are independent — no cross-talk.
type UnixSocketServer struct {
	socketPath string
	server     *server.MCPServer
	listener   net.Listener
	logger     *slog.Logger

	mu      sync.Mutex
	clients map[net.Conn]context.CancelFunc

	sessionCounter atomic.Int64
	shutdownOnce   sync.Once
}

// NewUnixSocketServer creates a new UnixSocketServer that will listen on the
// given socket path. The server is not yet listening; call Serve to start.
func NewUnixSocketServer(s *server.MCPServer, socketPath string, opts ...UnixSocketOption) *UnixSocketServer {
	us := &UnixSocketServer{
		socketPath: socketPath,
		server:     s,
		clients:    make(map[net.Conn]context.CancelFunc),
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(us)
	}
	return us
}

// UnixSocketOption configures a UnixSocketServer.
type UnixSocketOption func(*UnixSocketServer)

// WithLogger sets the logger for the unix socket server.
func WithLogger(logger *slog.Logger) UnixSocketOption {
	return func(s *UnixSocketServer) {
		s.logger = logger
	}
}

// Serve starts the accept loop. It blocks until ctx is cancelled or a fatal
// listener error occurs. Each accepted connection is handled in its own
// goroutine.
func (s *UnixSocketServer) Serve(ctx context.Context) error {
	// Ensure parent directory exists.
	dir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("unixsocket: create socket dir: %w", err)
	}

	// Remove stale socket file if it exists.
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unixsocket: remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("unixsocket: listen: %w", err)
	}
	s.listener = ln

	s.logger.Info("unix socket server listening", "path", s.socketPath)

	// Close the listener when the context is done so Accept unblocks.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Check if this is due to shutdown.
			select {
			case <-ctx.Done():
				wg.Wait()
				return ctx.Err()
			default:
			}
			s.logger.Error("unixsocket: accept error", "err", err)
			// Permanent listener failure.
			wg.Wait()
			return fmt.Errorf("unixsocket: accept: %w", err)
		}

		connCtx, cancel := context.WithCancel(ctx)
		s.mu.Lock()
		s.clients[conn] = cancel
		s.mu.Unlock()

		wg.Go(func() {
			defer cancel()
			s.handleConn(connCtx, conn)
			s.mu.Lock()
			delete(s.clients, conn)
			s.mu.Unlock()
		})
	}
}

// Shutdown gracefully shuts down the server: stops accepting new connections,
// cancels all active client contexts, and removes the socket file.
func (s *UnixSocketServer) Shutdown() error {
	var firstErr error
	s.shutdownOnce.Do(func() {
		if s.listener != nil {
			if err := s.listener.Close(); err != nil {
				firstErr = err
			}
		}
		s.mu.Lock()
		for conn, cancel := range s.clients {
			cancel()
			_ = conn.Close()
		}
		s.clients = make(map[net.Conn]context.CancelFunc)
		s.mu.Unlock()
		_ = os.Remove(s.socketPath)
	})
	return firstErr
}

// socketSession implements the mcp-go ClientSession interface for a single
// socket connection. Each connection gets its own session so the MCPServer
// can track clients independently.
type socketSession struct {
	id            string
	notifications chan mcp.JSONRPCNotification
	initialized   atomic.Bool
	loggingLevel  atomic.Value
	clientInfo    atomic.Value
	clientCaps    atomic.Value
}

func newSocketSession(id string) *socketSession {
	ss := &socketSession{
		id:            id,
		notifications: make(chan mcp.JSONRPCNotification, 100),
	}
	ss.loggingLevel.Store(mcp.LoggingLevelError)
	return ss
}

func (s *socketSession) SessionID() string                                   { return s.id }
func (s *socketSession) NotificationChannel() chan<- mcp.JSONRPCNotification { return s.notifications }
func (s *socketSession) Initialize()                                         { s.initialized.Store(true) }
func (s *socketSession) Initialized() bool                                   { return s.initialized.Load() }

func (s *socketSession) SetLogLevel(level mcp.LoggingLevel) { s.loggingLevel.Store(level) }
func (s *socketSession) GetLogLevel() mcp.LoggingLevel {
	if v := s.loggingLevel.Load(); v != nil {
		if l, ok := v.(mcp.LoggingLevel); ok {
			return l
		}
	}
	return mcp.LoggingLevelError
}

func (s *socketSession) GetClientInfo() mcp.Implementation {
	if v := s.clientInfo.Load(); v != nil {
		if ci, ok := v.(mcp.Implementation); ok {
			return ci
		}
	}
	return mcp.Implementation{}
}
func (s *socketSession) SetClientInfo(ci mcp.Implementation) { s.clientInfo.Store(ci) }
func (s *socketSession) GetClientCapabilities() mcp.ClientCapabilities {
	if v := s.clientCaps.Load(); v != nil {
		if cc, ok := v.(mcp.ClientCapabilities); ok {
			return cc
		}
	}
	return mcp.ClientCapabilities{}
}
func (s *socketSession) SetClientCapabilities(cc mcp.ClientCapabilities) { s.clientCaps.Store(cc) }

// Compile-time interface checks.
var (
	_ server.ClientSession         = (*socketSession)(nil)
	_ server.SessionWithLogging    = (*socketSession)(nil)
	_ server.SessionWithClientInfo = (*socketSession)(nil)
)

// handleConn processes a single socket connection. It registers a session,
// reads newline-delimited JSON-RPC messages, dispatches them to the MCPServer,
// and writes responses back. Notifications from the server are forwarded to
// the client via a separate goroutine.
func (s *UnixSocketServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	sessionID := fmt.Sprintf("unix-%d", s.sessionCounter.Add(1))
	session := newSocketSession(sessionID)

	if err := s.server.RegisterSession(ctx, session); err != nil {
		s.logger.Error("unixsocket: register session", "err", err, "session", sessionID)
		return
	}
	defer s.server.UnregisterSession(ctx, sessionID)

	sCtx := s.server.WithContext(ctx, session)

	// Writer mutex protects concurrent writes to the connection (responses +
	// notifications can race).
	var writeMu sync.Mutex

	// Forward server-initiated notifications to this connection.
	go func() {
		for {
			select {
			case notif, ok := <-session.notifications:
				if !ok {
					return
				}
				data, err := json.Marshal(notif)
				if err != nil {
					s.logger.Error("unixsocket: marshal notification", "err", err)
					continue
				}
				data = append(data, '\n')
				writeMu.Lock()
				_, _ = conn.Write(data)
				writeMu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	reader := bufio.NewReader(conn)
	for {
		if err := ctx.Err(); err != nil {
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				s.logger.Error("unixsocket: read", "err", err, "session", sessionID)
			}
			return
		}
		if len(line) == 0 {
			continue
		}

		var rawMessage json.RawMessage
		if err := json.Unmarshal([]byte(line), &rawMessage); err != nil {
			resp := createJSONRPCError(nil, -32700, "Parse error")
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			writeMu.Lock()
			_, _ = conn.Write(data)
			writeMu.Unlock()
			continue
		}

		response := s.server.HandleMessage(sCtx, rawMessage)
		if response != nil {
			data, err := json.Marshal(response)
			if err != nil {
				s.logger.Error("unixsocket: marshal response", "err", err)
				continue
			}
			data = append(data, '\n')
			writeMu.Lock()
			_, writeErr := conn.Write(data)
			writeMu.Unlock()
			if writeErr != nil {
				s.logger.Error("unixsocket: write response", "err", writeErr)
				return
			}
		}
	}
}

// jsonRPCError is a minimal JSON-RPC error response for parse errors.
type jsonRPCError struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func createJSONRPCError(id any, code int, message string) jsonRPCError {
	return jsonRPCError{
		JSONRPC: "2.0",
		ID:      id,
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{
			Code:    code,
			Message: message,
		},
	}
}

// WriteJSONLine marshals v as JSON and writes it as a newline-terminated line to w.
// The write is protected by mu if non-nil.
func WriteJSONLine(w io.Writer, mu *sync.Mutex, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	_, err = w.Write(data)
	return err
}

// DefaultSocketPath returns the default socket path for a given server name.
// It uses $XDG_RUNTIME_DIR/mcpkit/<name>.sock, falling back to /tmp/mcpkit/<name>.sock.
func DefaultSocketPath(name string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "mcpkit", name+".sock")
}
