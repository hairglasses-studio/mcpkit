//go:build !official_sdk

package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func makeReq(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func makeReqNilArgs() mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = nil
	return req
}

func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content is not TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// ==================== TextResult ====================

func TestTextResult(t *testing.T) {
	r := TextResult("hello world")
	got := extractText(t, r)
	if got != "hello world" {
		t.Errorf("TextResult text = %q, want %q", got, "hello world")
	}
	if r.IsError {
		t.Error("TextResult should not be an error")
	}
}

func TestTextResult_Empty(t *testing.T) {
	r := TextResult("")
	got := extractText(t, r)
	if got != "" {
		t.Errorf("TextResult text = %q, want empty", got)
	}
}

// ==================== ErrorResult ====================

func TestErrorResult(t *testing.T) {
	r := ErrorResult(errors.New("something broke"))
	got := extractText(t, r)
	if got != "something broke" {
		t.Errorf("ErrorResult text = %q, want %q", got, "something broke")
	}
	if !r.IsError {
		t.Error("ErrorResult should be an error")
	}
}

// ==================== JSONResult ====================

func TestJSONResult_Map(t *testing.T) {
	r := JSONResult(map[string]any{"status": "ok", "count": 42})
	got := extractText(t, r)
	if !strings.Contains(got, `"status": "ok"`) {
		t.Errorf("JSONResult missing status field, got: %s", got)
	}
	if r.IsError {
		t.Error("JSONResult should not be an error")
	}
}

func TestJSONResult_Unmarshalable(t *testing.T) {
	r := JSONResult(make(chan int))
	if !r.IsError {
		t.Error("JSONResult with unmarshalable data should be an error")
	}
}

// ==================== GetStringParam ====================

func TestGetStringParam(t *testing.T) {
	req := makeReq(map[string]any{"name": "alice"})
	if got := GetStringParam(req, "name"); got != "alice" {
		t.Errorf("GetStringParam = %q, want %q", got, "alice")
	}
}

func TestGetStringParam_Missing(t *testing.T) {
	req := makeReq(map[string]any{"other": "val"})
	if got := GetStringParam(req, "name"); got != "" {
		t.Errorf("GetStringParam missing key = %q, want empty", got)
	}
}

func TestGetStringParam_WrongType(t *testing.T) {
	req := makeReq(map[string]any{"name": 123})
	if got := GetStringParam(req, "name"); got != "" {
		t.Errorf("GetStringParam wrong type = %q, want empty", got)
	}
}

func TestGetStringParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	if got := GetStringParam(req, "name"); got != "" {
		t.Errorf("GetStringParam nil args = %q, want empty", got)
	}
}

// ==================== GetIntParam ====================

func TestGetIntParam(t *testing.T) {
	req := makeReq(map[string]any{"count": float64(7)})
	if got := GetIntParam(req, "count", 0); got != 7 {
		t.Errorf("GetIntParam = %d, want 7", got)
	}
}

func TestGetIntParam_Default(t *testing.T) {
	req := makeReq(map[string]any{})
	if got := GetIntParam(req, "count", 42); got != 42 {
		t.Errorf("GetIntParam default = %d, want 42", got)
	}
}

func TestGetIntParam_WrongType(t *testing.T) {
	req := makeReq(map[string]any{"count": "not a number"})
	if got := GetIntParam(req, "count", 99); got != 99 {
		t.Errorf("GetIntParam wrong type = %d, want 99", got)
	}
}

func TestGetIntParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	if got := GetIntParam(req, "count", 5); got != 5 {
		t.Errorf("GetIntParam nil args = %d, want 5", got)
	}
}

// ==================== GetBoolParam ====================

func TestGetBoolParam(t *testing.T) {
	req := makeReq(map[string]any{"flag": true})
	if got := GetBoolParam(req, "flag", false); !got {
		t.Error("GetBoolParam = false, want true")
	}
}

func TestGetBoolParam_Default(t *testing.T) {
	req := makeReq(map[string]any{})
	if got := GetBoolParam(req, "flag", true); !got {
		t.Error("GetBoolParam default = false, want true")
	}
}

func TestGetBoolParam_WrongType(t *testing.T) {
	req := makeReq(map[string]any{"flag": "yes"})
	if got := GetBoolParam(req, "flag", false); got {
		t.Error("GetBoolParam wrong type = true, want false")
	}
}

func TestGetBoolParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	if got := GetBoolParam(req, "flag", true); !got {
		t.Error("GetBoolParam nil args = false, want true")
	}
}

