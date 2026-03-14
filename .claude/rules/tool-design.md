---
paths: ["**/*.go"]
---

# MCP Tool Design Rules

## Descriptions Are Onboarding Docs

Tool descriptions are the primary way an LLM learns what a tool does (Anthropic research shows better descriptions improve tool selection accuracy from 72% to 90%). Write them as if onboarding a new developer:

- Lead with **what** the tool does, then **when** to use it
- Specify parameter constraints (min/max, allowed values, formats)
- Use `handler.FormatExamples()` to embed input/output examples in the description — this significantly improves accuracy

## Parameter Naming

- Use unambiguous names: `user_id` not `id`, `search_query` not `q`
- Add `jsonschema:"required,description=..."` tags on all TypedHandler input fields
- Optional fields: use `json:"...,omitempty"` and document the default in the description tag

## Error Handling

Always return `(*CallToolResult, nil)` — never `(nil, err)`. MCP tools communicate errors through the result content, not Go errors:

```go
// CORRECT
return handler.CodedErrorResult(handler.ErrInvalidParam, err), nil

// WRONG — breaks MCP protocol expectations
return nil, err
```

Error code constants are defined in `handler/result.go`: `ErrClientInit`, `ErrInvalidParam`, `ErrTimeout`, `ErrNotFound`, `ErrAPIError`, `ErrPermission`.

## Deferred Loading

When a server registers more than 10 tools, set `DeferLoading: true` on lower-priority tool definitions to reduce initial handshake payload. The client will fetch the full schema on demand.

## Struct Tags for TypedHandler

```go
type SearchInput struct {
    Query  string `json:"query"  jsonschema:"required,description=Search query string"`
    Limit  int    `json:"limit,omitempty" jsonschema:"description=Max results (default 10),minimum=1,maximum=100"`
}
```
