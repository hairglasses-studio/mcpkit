package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// --------------------------------------------------------------------------
// socketSession tests (unit tests for the session type)
// --------------------------------------------------------------------------

func TestSocketSession_NewSession(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("test-1")

	if ss.SessionID() != "test-1" {
		t.Errorf("expected session ID %q, got %q", "test-1", ss.SessionID())
	}
	if ss.Initialized() {
		t.Error("new session should not be initialized")
	}
}

func TestSocketSession_Initialize(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("init-test")

	ss.Initialize()
	if !ss.Initialized() {
		t.Error("expected session to be initialized after Initialize()")
	}

	// Idempotent.
	ss.Initialize()
	if !ss.Initialized() {
		t.Error("expected session to remain initialized")
	}
}

func TestSocketSession_NotificationChannel(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("notif-test")

	ch := ss.NotificationChannel()
	if ch == nil {
		t.Fatal("expected non-nil notification channel")
	}

	// Should be able to send without blocking (buffered channel).
	ch <- mcp.JSONRPCNotification{}
}

func TestSocketSession_LogLevel(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("log-test")

	// Default is error level.
	if ss.GetLogLevel() != mcp.LoggingLevelError {
		t.Errorf("expected default log level %v, got %v", mcp.LoggingLevelError, ss.GetLogLevel())
	}

	ss.SetLogLevel(mcp.LoggingLevelDebug)
	if ss.GetLogLevel() != mcp.LoggingLevelDebug {
		t.Errorf("expected log level %v, got %v", mcp.LoggingLevelDebug, ss.GetLogLevel())
	}
}

func TestSocketSession_ClientInfo(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("info-test")

	// Default should be zero value.
	info := ss.GetClientInfo()
	if info.Name != "" {
		t.Errorf("expected empty client name, got %q", info.Name)
	}

	ci := mcp.Implementation{Name: "test-client", Version: "1.0.0"}
	ss.SetClientInfo(ci)
	got := ss.GetClientInfo()
	if got.Name != "test-client" || got.Version != "1.0.0" {
		t.Errorf("expected %v, got %v", ci, got)
	}
}

func TestSocketSession_ClientCapabilities(t *testing.T) {
	t.Parallel()
	ss := newSocketSession("caps-test")

	// Default should be zero value.
	caps := ss.GetClientCapabilities()
	if caps.Sampling != nil {
		t.Errorf("expected nil sampling capability, got %v", caps.Sampling)
	}

	cc := mcp.ClientCapabilities{
		Sampling: &struct{}{},
	}
	ss.SetClientCapabilities(cc)
	got := ss.GetClientCapabilities()
	if got.Sampling == nil {
		t.Error("expected non-nil sampling capability after set")
	}
}

// --------------------------------------------------------------------------
// createJSONRPCError tests
// --------------------------------------------------------------------------

func TestCreateJSONRPCError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      any
		code    int
		message string
	}{
		{"nil ID", nil, -32700, "Parse error"},
		{"numeric ID", 42, -32600, "Invalid request"},
		{"string ID", "req-1", -32601, "Method not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := createJSONRPCError(tt.id, tt.code, tt.message)

			if err.JSONRPC != "2.0" {
				t.Errorf("expected jsonrpc 2.0, got %q", err.JSONRPC)
			}
			if err.ID != tt.id {
				t.Errorf("expected ID %v, got %v", tt.id, err.ID)
			}
			if err.Error.Code != tt.code {
				t.Errorf("expected code %d, got %d", tt.code, err.Error.Code)
			}
			if err.Error.Message != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, err.Error.Message)
			}

			// Should be JSON-serializable.
			data, jsonErr := json.Marshal(err)
			if jsonErr != nil {
				t.Fatalf("Marshal: %v", jsonErr)
			}
			if !strings.Contains(string(data), `"jsonrpc":"2.0"`) {
				t.Errorf("expected JSON to contain jsonrpc field, got %s", data)
			}
		})
	}
}

// --------------------------------------------------------------------------
// WriteJSONLine tests
// --------------------------------------------------------------------------

func TestWriteJSONLine_WithMutex(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	var mu sync.Mutex

	type msg struct {
		Method string `json:"method"`
	}

	err := WriteJSONLine(&buf, &mu, msg{Method: "test"})
	if err != nil {
		t.Fatalf("WriteJSONLine: %v", err)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Error("expected newline terminator")
	}
	if !strings.Contains(line, `"method":"test"`) {
		t.Errorf("expected method in output, got %q", line)
	}
}

func TestWriteJSONLine_WithoutMutex(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := WriteJSONLine(&buf, nil, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("WriteJSONLine: %v", err)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Error("expected newline terminator")
	}
	if !strings.Contains(line, `"key":"value"`) {
		t.Errorf("expected key in output, got %q", line)
	}
}

func TestWriteJSONLine_UnmarshalableValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// Channels cannot be marshaled to JSON.
	err := WriteJSONLine(&buf, nil, make(chan int))
	if err == nil {
		t.Error("expected error for unmarshalable value")
	}
}

// --------------------------------------------------------------------------
// DefaultSocketPath tests
// --------------------------------------------------------------------------

