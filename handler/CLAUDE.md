# handler

Helpers for building MCP tool handlers. Depends only on `registry`.

## Key Patterns

- **TypedHandler**: `handler.TypedHandler[In, Out](name, desc, fn)` — auto-generates input/output schemas from Go structs via `jsonschema` tags
- **Param extraction**: `GetStringParam`, `GetIntParam`, `GetBoolParam`, `GetFloatParam`, `GetStringArrayParam`, `HasParam` — all nil-safe, return zero/default on missing
- **Result builders**: `TextResult`, `JSONResult`, `ErrorResult`, `CodedErrorResult`, `ActionableErrorResult`, `StructuredResult`
- **Content helpers**: `content.go` — image/audio/resource content builders with MIME detection
- **Elicitation**: `ElicitForm(msg, schema)`, `ElicitURL(msg, id, url)`, `ElicitFormSchema(fields...)` — builds MCP elicitation params

## Struct Tags for TypedHandler

```go
type Input struct {
    Query string `json:"query" jsonschema:"required,description=Search query"`
    Limit int    `json:"limit,omitempty" jsonschema:"description=Max results"`
}
```

## Error Code Constants

`ErrClientInit`, `ErrInvalidParam`, `ErrTimeout`, `ErrNotFound`, `ErrAPIError`, `ErrPermission` — defined in `result.go`
