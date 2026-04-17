// Package handler provides helpers for building MCP tool handlers.
//
// It offers a generic [TypedHandler] that auto-generates JSON Schema from Go
// structs via jsonschema tags, type-safe parameter extraction functions
// ([GetStringParam], [GetIntParam], [GetBoolParam], etc.), result builder
// functions ([TextResult], [JSONResult], [ErrorResult], [CodedErrorResult]),
// structured output via [StructuredResult], and MCP elicitation builders for
// requesting additional user input mid-call.
//
// Key types: [TypedHandler], and error code constants [ErrInvalidParam],
// [ErrNotFound], [ErrAPIError], [ErrPermission], [ErrTimeout].
//
// Example using TypedHandler:
//
//	type GreetInput struct {
//	    Name string `json:"name" jsonschema:"required,description=Person to greet"`
//	}
//	type GreetOutput struct {
//	    Message string `json:"message"`
//	}
//	td := handler.TypedHandler[GreetInput, GreetOutput]("greet", "Say hello",
//	    func(ctx context.Context, in GreetInput) (*GreetOutput, error) {
//	        return &GreetOutput{Message: "hello, " + in.Name}, nil
//	    })
//
// # Token-efficient patterns for large-result tools
//
// For tools that return query results, search hits, or log streams, use the
// three helpers in pagination.go to keep caller context budgets in check:
//
//   - [Paginate] — generic cursor-based paging over a slice. Returns a [Page]
//     with Items, NextCursor, Total. Default limit is 50.
//   - [TruncateResult] — byte-budget enforcement on a built *CallToolResult.
//     Emits a RESULT_TRUNCATED marker when output exceeds the ceiling.
//   - [SchemaFirstResult] — dbhub-style schema-before-data. First call returns
//     metadata; caller opts into full data via a schema_only=false input.
//
// Composition pattern (see examples/pagination for a runnable server):
//
//	result := handler.SchemaFirstResult(input.SchemaOnly, mySchema,
//	    func() (any, error) {
//	        rows := fetchAll()
//	        if input.MinPrice > 0 { rows = filter(rows, input.MinPrice) }
//	        return handler.Paginate(rows, handler.PageCursor(input.Cursor), input.Limit), nil
//	    })
//	return handler.TruncateResult(result, 16*1024), nil
//
// The filter-before-paginate ordering is intentional: it keeps cursors stable
// for a given filter combination so the caller can page through a filtered
// set without surprise reordering.
package handler
