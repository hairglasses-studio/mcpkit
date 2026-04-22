//go:build !official_sdk

// Command smoke-matrix verifies that each public example binary works correctly on
// both stdio and streamable-HTTP transports.
//
// For each example it:
//   - Spawns the example binary (or skips with a reason for incompatible examples)
//   - Issues an initialize + tools/list handshake via each transport
//   - Verifies the response contains a non-empty tools array where each tool
//     has both name and description fields
//   - Records pass/fail per (example, transport) pair
//   - Outputs a JSON results file and a human-readable table to stdout
//
// Usage:
//
//	go run ./tools/smoke-matrix
//	go run ./tools/smoke-matrix -examples ./examples -json results.json
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Transport names.
const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

// Result status values.
const (
	StatusPass = "pass"
	StatusFail = "fail"
	StatusSkip = "skip"
)

// ExampleConfig describes how to smoke-test a single example.
type ExampleConfig struct {
	// Name is the directory name under examples/.
	Name string

	// StdioSkipReason is non-empty when the stdio transport should be skipped.
	StdioSkipReason string

	// HTTPSkipReason is non-empty when the HTTP transport should be skipped.
	HTTPSkipReason string

	// HTTPPort is the port the example listens on when using HTTP.
	// Zero means no HTTP support.
	HTTPPort int

	// HTTPEnv holds extra environment variables needed to start the HTTP server.
	HTTPEnv []string
}

// ToolInfo holds parsed tool metadata from a tools/list response.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// TransportResult records the outcome of one (example, transport) probe.
type TransportResult struct {
	Transport  string     `json:"transport"`
	Status     string     `json:"status"` // pass | fail | skip
	Reason     string     `json:"reason,omitempty"`
	Tools      []ToolInfo `json:"tools,omitempty"`
	Error      string     `json:"error,omitempty"`
	DurationMs int64      `json:"duration_ms,omitempty"`
}

// ExampleResult collects results for one example across all transports.
type ExampleResult struct {
	Example string            `json:"example"`
	Results []TransportResult `json:"results"`
}

// SmokeReport is the top-level JSON output.
type SmokeReport struct {
	Timestamp string          `json:"timestamp"`
	Pass      int             `json:"pass"`
	Fail      int             `json:"fail"`
	Skip      int             `json:"skip"`
	Examples  []ExampleResult `json:"examples"`
}

// ---------------------------------------------------------------------------
// Example catalog
// ---------------------------------------------------------------------------

// examples is the ordered list of all public example directories and their
// transport configuration. Edit this list when adding new examples.
var examples = []ExampleConfig{
	{
		Name:           "minimal",
		HTTPSkipReason: "stdio-only example (no HTTP transport)",
	},
	{
		Name:           "bounded-write",
		HTTPSkipReason: "stdio-only example (no HTTP transport)",
	},
	{
		Name:           "elicitation",
		HTTPSkipReason: "stdio-only example (no HTTP transport)",
	},
	{
		Name:           "full",
		HTTPSkipReason: "stdio-only example (no HTTP transport)",
	},
	{
		Name:           "pagination",
		HTTPSkipReason: "stdio-only example (no HTTP transport)",
	},
	{
		Name:           "truncate-demo",
		HTTPSkipReason: "stdio-only example (no HTTP transport)",
	},
	{
		Name:           "vuln-scanner",
		HTTPSkipReason: "stdio-only example (no HTTP transport)",
	},
	{
		Name: "http",
		// HTTP example serves on port 8080; we override with a random free port.
		StdioSkipReason: "HTTP-transport example (no stdio mode)",
		HTTPPort:        0, // resolved at runtime
	},
	{
		Name:            "stateless-http",
		StdioSkipReason: "HTTP-transport example (no stdio mode)",
		HTTPPort:        0, // resolved at runtime
	},
	{
		Name:            "gateway",
		StdioSkipReason: "gateway registers 0 tools without upstream env vars (MCP_UPSTREAM_*); skipping to avoid false negatives",
		HTTPSkipReason:  "gateway has no HTTP transport",
	},
	{
		Name:            "rdcycle",
		StdioSkipReason: "rdcycle runs a workflow loop, not a stdio MCP server",
		HTTPSkipReason:  "rdcycle has no HTTP transport",
	},
	{
		Name:            "a2a-bridge",
		StdioSkipReason: "a2a-bridge serves the A2A protocol, not MCP over stdio",
		HTTPSkipReason:  "a2a-bridge serves the A2A protocol (not MCP streamable-HTTP); tools/list is not available",
	},
}

