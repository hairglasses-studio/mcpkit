package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/registry"
)

func TestEncodeDecodeOffsetCursor(t *testing.T) {
	for _, offset := range []int{0, 1, 50, 999, 1_000_000} {
		cursor := EncodeOffsetCursor(offset)
		if cursor.Empty() && offset != 0 {
			t.Errorf("cursor empty for offset %d", offset)
		}
		got, err := DecodeOffsetCursor(cursor)
		if err != nil {
			t.Errorf("decode offset %d: %v", offset, err)
			continue
		}
		if got != offset {
			t.Errorf("offset roundtrip: got %d, want %d", got, offset)
		}
	}
}

func TestDecodeOffsetCursor_Empty(t *testing.T) {
	got, err := DecodeOffsetCursor("")
	if err != nil {
		t.Errorf("empty cursor should not error: %v", err)
	}
	if got != 0 {
		t.Errorf("empty cursor should decode to 0, got %d", got)
	}
}

func TestDecodeOffsetCursor_Malformed(t *testing.T) {
	_, err := DecodeOffsetCursor("not-base64!")
	if err == nil {
		t.Error("malformed cursor should return error")
	}
}

func TestDecodeOffsetCursor_WrongScheme(t *testing.T) {
	// Well-formed base64 but wrong payload format
	bogus := PageCursor("aGVsbG8") // "hello"
	_, err := DecodeOffsetCursor(bogus)
	if err == nil {
		t.Error("wrong scheme should return error")
	}
}

func TestPaginate_FirstPage(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	page := Paginate(items, "", 3)

	if len(page.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(page.Items))
	}
	if page.Items[0] != 1 || page.Items[2] != 3 {
		t.Errorf("wrong items: %v", page.Items)
	}
	if page.NextCursor.Empty() {
		t.Error("expected next cursor, got empty")
	}
	if page.Total != 10 {
		t.Errorf("total = %d, want 10", page.Total)
	}
}

func TestPaginate_SecondPage(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	first := Paginate(items, "", 3)
	second := Paginate(items, first.NextCursor, 3)

	if len(second.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(second.Items))
	}
	if second.Items[0] != 4 || second.Items[2] != 6 {
		t.Errorf("wrong second page items: %v", second.Items)
	}
}

func TestPaginate_LastPagePartial(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	page := Paginate(items, EncodeOffsetCursor(3), 10)

	if len(page.Items) != 2 {
		t.Errorf("expected 2 items on last page, got %d", len(page.Items))
	}
	if !page.NextCursor.Empty() {
		t.Errorf("expected empty next cursor at end, got %q", page.NextCursor)
	}
}

func TestPaginate_DefaultLimit(t *testing.T) {
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	page := Paginate(items, "", 0) // 0 triggers default (50)
	if len(page.Items) != 50 {
		t.Errorf("default limit should return 50, got %d", len(page.Items))
	}
}

func TestPaginate_OffsetBeyondEnd(t *testing.T) {
	items := []int{1, 2, 3}
	// Malformed cursor past the end — should reset to offset 0
	page := Paginate(items, EncodeOffsetCursor(100), 10)
	if len(page.Items) != 3 {
		t.Errorf("out-of-range offset should reset, got %d items", len(page.Items))
	}
}

func TestPaginate_Empty(t *testing.T) {
	page := Paginate([]int{}, "", 10)
	if len(page.Items) != 0 {
		t.Errorf("empty input should return empty page")
	}
	if !page.NextCursor.Empty() {
		t.Error("empty input should not produce next cursor")
	}
}

func TestTruncateResult_UnderBudget(t *testing.T) {
	r := registry.MakeTextResult("small")
	got := TruncateResult(r, 1024)
	if got != r {
		t.Error("under-budget result should be returned unchanged")
	}
}

func TestTruncateResult_OverBudget(t *testing.T) {
	big := strings.Repeat("A", 5000)
	r := registry.MakeTextResult(big)
	got := TruncateResult(r, 100)

	encoded, _ := json.Marshal(got)
	if len(encoded) > 500 {
		t.Errorf("truncated result should be small, got %d bytes", len(encoded))
	}
	// Find text content
	var text string
	for _, c := range got.Content {
		b, _ := json.Marshal(c)
		if strings.Contains(string(b), "RESULT_TRUNCATED") {
			text = string(b)
			break
		}
	}
	if text == "" {
		t.Error("truncated result should carry RESULT_TRUNCATED marker")
	}
}

func TestTruncateResult_ZeroBudget(t *testing.T) {
	r := registry.MakeTextResult("anything")
	got := TruncateResult(r, 0)
	if got != r {
		t.Error("zero budget = unlimited (no-op)")
	}
}

func TestTruncateResult_Nil(t *testing.T) {
	if TruncateResult(nil, 100) != nil {
		t.Error("nil should return nil")
	}
}

func TestSchemaFirstResult_SchemaOnly(t *testing.T) {
	schema := map[string]any{"id": "int", "name": "string"}
	called := false
	produce := func() (any, error) {
		called = true
		return nil, nil
	}
	result := SchemaFirstResult(true, schema, produce)

	if called {
		t.Error("producer should not be invoked in schema-only mode")
	}
	b, _ := json.Marshal(result)
	if !strings.Contains(string(b), "schema") {
		t.Errorf("schema-only result should include schema: %s", b)
	}
}

func TestSchemaFirstResult_FullData(t *testing.T) {
	schema := map[string]any{"id": "int"}
	data := map[string]any{"id": 42}
	result := SchemaFirstResult(false, schema, func() (any, error) {
		return data, nil
	})
	b, _ := json.Marshal(result)
	if !strings.Contains(string(b), "42") {
		t.Errorf("full-data result should include produced data: %s", b)
	}
}

func TestSchemaFirstResult_ProducerError(t *testing.T) {
	schema := map[string]any{}
	result := SchemaFirstResult(false, schema, func() (any, error) {
		return nil, errors.New("db down")
	})
	b, _ := json.Marshal(result)
	if !strings.Contains(string(b), "db down") {
		t.Errorf("producer error should surface in result: %s", b)
	}
}
