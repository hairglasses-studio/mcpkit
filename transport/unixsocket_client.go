package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// UnixSocketClient connects to an existing MCP server via Unix domain socket.
// It provides a simple request/response Call method for JSON-RPC communication.
type UnixSocketClient struct {
	socketPath string
	conn       net.Conn
	reader     *bufio.Reader

	writeMu   sync.Mutex
	requestID atomic.Int64

	// pending tracks in-flight requests by ID.
	pendingMu sync.Mutex
	pending   map[int64]chan json.RawMessage

	done chan struct{}
}

// DialUnixSocket connects to an MCP server at the given Unix socket path.
func DialUnixSocket(socketPath string) (*UnixSocketClient, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("unixsocket: dial: %w", err)
	}

	c := &UnixSocketClient{
		socketPath: socketPath,
		conn:       conn,
		reader:     bufio.NewReader(conn),
		pending:    make(map[int64]chan json.RawMessage),
		done:       make(chan struct{}),
	}

	go c.readLoop()
	return c, nil
}

// readLoop reads responses from the socket and routes them to pending callers.
func (c *UnixSocketClient) readLoop() {
	defer close(c.done)
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			// Connection closed — wake all pending callers.
			c.pendingMu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.pendingMu.Unlock()
			return
		}

		// Parse the response to extract the ID.
		var envelope struct {
			ID json.Number `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			continue // notification or malformed — skip
		}

		id, err := envelope.ID.Int64()
		if err != nil {
			continue // notification (no numeric id)
		}

		c.pendingMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()

		if ok {
			ch <- json.RawMessage(line)
		}
	}
}

// jsonRPCRequest is the wire format for outgoing requests.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Call sends a JSON-RPC request and waits for the response.
func (c *UnixSocketClient) Call(method string, params any) (json.RawMessage, error) {
	id := c.requestID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("unixsocket: marshal request: %w", err)
	}
	data = append(data, '\n')

	// Register the pending response channel before sending.
	ch := make(chan json.RawMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	_, writeErr := c.conn.Write(data)
	c.writeMu.Unlock()
	if writeErr != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("unixsocket: write: %w", writeErr)
	}

	// Wait for the response.
	resp, ok := <-ch
	if !ok {
		return nil, fmt.Errorf("unixsocket: connection closed")
	}
	return resp, nil
}

// Close closes the connection to the server.
func (c *UnixSocketClient) Close() error {
	err := c.conn.Close()
	<-c.done // wait for readLoop to finish
	return err
}
