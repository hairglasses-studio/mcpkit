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
package handler
