package registry

import "testing"

func TestNewCallToolRequest(t *testing.T) {
	req, err := NewCallToolRequest("demo", map[string]any{"query": "hello", "limit": float64(2)})
	if err != nil {
		t.Fatalf("NewCallToolRequest: %v", err)
	}
	if req.Params.Name != "demo" {
		t.Fatalf("Name = %q, want demo", req.Params.Name)
	}

	args := ExtractArguments(req)
	if args["query"] != "hello" {
		t.Fatalf("query = %v, want hello", args["query"])
	}
	if args["limit"] != float64(2) {
		t.Fatalf("limit = %v, want 2", args["limit"])
	}
}

func TestSetCallToolArgumentsNilRequest(t *testing.T) {
	if err := SetCallToolArguments(nil, map[string]any{"x": 1}); err == nil {
		t.Fatal("expected nil request error")
	}
}