// ==================== GetFloatParam ====================

func TestGetFloatParam(t *testing.T) {
	req := makeReq(map[string]any{"price": float64(19.99)})
	if got := GetFloatParam(req, "price", 0); got != 19.99 {
		t.Errorf("GetFloatParam = %f, want 19.99", got)
	}
}

func TestGetFloatParam_Default(t *testing.T) {
	req := makeReq(map[string]any{})
	if got := GetFloatParam(req, "price", 9.99); got != 9.99 {
		t.Errorf("GetFloatParam default = %f, want 9.99", got)
	}
}

func TestGetFloatParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	if got := GetFloatParam(req, "price", 3.14); got != 3.14 {
		t.Errorf("GetFloatParam nil args = %f, want 3.14", got)
	}
}

// ==================== HasParam ====================

func TestHasParam_Present(t *testing.T) {
	req := makeReq(map[string]any{"key": "value"})
	if !HasParam(req, "key") {
		t.Error("HasParam should return true for present key")
	}
}

func TestHasParam_Absent(t *testing.T) {
	req := makeReq(map[string]any{"other": "value"})
	if HasParam(req, "key") {
		t.Error("HasParam should return false for absent key")
	}
}

func TestHasParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	if HasParam(req, "key") {
		t.Error("HasParam should return false for nil args")
	}
}

func TestHasParam_NilValue(t *testing.T) {
	req := makeReq(map[string]any{"key": nil})
	if !HasParam(req, "key") {
		t.Error("HasParam should return true when key exists with nil value")
	}
}

// ==================== GetStringArrayParam ====================

func TestGetStringArrayParam(t *testing.T) {
	req := makeReq(map[string]any{
		"tags": []any{"a", "b", "c"},
	})
	got := GetStringArrayParam(req, "tags")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("GetStringArrayParam = %v, want [a b c]", got)
	}
}

func TestGetStringArrayParam_Empty(t *testing.T) {
	req := makeReq(map[string]any{
		"tags": []any{},
	})
	got := GetStringArrayParam(req, "tags")
	if len(got) != 0 {
		t.Errorf("GetStringArrayParam empty = %v, want []", got)
	}
}

func TestGetStringArrayParam_Missing(t *testing.T) {
	req := makeReq(map[string]any{})
	got := GetStringArrayParam(req, "tags")
	if got != nil {
		t.Errorf("GetStringArrayParam missing = %v, want nil", got)
	}
}

func TestGetStringArrayParam_WrongType(t *testing.T) {
	req := makeReq(map[string]any{"tags": "not-an-array"})
	got := GetStringArrayParam(req, "tags")
	if got != nil {
		t.Errorf("GetStringArrayParam wrong type = %v, want nil", got)
	}
}

func TestGetStringArrayParam_MixedTypes(t *testing.T) {
	req := makeReq(map[string]any{
		"tags": []any{"a", 42, "b", true},
	})
	got := GetStringArrayParam(req, "tags")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("GetStringArrayParam mixed = %v, want [a b]", got)
	}
}

func TestGetStringArrayParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	got := GetStringArrayParam(req, "tags")
	if got != nil {
		t.Errorf("GetStringArrayParam nil args = %v, want nil", got)
	}
}

// ==================== CodedErrorResult ====================

func TestCodedErrorResult(t *testing.T) {
	r := CodedErrorResult(ErrNotFound, errors.New("item 123 not found"))
	got := extractText(t, r)
	if !strings.HasPrefix(got, "[NOT_FOUND]") {
		t.Errorf("CodedErrorResult prefix missing, got: %s", got)
	}
	if !r.IsError {
		t.Error("CodedErrorResult should be an error")
	}
}

func TestCodedErrorResult_AllCodes(t *testing.T) {
	codes := []string{ErrClientInit, ErrInvalidParam, ErrTimeout, ErrNotFound, ErrAPIError, ErrPermission}
	for _, code := range codes {
		r := CodedErrorResult(code, errors.New("test"))
		got := extractText(t, r)
		expected := "[" + code + "] test"
		if got != expected {
			t.Errorf("CodedErrorResult(%s) = %q, want %q", code, got, expected)
		}
	}
}

// ==================== ActionableErrorResult ====================