func TestDefaultSocketPath_WithXDGRuntimeDir(t *testing.T) {
	// Save and restore original value.
	orig := os.Getenv("XDG_RUNTIME_DIR")
	t.Cleanup(func() { os.Setenv("XDG_RUNTIME_DIR", orig) })

	os.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	path := DefaultSocketPath("test-server")
	expected := "/run/user/1000/mcpkit/test-server.sock"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestDefaultSocketPath_WithoutXDGRuntimeDir(t *testing.T) {
	orig := os.Getenv("XDG_RUNTIME_DIR")
	t.Cleanup(func() { os.Setenv("XDG_RUNTIME_DIR", orig) })

	os.Unsetenv("XDG_RUNTIME_DIR")
	path := DefaultSocketPath("test-server")
	expected := "/tmp/mcpkit/test-server.sock"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestDefaultSocketPath_EmptyName(t *testing.T) {
	orig := os.Getenv("XDG_RUNTIME_DIR")
	t.Cleanup(func() { os.Setenv("XDG_RUNTIME_DIR", orig) })

	os.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	path := DefaultSocketPath("")
	expected := "/run/user/1000/mcpkit/.sock"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

// --------------------------------------------------------------------------
// NewUnixSocketServer + WithLogger
// --------------------------------------------------------------------------

func TestNewUnixSocketServer(t *testing.T) {
	t.Parallel()
	s := server.NewMCPServer("test", "1.0.0")
	us := NewUnixSocketServer(s, "/tmp/test-mcpkit.sock")
	if us == nil {
		t.Fatal("expected non-nil UnixSocketServer")
	}
	if us.socketPath != "/tmp/test-mcpkit.sock" {
		t.Errorf("unexpected socket path: %s", us.socketPath)
	}
}

func TestNewUnixSocketServer_WithLogger(t *testing.T) {
	t.Parallel()
	s := server.NewMCPServer("test", "1.0.0")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	us := NewUnixSocketServer(s, "/tmp/test-logger.sock", WithLogger(logger))
	if us.logger != logger {
		t.Error("expected custom logger to be set")
	}
}

// --------------------------------------------------------------------------
// Unix socket server integration tests (Serve + handleConn + Shutdown)
// --------------------------------------------------------------------------

// testSocketPath returns a unique socket path inside t.TempDir().
func testSocketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.sock")
}

// waitForSocket polls until the socket file exists or the deadline is reached.
func waitForSocket(t *testing.T, sockPath string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket %s did not appear in time", sockPath)
}

func TestUnixSocketServer_ServeAndShutdown(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-srv", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath, WithLogger(slog.Default()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- us.Serve(ctx)
	}()

	waitForSocket(t, sockPath)

	// Connect and send a valid JSON-RPC initialize request.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	if _, err := conn.Write([]byte(initReq)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the response.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	// Should be valid JSON-RPC response.
	var resp map[string]any
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("unmarshal response: %v (raw: %s)", err, line)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}

	// Shutdown the server.
	if err := us.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Cancel context so Serve returns.
	cancel()

	select {
	case err := <-serveErr:
		// Serve should return with context.Canceled or a listener close error.
		if err != nil && !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "use of closed") {
			t.Logf("Serve returned: %v (acceptable)", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after shutdown")
	}

	// Socket file should be removed.
	if _, err := os.Stat(sockPath); err == nil {
		t.Error("expected socket file to be removed after Shutdown")
	}
}

func TestUnixSocketServer_HandleConn_ParseError(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-parse", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON to trigger parse error path.
	if _, err := conn.Write([]byte("not valid json\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Should receive a JSON-RPC error response.
	var errResp jsonRPCError
	if err := json.Unmarshal([]byte(line), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if errResp.Error.Message != "Parse error" {
		t.Errorf("expected 'Parse error', got %q", errResp.Error.Message)
	}

	us.Shutdown()
	cancel()
}

func TestUnixSocketServer_HandleConn_MultipleClients(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-multi", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	// Connect two clients concurrently.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			conn, err := net.Dial("unix", sockPath)
			if err != nil {
				t.Errorf("client %d dial: %v", clientID, err)
				return
			}
			defer conn.Close()

			initReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"client-%d","version":"1.0"}}}`+"\n", clientID, clientID)
			if _, err := conn.Write([]byte(initReq)); err != nil {
				t.Errorf("client %d write: %v", clientID, err)
				return
			}

			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			reader := bufio.NewReader(conn)
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Errorf("client %d read: %v", clientID, err)
				return
			}

			var resp map[string]any
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				t.Errorf("client %d unmarshal: %v", clientID, err)
				return
			}
			if resp["jsonrpc"] != "2.0" {
				t.Errorf("client %d: expected jsonrpc 2.0, got %v", clientID, resp["jsonrpc"])
			}
		}(i + 1)
	}
	wg.Wait()

	us.Shutdown()
	cancel()
}

func TestUnixSocketServer_Shutdown_Idempotent(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-idempotent", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	// Double shutdown should not panic.
	if err := us.Shutdown(); err != nil {
		t.Fatalf("first Shutdown: %v", err)
	}
	if err := us.Shutdown(); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}

	cancel()
}

func TestUnixSocketServer_Serve_InvalidPath(t *testing.T) {
	t.Parallel()

	// Use a path that cannot be created.
	mcpSrv := server.NewMCPServer("test-bad", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, "/dev/null/impossible/test.sock")

	ctx := context.Background()
	err := us.Serve(ctx)
	if err == nil {
		t.Fatal("expected error for invalid socket path")
	}
}

// --------------------------------------------------------------------------
// UnixSocketClient integration tests (DialUnixSocket, Call, Close)
// --------------------------------------------------------------------------

func TestUnixSocketClient_DialCallClose(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-client", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	// Dial the socket using the client.
	client, err := DialUnixSocket(sockPath)
	if err != nil {
		t.Fatalf("DialUnixSocket: %v", err)
	}

	// Call initialize.
	resp, err := client.Call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "go-test", "version": "1.0"},
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	// Response should be valid JSON-RPC.
	var parsed map[string]any
	if err := json.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", parsed["jsonrpc"])
	}

	// Close the client.
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	us.Shutdown()
	cancel()
}

func TestUnixSocketClient_DialFailure(t *testing.T) {
	t.Parallel()

	_, err := DialUnixSocket("/tmp/nonexistent-mcpkit-test.sock")
	if err == nil {
		t.Fatal("expected error dialing nonexistent socket")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Errorf("expected dial error, got: %v", err)
	}
}

func TestUnixSocketClient_ConnectionClosedDuringCall(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-close-during", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	client, err := DialUnixSocket(sockPath)
	if err != nil {
		t.Fatalf("DialUnixSocket: %v", err)
	}

	// Shutdown the server, then try to call -- should get connection closed error.
	us.Shutdown()
	cancel()

	// Give the server a moment to close connections.
	time.Sleep(50 * time.Millisecond)

	_, err = client.Call("initialize", nil)
	if err == nil {
		t.Fatal("expected error after server shutdown")
	}

	// Client close should still work.
	_ = client.Close()
}

func TestUnixSocketServer_HandleConn_InitThenToolsList(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-tools", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Initialize.
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	conn.Write([]byte(initReq))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader.ReadString('\n')

	// Send initialized notification.
	initNotif := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	conn.Write([]byte(initNotif))

	// Now call tools/list — response handler should write back.
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	conn.Write([]byte(toolsReq))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read tools response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("unmarshal: %v (raw: %s)", err, line)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}

	us.Shutdown()
	cancel()
}

func TestUnixSocketServer_HandleConn_EmptyLine(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-empty-line", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send an empty newline — should be silently skipped.
	conn.Write([]byte("\n"))

	// Then send a valid request.
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	conn.Write([]byte(initReq))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}

	us.Shutdown()
	cancel()
}

func TestUnixSocketServer_HandleConn_ClientDisconnect(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-disconnect", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send initialize.
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	conn.Write([]byte(initReq))

	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader.ReadString('\n')

	// Close the client connection — handleConn should handle the EOF gracefully.
	conn.Close()

	// Give the server time to process the disconnect.
	time.Sleep(50 * time.Millisecond)

	us.Shutdown()
	cancel()
}

func TestUnixSocketServer_HandleConn_ContextCancelled(t *testing.T) {
	t.Parallel()

	sockPath := testSocketPath(t)
	mcpSrv := server.NewMCPServer("test-ctx-cancel", "1.0.0")
	us := NewUnixSocketServer(mcpSrv, sockPath)

	ctx, cancel := context.WithCancel(context.Background())

	go us.Serve(ctx)
	waitForSocket(t, sockPath)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Initialize.
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	conn.Write([]byte(initReq))

	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader.ReadString('\n')

	// Cancel context — handleConn should exit via ctx.Err() check.
	cancel()

	// Give time for context propagation.
	time.Sleep(100 * time.Millisecond)
}

func TestUnixSocketClient_ReadLoopNotificationSkip(t *testing.T) {
	t.Parallel()

	// Create a raw unix socket pair to test the client readLoop's
	// handling of notification messages (no numeric ID).
	sockPath := testSocketPath(t)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	client, err := DialUnixSocket(sockPath)
	if err != nil {
		t.Fatalf("DialUnixSocket: %v", err)
	}
	defer client.Close()

	serverConn := <-accepted
	defer serverConn.Close()

	// Send a notification (no numeric id) -- client readLoop should skip it.
	notif := `{"jsonrpc":"2.0","method":"notifications/message","params":{}}` + "\n"
	if _, err := serverConn.Write([]byte(notif)); err != nil {
		t.Fatalf("write notification: %v", err)
	}

	// Now send a proper response to a Call.
	go func() {
		reader := bufio.NewReader(serverConn)
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		var req struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal([]byte(line), &req)
		resp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"ok":true}}`, req.ID) + "\n"
		serverConn.Write([]byte(resp))
	}()

	resp, err := client.Call("ping", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !strings.Contains(string(resp), `"ok":true`) {
		t.Errorf("unexpected response: %s", resp)
	}
}
