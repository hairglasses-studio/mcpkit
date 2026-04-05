//go:build !official_sdk

package handler

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// ==================== StructuredResult ====================

func TestStructuredResult_HasTextContent(t *testing.T) {
	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r := StructuredResult(item{ID: 1, Name: "widget"})
	if r == nil {
		t.Fatal("StructuredResult should not return nil")
	}
	if len(r.Content) == 0 {
		t.Fatal("StructuredResult should have at least one content item")
	}
	text := extractText(t, r)
	if !strings.Contains(text, `"id": 1`) {
		t.Errorf("StructuredResult text missing id field, got: %s", text)
	}
	if !strings.Contains(text, `"name": "widget"`) {
		t.Errorf("StructuredResult text missing name field, got: %s", text)
	}
}

func TestStructuredResult_HasStructuredContent(t *testing.T) {
	type point struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	data := point{X: 1.5, Y: 2.5}
	r := StructuredResult(data)
	if r.StructuredContent == nil {
		t.Error("StructuredResult should set StructuredContent")
	}
}

func TestStructuredResult_NotError(t *testing.T) {
	r := StructuredResult(map[string]any{"ok": true})
	if r.IsError {
		t.Error("StructuredResult with valid data should not be an error")
	}
}

func TestStructuredResult_TextMatchesJSON(t *testing.T) {
	// Text content should be indented JSON of the struct
	type rec struct {
		Status string `json:"status"`
	}
	r := StructuredResult(rec{Status: "active"})
	text := extractText(t, r)
	if !strings.HasPrefix(text, "{") {
		t.Errorf("StructuredResult text should be JSON object, got: %s", text)
	}
	if !strings.Contains(text, `"status": "active"`) {
		t.Errorf("StructuredResult text missing status field, got: %s", text)
	}
}

func TestStructuredResult_Unmarshalable(t *testing.T) {
	// A channel cannot be marshaled to JSON
	r := StructuredResult(make(chan int))
	if r == nil {
		t.Fatal("StructuredResult should not return nil on error")
	}
	if !r.IsError {
		t.Error("StructuredResult with unmarshalable input should return an error result")
	}
}

func TestStructuredResult_Unmarshalable_NoPanic(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("StructuredResult panicked: %v", rec)
		}
	}()
	_ = StructuredResult(make(chan struct{}))
}

func TestStructuredResult_Slice(t *testing.T) {
	r := StructuredResult([]int{1, 2, 3})
	if r.IsError {
		t.Error("StructuredResult with slice should not be an error")
	}
	text := extractText(t, r)
	if !strings.Contains(text, "1") {
		t.Errorf("StructuredResult slice text = %q, expected elements", text)
	}
}

// ==================== GetResponseFormat ====================

func makeReqWithFormat(format string) mcp.CallToolRequest {
	return makeReq(map[string]any{"response_format": format})
}

func TestGetResponseFormat_Default(t *testing.T) {
	// No response_format argument — should default to FormatDetailed
	req := makeReq(map[string]any{})
	got := GetResponseFormat(req)
	if got != FormatDetailed {
		t.Errorf("GetResponseFormat default = %q, want %q", got, FormatDetailed)
	}
}

func TestGetResponseFormat_Detailed(t *testing.T) {
	req := makeReqWithFormat("detailed")
	got := GetResponseFormat(req)
	if got != FormatDetailed {
		t.Errorf("GetResponseFormat detailed = %q, want %q", got, FormatDetailed)
	}
}

func TestGetResponseFormat_Concise(t *testing.T) {
	req := makeReqWithFormat("concise")
	got := GetResponseFormat(req)
	if got != FormatConcise {
		t.Errorf("GetResponseFormat concise = %q, want %q", got, FormatConcise)
	}
}

func TestGetResponseFormat_Unknown(t *testing.T) {
	// Unknown values should fall back to FormatDetailed
	req := makeReqWithFormat("verbose")
	got := GetResponseFormat(req)
	if got != FormatDetailed {
		t.Errorf("GetResponseFormat unknown = %q, want %q", got, FormatDetailed)
	}
}

func TestGetResponseFormat_Empty(t *testing.T) {
	req := makeReqWithFormat("")
	got := GetResponseFormat(req)
	if got != FormatDetailed {
		t.Errorf("GetResponseFormat empty = %q, want %q", got, FormatDetailed)
	}
}

func TestGetResponseFormat_NilArgs(t *testing.T) {
	req := makeReqNilArgs()
	got := GetResponseFormat(req)
	if got != FormatDetailed {
		t.Errorf("GetResponseFormat nil args = %q, want %q", got, FormatDetailed)
	}
}

// ==================== ResponseFormatSchema ====================

func TestResponseFormatSchema_ReturnsMap(t *testing.T) {
	schema := ResponseFormatSchema()
	if schema == nil {
		t.Fatal("ResponseFormatSchema should return a non-nil map")
	}
}

func TestResponseFormatSchema_TypeIsString(t *testing.T) {
	schema := ResponseFormatSchema()
	typ, ok := schema["type"]
	if !ok {
		t.Fatal("ResponseFormatSchema missing 'type' key")
	}
	if typ != "string" {
		t.Errorf("ResponseFormatSchema type = %v, want %q", typ, "string")
	}
}

func TestResponseFormatSchema_EnumHasValues(t *testing.T) {
	schema := ResponseFormatSchema()
	rawEnum, ok := schema["enum"]
	if !ok {
		t.Fatal("ResponseFormatSchema missing 'enum' key")
	}
	enums, ok := rawEnum.([]string)
	if !ok {
		t.Fatalf("ResponseFormatSchema enum is not []string, got %T", rawEnum)
	}
	if len(enums) < 2 {
		t.Errorf("ResponseFormatSchema enum should have at least 2 values, got %v", enums)
	}
	hasDetailed := false
	hasConcise := false
	for _, v := range enums {
		if v == "detailed" {
			hasDetailed = true
		}
		if v == "concise" {
			hasConcise = true
		}
	}
	if !hasDetailed {
		t.Error("ResponseFormatSchema enum should include 'detailed'")
	}
	if !hasConcise {
		t.Error("ResponseFormatSchema enum should include 'concise'")
	}
}

func TestResponseFormatSchema_DefaultIsDetailed(t *testing.T) {
	schema := ResponseFormatSchema()
	def, ok := schema["default"]
	if !ok {
		t.Fatal("ResponseFormatSchema missing 'default' key")
	}
	if def != "detailed" {
		t.Errorf("ResponseFormatSchema default = %v, want %q", def, "detailed")
	}
}

func TestResponseFormatSchema_HasDescription(t *testing.T) {
	schema := ResponseFormatSchema()
	desc, ok := schema["description"]
	if !ok {
		t.Fatal("ResponseFormatSchema missing 'description' key")
	}
	descStr, ok := desc.(string)
	if !ok {
		t.Fatalf("ResponseFormatSchema description is not string, got %T", desc)
	}
	if descStr == "" {
		t.Error("ResponseFormatSchema description should not be empty")
	}
}