func TestActionableErrorResult_NoSuggestions(t *testing.T) {
	r := ActionableErrorResult(ErrAPIError, errors.New("connection refused"))
	got := extractText(t, r)
	if !strings.HasPrefix(got, "[API_ERROR]") {
		t.Errorf("ActionableErrorResult prefix missing, got: %s", got)
	}
	if strings.Contains(got, "Suggested actions") {
		t.Error("ActionableErrorResult with no suggestions should not contain 'Suggested actions'")
	}
}

func TestActionableErrorResult_WithSuggestions(t *testing.T) {
	r := ActionableErrorResult(
		ErrClientInit,
		errors.New("cannot connect"),
		"Check that the service is running",
		"Verify credentials in .env",
	)
	got := extractText(t, r)
	if !strings.Contains(got, "Suggested actions:") {
		t.Errorf("ActionableErrorResult missing suggestions header, got: %s", got)
	}
	if !strings.Contains(got, "Check that the service is running") {
		t.Errorf("ActionableErrorResult missing first suggestion, got: %s", got)
	}
}

// ==================== Edge cases: non-map arguments ====================

func TestGetters_NonMapArguments(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = "not a map"

	if got := GetStringParam(req, "x"); got != "" {
		t.Errorf("GetStringParam non-map = %q, want empty", got)
	}
	if got := GetIntParam(req, "x", 10); got != 10 {
		t.Errorf("GetIntParam non-map = %d, want 10", got)
	}
	if got := GetBoolParam(req, "x", true); !got {
		t.Error("GetBoolParam non-map = false, want true")
	}
	if got := GetFloatParam(req, "x", 2.5); got != 2.5 {
		t.Errorf("GetFloatParam non-map = %f, want 2.5", got)
	}
	if HasParam(req, "x") {
		t.Error("HasParam non-map should return false")
	}
	if got := GetStringArrayParam(req, "x"); got != nil {
		t.Errorf("GetStringArrayParam non-map = %v, want nil", got)
	}
}

// ==================== RequireStringParam ====================

func TestRequireStringParam_Present(t *testing.T) {
	req := makeReq(map[string]any{"name": "alice"})
	val, errResult := RequireStringParam(req, "name")
	if errResult != nil {
		t.Fatalf("expected no error result, got %v", errResult)
	}
	if val != "alice" {
		t.Errorf("val = %q, want %q", val, "alice")
	}
}

func TestRequireStringParam_Empty(t *testing.T) {
	req := makeReq(map[string]any{"name": ""})
	_, errResult := RequireStringParam(req, "name")
	if errResult == nil {
		t.Fatal("expected error result for empty string")
	}
	text := extractText(t, errResult)
	if !strings.Contains(text, "INVALID_PARAM") {
		t.Errorf("error should contain INVALID_PARAM, got %q", text)
	}
}

func TestRequireStringParam_Missing(t *testing.T) {
	req := makeReq(map[string]any{})
	_, errResult := RequireStringParam(req, "name")
	if errResult == nil {
		t.Fatal("expected error result for missing param")
	}
}

func TestRequireStringParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	_, errResult := RequireStringParam(req, "name")
	if errResult == nil {
		t.Fatal("expected error result for nil args")
	}
}

// ==================== RequireIntParam ====================

func TestRequireIntParam_Present(t *testing.T) {
	req := makeReq(map[string]any{"count": float64(42)})
	val, errResult := RequireIntParam(req, "count")
	if errResult != nil {
		t.Fatalf("expected no error result, got %v", errResult)
	}
	if val != 42 {
		t.Errorf("val = %d, want 42", val)
	}
}

func TestRequireIntParam_Zero(t *testing.T) {
	req := makeReq(map[string]any{"count": float64(0)})
	val, errResult := RequireIntParam(req, "count")
	if errResult != nil {
		t.Fatalf("expected no error for zero value, got %v", errResult)
	}
	if val != 0 {
		t.Errorf("val = %d, want 0", val)
	}
}

func TestRequireIntParam_Missing(t *testing.T) {
	req := makeReq(map[string]any{})
	_, errResult := RequireIntParam(req, "count")
	if errResult == nil {
		t.Fatal("expected error result for missing param")
	}
}

func TestRequireIntParam_WrongType(t *testing.T) {
	req := makeReq(map[string]any{"count": "not a number"})
	_, errResult := RequireIntParam(req, "count")
	if errResult == nil {
		t.Fatal("expected error result for wrong type")
	}
}

func TestRequireIntParam_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	_, errResult := RequireIntParam(req, "count")
	if errResult == nil {
		t.Fatal("expected error result for nil args")
	}
}
