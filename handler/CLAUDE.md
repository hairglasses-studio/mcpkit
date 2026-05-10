# handler

Helpers for building MCP tool handlers. Depends only on `registry`.

## Key Patterns

- **TypedHandler**: `handler.TypedHandler[In, Out](name, desc, fn)` — auto-generates input/output schemas from Go structs via `jsonschema` tags
- **Param extraction (optional)**: `GetStringParam`, `GetIntParam`, `GetBoolParam`, `GetFloatParam`, `GetStringArrayParam`, `HasParam` — all nil-safe, return zero/default on missing
- **Param extraction (required)**: `RequireStringParam(req, name) (string, *CallToolResult)` and `RequireIntParam(req, name) (int, *CallToolResult)` — return a pre-built error result when the param is missing/empty/invalid. Prefer these over manual `if value == "" { return ErrorResult(...) }` blocks; downstream MCP servers (hg-mcp, jellyfin-mcp-deluxe, shielddd) have ~hundreds of those copy-pasted today and should migrate.
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