// ---------------------------------------------------------------------------
// JSON-RPC helpers
// ---------------------------------------------------------------------------

// rpcRequest builds a minimal JSON-RPC 2.0 request.
func rpcRequest(id int, method string, params any) ([]byte, error) {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	return json.Marshal(req)
}

// extractTools parses the tools array from a tools/list JSON-RPC response body.
func extractTools(body []byte) ([]ToolInfo, error) {
	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	tools := make([]ToolInfo, 0, len(rpcResp.Result.Tools))
	for _, t := range rpcResp.Result.Tools {
		tools = append(tools, ToolInfo{Name: t.Name, Description: t.Description})
	}
	return tools, nil
}

// validateTools checks that each tool has a non-empty name and description.
func validateTools(tools []ToolInfo) error {
	if len(tools) == 0 {
		return fmt.Errorf("tools array is empty")
	}
	for i, t := range tools {
		if t.Name == "" {
			return fmt.Errorf("tool[%d] missing name", i)
		}
		if t.Description == "" {
			return fmt.Errorf("tool[%d] %q missing description", i, t.Name)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Stdio transport probe
// ---------------------------------------------------------------------------

// probeStdio spawns the binary, sends initialize + tools/list over stdin,
// reads the responses from stdout, and returns the discovered tools.
func probeStdio(ctx context.Context, binary string) ([]ToolInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard // suppress example startup logs

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// MCP stdio framing: each message is a single newline-terminated JSON line.
	send := func(msg []byte) error {
		_, err := fmt.Fprintf(stdin, "%s\n", msg)
		return err
	}

	// Read one complete JSON object from stdout.
	readOne := func() ([]byte, error) {
		var buf bytes.Buffer
		tmp := make([]byte, 1)
		depth := 0
		started := false
		for {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			n, err := stdout.Read(tmp)
			if err != nil {
				return nil, fmt.Errorf("read stdout: %w", err)
			}
			if n == 0 {
				continue
			}
			b := tmp[0]
			if !started && b == '{' {
				started = true
			}
			if !started {
				continue
			}
			buf.WriteByte(b)
			switch b {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return buf.Bytes(), nil
				}
			}
		}
	}

	// Step 1: initialize
	initReq, _ := rpcRequest(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "smoke-matrix", "version": "1.0.0"},
	})
	if err := send(initReq); err != nil {
		return nil, fmt.Errorf("send initialize: %w", err)
	}
	_, err = readOne() // consume initialize response (don't need it)
	if err != nil {
		return nil, fmt.Errorf("read initialize response: %w", err)
	}

	// Step 2: initialized notification (required by spec before tool calls)
	initNotify := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if err := send(initNotify); err != nil {
		return nil, fmt.Errorf("send initialized notification: %w", err)
	}

	// Step 3: tools/list
	listReq, _ := rpcRequest(2, "tools/list", nil)
	if err := send(listReq); err != nil {
		return nil, fmt.Errorf("send tools/list: %w", err)
	}
	listResp, err := readOne()
	if err != nil {
		return nil, fmt.Errorf("read tools/list response: %w", err)
	}

	return extractTools(listResp)
}

// ---------------------------------------------------------------------------
// HTTP transport probe
// ---------------------------------------------------------------------------

// freePort finds a random free TCP port.
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

