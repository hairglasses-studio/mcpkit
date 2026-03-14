package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

type testInput struct {
	Query string `json:"query" jsonschema:"required,description=Search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Max results"`
}

type testOutput struct {
	Results []string `json:"results"`
	Total   int      `json:"total"`
}

func TestTypedHandler_Basic(t *testing.T) {
	td := TypedHandler[testInput, testOutput](
		"test_search",
		"Test search tool",
		func(ctx context.Context, input testInput) (testOutput, error) {
			return testOutput{
				Results: []string{"a", "b"},
				Total:   2,
			}, nil
		},
	)

	if td.Tool.Name != "test_search" {
		t.Errorf("name = %q, want %q", td.Tool.Name, "test_search")
	}
	if td.Tool.Description != "Test search tool" {
		t.Errorf("description = %q", td.Tool.Description)
	}
	if td.OutputSchema == nil {
		t.Fatal("output schema is nil")
	}
	if td.OutputSchema.Type != "object" {
		t.Errorf("output schema type = %q, want object", td.OutputSchema.Type)
	}
	if _, ok := td.OutputSchema.Properties["results"]; !ok {
		t.Error("output schema missing 'results' property")
	}
	if _, ok := td.OutputSchema.Properties["total"]; !ok {
		t.Error("output schema missing 'total' property")
	}
}

func TestTypedHandler_InputSchema(t *testing.T) {
	td := TypedHandler[testInput, testOutput](
		"test",
		"test",
		func(ctx context.Context, input testInput) (testOutput, error) {
			return testOutput{}, nil
		},
	)

	if td.Tool.InputSchema.Type != "object" {
		t.Errorf("input schema type = %q", td.Tool.InputSchema.Type)
	}
	if _, ok := td.Tool.InputSchema.Properties["query"]; !ok {
		t.Error("input schema missing 'query' property")
	}
}

func TestTypedHandler_Execution(t *testing.T) {
	td := TypedHandler[testInput, testOutput](
		"test",
		"test",
		func(ctx context.Context, input testInput) (testOutput, error) {
			return testOutput{
				Results: []string{input.Query},
				Total:   1,
			}, nil
		},
	)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"query": "hello",
		"limit": float64(10),
	}

	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.IsError {
		t.Error("result should not be error")
	}

	// Check structured content
	if result.StructuredContent == nil {
		t.Fatal("structured content is nil")
	}

	// Check text content
	if len(result.Content) == 0 {
		t.Fatal("no text content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content is not TextContent")
	}

	var out testOutput
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("failed to unmarshal text content: %v", err)
	}
	if out.Total != 1 || len(out.Results) != 1 || out.Results[0] != "hello" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestTypedHandler_Error(t *testing.T) {
	td := TypedHandler[testInput, testOutput](
		"test",
		"test",
		func(ctx context.Context, input testInput) (testOutput, error) {
			return testOutput{}, fmt.Errorf("something went wrong")
		},
	)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"query": "test"}

	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler should not return error: %v", err)
	}
	if !result.IsError {
		t.Error("result should be an error")
	}
}

func TestTypedHandler_NilArgs(t *testing.T) {
	td := TypedHandler[testInput, testOutput](
		"test",
		"test",
		func(ctx context.Context, input testInput) (testOutput, error) {
			return testOutput{Total: 0}, nil
		},
	)

	req := mcp.CallToolRequest{}
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Error("result should not be an error for nil args")
	}
}
