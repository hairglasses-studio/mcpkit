package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hairglasses-studio/mcpkit/handler"
)

// End-to-end tests for the pagination example, exercising SchemaFirstResult,
// Paginate, and TruncateResult composition through the handler surface.

func TestListProducts_SchemaOnlyDefault(t *testing.T) {
	// Empty input → schema_only default path returns schema metadata
	out, err := handleListProducts(context.Background(), ListProductsInput{SchemaOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Schema == nil {
		t.Error("expected schema in output, got nil")
	}
	if out.Page != nil {
		t.Error("schema-only call should not include page data")
	}
	if !strings.Contains(out.Hint, "schema_only=false") {
		t.Errorf("hint should guide caller to fetch data: %q", out.Hint)
	}
}

func TestListProducts_FirstPage(t *testing.T) {
	out, err := handleListProducts(context.Background(), ListProductsInput{
		SchemaOnly: false,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Page == nil {
		t.Fatal("data call should include page")
	}
	if len(out.Page.Items) != 10 {
		t.Errorf("limit=10 should yield 10 items, got %d", len(out.Page.Items))
	}
	if out.Page.Items[0].ID != 1 {
		t.Errorf("first page should start at ID 1, got %d", out.Page.Items[0].ID)
	}
	if out.Page.NextCursor.Empty() {
		t.Error("first page of 500 with limit=10 should have a next cursor")
	}
	if out.Page.Total != 500 {
		t.Errorf("total should be 500, got %d", out.Page.Total)
	}
}

func TestListProducts_CursorFlow(t *testing.T) {
	// Fetch page 1
	p1, _ := handleListProducts(context.Background(), ListProductsInput{
		SchemaOnly: false,
		Limit:      10,
	})
	// Pass cursor back for page 2
	p2, err := handleListProducts(context.Background(), ListProductsInput{
		SchemaOnly: false,
		Limit:      10,
		Cursor:     string(p1.Page.NextCursor),
	})
	if err != nil {
		t.Fatalf("page 2 error: %v", err)
	}
	if p2.Page.Items[0].ID != 11 {
		t.Errorf("page 2 should start at ID 11, got %d", p2.Page.Items[0].ID)
	}
}

func TestListProducts_MinPriceFilter(t *testing.T) {
	// generateProducts creates prices 9.99 + i*0.10; so ID 100 ≈ $19.89
	// Set min_price = $30.00 → filter keeps the tail (~200+ items)
	out, err := handleListProducts(context.Background(), ListProductsInput{
		SchemaOnly: false,
		MinPrice:   30.00,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Page == nil {
		t.Fatal("expected page, got nil")
	}
	for _, p := range out.Page.Items {
		if p.Price < 30.00 {
			t.Errorf("filter leaked: product %d price $%.2f < $30.00", p.ID, p.Price)
		}
	}
	if out.Page.Total >= 500 {
		t.Errorf("filter should reduce total below 500, got %d", out.Page.Total)
	}
}

func TestListProducts_LastPagePartialNoCursor(t *testing.T) {
	// Fetch a late page that contains the final items → no next cursor
	out, err := handleListProducts(context.Background(), ListProductsInput{
		SchemaOnly: false,
		Cursor:     string(handler.EncodeOffsetCursor(495)),
		Limit:      100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Page.Items) != 5 {
		t.Errorf("last 5 items expected, got %d", len(out.Page.Items))
	}
	if !out.Page.NextCursor.Empty() {
		t.Errorf("last page should have empty next_cursor, got %q", out.Page.NextCursor)
	}
}

func TestListProducts_FilterStableCursor(t *testing.T) {
	// Apply the same filter twice with the same cursor — results must match.
	// This validates the documented "filter before paginate" contract.
	first, _ := handleListProducts(context.Background(), ListProductsInput{
		SchemaOnly: false,
		MinPrice:   20.00,
		Limit:      5,
	})
	second, _ := handleListProducts(context.Background(), ListProductsInput{
		SchemaOnly: false,
		MinPrice:   20.00,
		Limit:      5,
	})
	if len(first.Page.Items) != len(second.Page.Items) {
		t.Fatalf("filter is not deterministic")
	}
	for i := range first.Page.Items {
		if first.Page.Items[i].ID != second.Page.Items[i].ID {
			t.Errorf("filter result diverged at %d: %d vs %d",
				i, first.Page.Items[i].ID, second.Page.Items[i].ID)
		}
	}
}

func TestProductsSchema_HasExpectedShape(t *testing.T) {
	schema := productsSchema()
	b, _ := json.Marshal(schema)
	s := string(b)
	for _, required := range []string{"row_shape", "total_rows", "id", "price", "stock"} {
		if !strings.Contains(s, required) {
			t.Errorf("schema missing %q: %s", required, s)
		}
	}
}