// probeHTTP starts the binary on a free port, waits for it to be ready,
// then sends initialize + tools/list over StreamableHTTP.
func probeHTTP(ctx context.Context, binary string, extraEnv []string) ([]ToolInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("find free port: %w", err)
	}

	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"SERVER_ID=smoke-test",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	endpoint := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 10 * time.Second}

	// Wait for the server to be ready (poll for up to 10 seconds).
	ready := waitForHTTP(ctx, client, endpoint)
	if !ready {
		return nil, fmt.Errorf("server did not become ready on port %d within timeout", port)
	}

	// StreamableHTTP: use a session ID header, send initialize first.
	sessionID := fmt.Sprintf("smoke-session-%d", time.Now().UnixNano())

	doPost := func(body []byte) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Session-Id", sessionID)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("POST: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		// The server may return SSE or raw JSON.
		// Strip SSE framing if present.
		if resp.Header.Get("Content-Type") == "text/event-stream" {
			data = stripSSEFrame(data)
		}

		return data, nil
	}

	// Step 1: initialize
	initReq, _ := rpcRequest(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "smoke-matrix", "version": "1.0.0"},
	})
	initResp, err := doPost(initReq)
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	// Extract session ID from initialize response if provided.
	if sid := parseSessionID(initResp); sid != "" {
		sessionID = sid
	}

	// Step 2: initialized notification
	initNotify := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	_, _ = doPost(initNotify)

	// Step 3: tools/list
	listReq, _ := rpcRequest(2, "tools/list", nil)
	listResp, err := doPost(listReq)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	return extractTools(listResp)
}

// waitForHTTP polls the endpoint until it responds (or context is done).
func waitForHTTP(ctx context.Context, client *http.Client, endpoint string) bool {
	for {
		if ctx.Err() != nil {
			return false
		}
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader([]byte(`{}`)))
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// stripSSEFrame extracts the JSON data line(s) from an SSE frame.
// SSE format: "data: <json>\n\n"
func stripSSEFrame(b []byte) []byte {
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload != "" {
				return []byte(payload)
			}
		}
	}
	return b
}

// parseSessionID tries to extract an Mcp-Session-Id value from JSON response.
func parseSessionID(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	// Some implementations return session ID in the result.
	if result, ok := m["result"].(map[string]any); ok {
		if sid, ok := result["sessionId"].(string); ok {
			return sid
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Build helpers
// ---------------------------------------------------------------------------

// buildExample compiles the example binary to a temp file and returns the path.
func buildExample(ctx context.Context, examplesDir, name string) (string, error) {
	binPath := filepath.Join(os.TempDir(), fmt.Sprintf("mcpkit-smoke-%s-%d", name, time.Now().UnixNano()))

	pkg := fmt.Sprintf("./examples/%s", name)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, pkg)
	cmd.Dir = filepath.Dir(examplesDir) // mcpkit root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build ./examples/%s: %w", name, err)
	}
	return binPath, nil
}

// ---------------------------------------------------------------------------
// Probe runners
// ---------------------------------------------------------------------------

// probeTransport runs one (example, transport) probe and returns the result.
func probeTransport(ctx context.Context, binary, transport string, extraEnv []string) TransportResult {
	start := time.Now()
	var tools []ToolInfo
	var err error

	switch transport {
	case TransportStdio:
		tools, err = probeStdio(ctx, binary)
	case TransportHTTP:
		tools, err = probeHTTP(ctx, binary, extraEnv)
	default:
		return TransportResult{Transport: transport, Status: StatusSkip, Reason: "unknown transport"}
	}

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return TransportResult{
			Transport:  transport,
			Status:     StatusFail,
			Error:      err.Error(),
			DurationMs: elapsed,
		}
	}

	if verr := validateTools(tools); verr != nil {
		return TransportResult{
			Transport:  transport,
			Status:     StatusFail,
			Error:      verr.Error(),
			Tools:      tools,
			DurationMs: elapsed,
		}
	}

	return TransportResult{
		Transport:  transport,
		Status:     StatusPass,
		Tools:      tools,
		DurationMs: elapsed,
	}
}

