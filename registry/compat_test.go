//go:build !official_sdk

package registry

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestExtractResourceText_Nil(t *testing.T) {
	_, ok := ExtractResourceText(nil)
	if ok {
		t.Error("ExtractResourceText(nil) should return false")
	}
}

func TestExtractResourceText_Empty(t *testing.T) {
	result := &ReadResourceResult{Contents: nil}
	_, ok := ExtractResourceText(result)
	if ok {
		t.Error("ExtractResourceText with empty Contents should return false")
	}
}

func TestExtractResourceText_TextContent(t *testing.T) {
	result := &ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{URI: "file://test.txt", Text: "hello world"},
		},
	}
	text, ok := ExtractResourceText(result)
	if !ok {
		t.Fatal("expected ok=true for TextResourceContents")
	}
	if text != "hello world" {
		t.Errorf("text = %q, want hello world", text)
	}
}

func TestExtractResourceText_BlobContent(t *testing.T) {
	result := &ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.BlobResourceContents{URI: "file://test.bin", Blob: "AAAA"},
		},
	}
	_, ok := ExtractResourceText(result)
	if ok {
		t.Error("ExtractResourceText with blob content should return false")
	}
}

func TestExtractArguments_Nil(t *testing.T) {
	req := CallToolRequest{}
	args := ExtractArguments(req)
	if args != nil {
		t.Errorf("ExtractArguments with nil params should return nil, got %v", args)
	}
}

func TestExtractArguments_Map(t *testing.T) {
	req := CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"query": "test",
		"limit": 10,
	}
	args := ExtractArguments(req)
	if args == nil {
		t.Fatal("ExtractArguments should return map, got nil")
	}
	if args["query"] != "test" {
		t.Errorf("query = %v, want test", args["query"])
	}
}

func TestExtractArguments_WrongType(t *testing.T) {
	req := CallToolRequest{}
	req.Params.Arguments = "not a map"
	args := ExtractArguments(req)
	if args != nil {
		t.Errorf("ExtractArguments with wrong type should return nil, got %v", args)
	}
}

func TestGetToolTaskSupport_NilExecution(t *testing.T) {
	tool := Tool{Name: "test_tool"}
	support := GetToolTaskSupport(tool)
	if support != TaskSupportForbidden {
		t.Errorf("nil Execution should return TaskSupportForbidden, got %v", support)
	}
}

func TestGetToolTaskSupport_WithExecution(t *testing.T) {
	tool := Tool{
		Name: "test_tool",
		Execution: &ToolExecution{
			TaskSupport: TaskSupportOptional,
		},
	}
	support := GetToolTaskSupport(tool)
	if support != TaskSupportOptional {
		t.Errorf("TaskSupport = %v, want TaskSupportOptional", support)
	}
}

func TestHasTaskParams_NoTask(t *testing.T) {
	req := CallToolRequest{}
	if HasTaskParams(req) {
		t.Error("HasTaskParams should return false when Task is nil")
	}
}

func TestHasTaskParams_WithTask(t *testing.T) {
	req := CallToolRequest{}
	req.Params.Task = &mcp.TaskParams{}
	if !HasTaskParams(req) {
		t.Error("HasTaskParams should return true when Task is set")
	}
}

func TestExtractTaskTTL_NoTask(t *testing.T) {
	req := CallToolRequest{}
	ttl := ExtractTaskTTL(req)
	if ttl != 0 {
		t.Errorf("ExtractTaskTTL with no task should return 0, got %d", ttl)
	}
}

func TestExtractTaskTTL_NoTTL(t *testing.T) {
	req := CallToolRequest{}
	req.Params.Task = &mcp.TaskParams{}
	ttl := ExtractTaskTTL(req)
	if ttl != 0 {
		t.Errorf("ExtractTaskTTL with nil TTL should return 0, got %d", ttl)
	}
}

func TestExtractTaskTTL_WithTTL(t *testing.T) {
	req := CallToolRequest{}
	ttlVal := int64(300)
	req.Params.Task = &mcp.TaskParams{TTL: &ttlVal}
	ttl := ExtractTaskTTL(req)
	if ttl != 300 {
		t.Errorf("ExtractTaskTTL = %d, want 300", ttl)
	}
}

func TestMakeStructuredResult(t *testing.T) {
	content := MakeTextContent("summary text")
	data := map[string]any{"count": 42}
	result := MakeStructuredResult(content, data)

	if result == nil {
		t.Fatal("MakeStructuredResult returned nil")
	}
	if len(result.Content) != 1 {
		t.Errorf("Content length = %d, want 1", len(result.Content))
	}
	if result.IsError {
		t.Error("structured result should not be an error")
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want map[string]any", result.StructuredContent)
	}
	if structured["count"] != 42 {
		t.Errorf("count = %v, want 42", structured["count"])
	}
}
