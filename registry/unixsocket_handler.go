//go:build !official_sdk

// unixsocket_handler.go provides the mcp-go variant of the MCP ConnHandler
// for Unix socket transport. It bridges the transport layer with the
// mcp-go server by implementing session management and JSON-RPC dispatch.
package registry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/hairglasses-studio/mcpkit/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPConnHandler adapts an MCPServer to the transport.ConnHandler interface.
// Each connection gets its own mcp-go session so the server can track
// clients independently while sharing tool handlers.
type MCPConnHandler struct {
	Server *MCPServer
}

// NewMCPConnHandler creates a ConnHandler that dispatches to the given MCPServer.
func NewMCPConnHandler(s *MCPServer) *MCPConnHandler {
	return &MCPConnHandler{Server: s}
}

// Handle processes a single Unix socket connection. It registers a session,
// reads newline-delimited JSON-RPC messages, dispatches them to the MCPServer,
// and writes responses back. Notifications from the server are forwarded to
// the client via a separate goroutine.
func (h *MCPConnHandler) Handle(ctx context.Context, conn net.Conn, sessionID string) error {
	defer conn.Close()

	session := newMCPSocketSession(sessionID)

	if err := h.Server.RegisterSession(ctx, session); err != nil {
		return fmt.Errorf("register session: %w", err)
	}
	defer h.Server.UnregisterSession(ctx, sessionID)

	sCtx := h.Server.WithContext(ctx, session)

	// Writer mutex protects concurrent writes (responses + notifications).
	var writeMu sync.Mutex

	// Forward server-initiated notifications to this connection.
	go func() {
		for {
			select {
			case notif, ok := <-session.notifications:
				if !ok {
					return
				}
				_ = transport.WriteJSONLine(conn, &writeMu, notif)
			case <-ctx.Done():
				return
			}
		}
	}()

	reader := bufio.NewReader(conn)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if len(line) == 0 {
			continue
		}

		var rawMessage json.RawMessage
		if err := json.Unmarshal([]byte(line), &rawMessage); err != nil {
			errResp := struct {
				JSONRPC string `json:"jsonrpc"`
				ID      any    `json:"id"`
				Error   struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}{
				JSONRPC: "2.0",
				ID:      nil,
				Error: struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{Code: -32700, Message: "Parse error"},
			}
			_ = transport.WriteJSONLine(conn, &writeMu, errResp)
			continue
		}

		response := h.Server.HandleMessage(sCtx, rawMessage)
		if response != nil {
			if err := transport.WriteJSONLine(conn, &writeMu, response); err != nil {
				return fmt.Errorf("write response: %w", err)
			}
		}
	}
}

// mcpSocketSession implements the mcp-go ClientSession interface for a single
// socket connection. Each connection gets its own session so the MCPServer
// can track clients independently.
type mcpSocketSession struct {
	id            string
	notifications chan mcp.JSONRPCNotification
	initialized   atomic.Bool
	loggingLevel  atomic.Value
	clientInfo    atomic.Value
	clientCaps    atomic.Value
}

func newMCPSocketSession(id string) *mcpSocketSession {
	ss := &mcpSocketSession{
		id:            id,
		notifications: make(chan mcp.JSONRPCNotification, 100),
	}
	ss.loggingLevel.Store(mcp.LoggingLevelError)
	return ss
}

func (s *mcpSocketSession) SessionID() string { return s.id }
func (s *mcpSocketSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return s.notifications
}
func (s *mcpSocketSession) Initialize()       { s.initialized.Store(true) }
func (s *mcpSocketSession) Initialized() bool { return s.initialized.Load() }

func (s *mcpSocketSession) SetLogLevel(level mcp.LoggingLevel) { s.loggingLevel.Store(level) }
func (s *mcpSocketSession) GetLogLevel() mcp.LoggingLevel {
	if v := s.loggingLevel.Load(); v != nil {
		if l, ok := v.(mcp.LoggingLevel); ok {
			return l
		}
	}
	return mcp.LoggingLevelError
}

func (s *mcpSocketSession) GetClientInfo() mcp.Implementation {
	if v := s.clientInfo.Load(); v != nil {
		if ci, ok := v.(mcp.Implementation); ok {
			return ci
		}
	}
	return mcp.Implementation{}
}
func (s *mcpSocketSession) SetClientInfo(ci mcp.Implementation) { s.clientInfo.Store(ci) }
func (s *mcpSocketSession) GetClientCapabilities() mcp.ClientCapabilities {
	if v := s.clientCaps.Load(); v != nil {
		if cc, ok := v.(mcp.ClientCapabilities); ok {
			return cc
		}
	}
	return mcp.ClientCapabilities{}
}
func (s *mcpSocketSession) SetClientCapabilities(cc mcp.ClientCapabilities) {
	s.clientCaps.Store(cc)
}

// Compile-time interface checks.
var (
	_ server.ClientSession         = (*mcpSocketSession)(nil)
	_ server.SessionWithLogging    = (*mcpSocketSession)(nil)
	_ server.SessionWithClientInfo = (*mcpSocketSession)(nil)
)
