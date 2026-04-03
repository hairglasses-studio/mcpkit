//go:build !official_sdk

package registry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditMiddleware_WritesJSONL(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "audit.jsonl")

	td := ToolDefinition{
		Tool:       Tool{Name: "test_tool"},
		IsWrite:    true,
		Complexity: ComplexityModerate,
	}

	next := func(ctx context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("ok"), nil
	}

	mw := AuditMiddleware(logPath)
	handler := mw("test_tool", td, next)
	_, err := handler(context.Background(), CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read and parse the log entry.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	var entry AuditEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal: %v (data: %s)", err, data)
	}

	if entry.Tool != "test_tool" {
		t.Errorf("tool = %q, want %q", entry.Tool, "test_tool")
	}
	if entry.Tier != TierMutating {
		t.Errorf("tier = %q, want %q", entry.Tier, TierMutating)
	}
	if entry.Error {
		t.Error("error should be false for successful call")
	}
	if entry.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
	if entry.Duration == "" {
		t.Error("duration should not be empty")
	}
}

func TestAuditMiddleware_RecordsErrors(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "audit.jsonl")

	td := ToolDefinition{
		Tool: Tool{Name: "fail_tool"},
	}

	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return nil, fmt.Errorf("boom")
	}

	mw := AuditMiddleware(logPath)
	handler := mw("fail_tool", td, next)
	_, _ = handler(context.Background(), CallToolRequest{})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	var entry AuditEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !entry.Error {
		t.Error("error should be true for failed call")
	}
}

func TestAuditMiddleware_RecordsResultErrors(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "audit.jsonl")

	td := ToolDefinition{
		Tool: Tool{Name: "err_result_tool"},
	}

	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeErrorResult("something went wrong"), nil
	}

	mw := AuditMiddleware(logPath)
	handler := mw("err_result_tool", td, next)
	_, _ = handler(context.Background(), CallToolRequest{})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	var entry AuditEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !entry.Error {
		t.Error("error should be true for error result")
	}
}

func TestAuditMiddleware_DefaultPath(t *testing.T) {
	t.Parallel()

	// Just verify the default path function returns a reasonable value.
	path := defaultAuditLogPath()
	if !strings.HasSuffix(path, filepath.Join("mcpkit", "audit.jsonl")) {
		t.Errorf("unexpected default path: %s", path)
	}
}

func TestAuditMiddleware_Rotation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	// Create a file that exceeds the rotation threshold.
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Write just over 10MB of data.
	line := strings.Repeat("x", 1024) + "\n"
	for written := 0; written < maxAuditFileSize+1; written += len(line) {
		f.WriteString(line)
	}
	f.Close()

	td := ToolDefinition{Tool: Tool{Name: "rotate_tool"}}
	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("ok"), nil
	}

	mw := AuditMiddleware(logPath)
	handler := mw("rotate_tool", td, next)
	_, _ = handler(context.Background(), CallToolRequest{})

	// The old file should have been rotated to .1.
	rotatedPath := logPath + ".1"
	if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
		t.Error("rotated file should exist at .1")
	}

	// The new file should contain exactly one entry.
	newData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read new log: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(newData)))
	count := 0
	for scanner.Scan() {
		if scanner.Text() != "" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("new log should have 1 entry, got %d", count)
	}
}

func TestExtractParamKeys(t *testing.T) {
	t.Parallel()

	// With nil arguments, should return empty slice.
	req := CallToolRequest{}
	keys := extractParamKeys(req)
	if len(keys) != 0 {
		t.Errorf("expected empty keys, got %v", keys)
	}
}