// runExample runs both transports for one example.
func runExample(ctx context.Context, examplesDir string, cfg ExampleConfig) ExampleResult {
	result := ExampleResult{Example: cfg.Name}

	for _, transport := range []string{TransportStdio, TransportHTTP} {
		var skipReason string
		switch transport {
		case TransportStdio:
			skipReason = cfg.StdioSkipReason
		case TransportHTTP:
			skipReason = cfg.HTTPSkipReason
		}

		if skipReason != "" {
			result.Results = append(result.Results, TransportResult{
				Transport: transport,
				Status:    StatusSkip,
				Reason:    skipReason,
			})
			continue
		}

		// Build the binary.
		fmt.Fprintf(os.Stderr, "  building %s ...\n", cfg.Name)
		binary, err := buildExample(ctx, examplesDir, cfg.Name)
		if err != nil {
			result.Results = append(result.Results, TransportResult{
				Transport: transport,
				Status:    StatusFail,
				Error:     err.Error(),
			})
			continue
		}
		defer os.Remove(binary)

		fmt.Fprintf(os.Stderr, "  probing %s/%s ...\n", cfg.Name, transport)
		tr := probeTransport(ctx, binary, transport, cfg.HTTPEnv)
		result.Results = append(result.Results, tr)
	}

	return result
}

// ---------------------------------------------------------------------------
// Output formatting
// ---------------------------------------------------------------------------

// printTable writes the human-readable results table to w.
func printTable(w io.Writer, report SmokeReport) {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Transport Smoke Matrix")
	fmt.Fprintln(w, "======================")
	fmt.Fprintf(w, "%-20s  %-8s  %-8s  %s\n", "EXAMPLE", "STDIO", "HTTP", "TOOLS / REASON")
	fmt.Fprintln(w, strings.Repeat("-", 80))

	for _, ex := range report.Examples {
		stdioCell := "  -   "
		httpCell := "  -   "
		detailCell := ""

		for _, tr := range ex.Results {
			cell := statusCell(tr.Status)
			switch tr.Transport {
			case TransportStdio:
				stdioCell = cell
				switch tr.Status {
				case StatusPass:
					detailCell = fmt.Sprintf("%d tools", len(tr.Tools))
				case StatusFail:
					detailCell = "FAIL: " + tr.Error
				case StatusSkip:
					detailCell = tr.Reason
				}
			case TransportHTTP:
				httpCell = cell
				if tr.Status == StatusPass && detailCell == "" {
					detailCell = fmt.Sprintf("%d tools", len(tr.Tools))
				} else if tr.Status == StatusFail && !strings.HasPrefix(detailCell, "FAIL") {
					detailCell = "FAIL: " + tr.Error
				} else if tr.Status == StatusSkip && detailCell == "" {
					detailCell = tr.Reason
				}
			}
		}

		fmt.Fprintf(w, "%-20s  %-8s  %-8s  %s\n", ex.Example, stdioCell, httpCell, detailCell)
	}

	fmt.Fprintln(w, strings.Repeat("-", 80))
	fmt.Fprintf(w, "Total: %d pass, %d fail, %d skip\n", report.Pass, report.Fail, report.Skip)
}

func statusCell(s string) string {
	switch s {
	case StatusPass:
		return "PASS"
	case StatusFail:
		return "FAIL"
	case StatusSkip:
		return "skip"
	default:
		return "  ?  "
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	examplesFlag := flag.String("examples", "./examples", "path to examples directory")
	jsonOut := flag.String("json", "", "write JSON report to this file (default: stdout only)")
	flag.Parse()

	examplesDir, err := filepath.Abs(*examplesFlag)
	if err != nil {
		log.Fatalf("resolve examples path: %v", err)
	}

	ctx := context.Background()

	report := SmokeReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	fmt.Fprintln(os.Stderr, "Running transport smoke matrix...")

	for _, cfg := range examples {
		fmt.Fprintf(os.Stderr, "[%s]\n", cfg.Name)
		ex := runExample(ctx, examplesDir, cfg)
		report.Examples = append(report.Examples, ex)

		for _, tr := range ex.Results {
			switch tr.Status {
			case StatusPass:
				report.Pass++
			case StatusFail:
				report.Fail++
			case StatusSkip:
				report.Skip++
			}
		}
	}

	// Print human-readable table.
	printTable(os.Stdout, report)

	// Write JSON output.
	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("marshal report: %v", err)
	}

	if *jsonOut != "" {
		if err := os.WriteFile(*jsonOut, jsonBytes, 0644); err != nil {
			log.Fatalf("write JSON report: %v", err)
		}
		fmt.Fprintf(os.Stderr, "\nJSON report written to %s\n", *jsonOut)
	}

	// Exit non-zero if any test failed.
	if report.Fail > 0 {
		os.Exit(1)
	}
}
