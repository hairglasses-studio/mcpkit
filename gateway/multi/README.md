# gateway/multi

The first Go multi-protocol agent gateway. Route MCP, A2A, and OpenAI function calling requests to a single mcpkit tool registry through automatic protocol detection and canonical translation.

## Why

Agent ecosystems are fragmenting across protocols. MCP clients, A2A agents, and OpenAI-compatible toolchains all want to invoke the same tools but speak different wire formats. `gateway/multi` unifies them behind one HTTP endpoint: register your tools once, serve every protocol.

## Quick Start

```go
package main

import (
    "log"
    "net/http"

    "github.com/hairglasses-studio/mcpkit/gateway/multi"
    "github.com/hairglasses-studio/mcpkit/handler"
    "github.com/hairglasses-studio/mcpkit/registry"
)

type EchoInput struct {
    Message string `json:"message" jsonschema:"required"`
}

type EchoOutput struct {
    Reply string `json:"reply"`
}

func main() {
    // 1. Register tools.
    reg := registry.NewToolRegistry()
    td := handler.TypedHandler[EchoInput, EchoOutput](
        "echo", "Echo a message back",
        func(ctx context.Context, in EchoInput) (EchoOutput, error) {
            return EchoOutput{Reply: in.Message}, nil
        },
    )
    reg.RegisterTool(td)

    // 2. Create the gateway router.
    router := multi.NewRouter(reg)

    // 3. Register protocol adapters.
    router.Register(multi.NewMCPAdapter())
    router.Register(multi.NewA2AAdapter())
    router.Register(multi.NewOpenAIAdapter())

    // 4. Serve — one endpoint, three protocols.
    log.Fatal(http.ListenAndServe(":9090", router))
}
```

Clients can now hit `:9090` with MCP JSON-RPC, A2A `sendMessage`, or OpenAI `tool_calls` — the gateway detects the protocol and responds in kind.

## Architecture

```
                        HTTP Request
                             |
                    +--------v---------+
                    |   Body Peek      |
                    |   (512 bytes)    |
                    +--------+---------+
                             |
                    +--------v---------+
                    |  DetectProtocol   |
                    |                   |
                    |  1. Headers       |  MCP-Protocol-Version? -> MCP (definitive)
                    |  2. Path          |  /.well-known/agent-card.json? -> A2A (definitive)
                    |  3. Path prefix   |  /mcp/, /a2a/, /openai/ -> (high)
                    |  4. JSON-RPC      |  tools/call -> MCP, a2a.* -> A2A (definitive)
                    |  5. Body shape    |  tool_calls -> OpenAI (high)
                    +--------+---------+
                             |
                    +--------v---------+
                    |  Adapter.Decode   |  Protocol-specific -> CanonicalRequest
                    +--------+---------+
                             |
                    +--------v---------+
                    |  ToolRegistry     |  Look up tool, invoke handler
                    |  (mcpkit)         |
                    +--------+---------+
                             |
                    +--------v---------+
                    |  Adapter.Encode   |  CanonicalResponse -> Protocol-specific
                    +--------+---------+
                             |
                        HTTP Response
```

The body is buffered once (up to 512 bytes) for detection, then made available in full to the selected adapter's `Decode` method. No double-reads, no request cloning.

## Protocol Support

| Protocol | Wire Format | Detection Signals | Adapter |
|----------|-------------|-------------------|---------|
| **MCP** | JSON-RPC 2.0 | `MCP-Protocol-Version` header, `/mcp` path, `tools/call` method | `MCPAdapter` |
| **A2A** | JSON-RPC 2.0 + REST | `/.well-known/agent-card.json` path, `/a2a` path, `a2a.*` methods | `A2AAdapter` |
| **OpenAI** | Function calling JSON | `/v1/chat/completions` path, `/openai` path, `tool_calls` in body | `OpenAIAdapter` |

## Detection Logic

Protocol detection runs a 4-layer priority chain. Each layer is cheaper than the next:

1. **Custom headers** (no body read) — `MCP-Protocol-Version` or `Mcp-Session-Id` headers yield a definitive MCP match.
2. **Well-known paths** — `/.well-known/agent-card.json` and `/agent-card:extended` are definitive A2A. Path prefixes (`/mcp/`, `/a2a/`, `/openai/`, `/v1/chat/`) give high confidence.
3. **JSON-RPC method** (body peek) — For POST requests with JSON content, the `"method"` field is extracted via byte scanning (no full JSON parse). MCP methods (`tools/call`, `initialize`, etc.) and A2A methods (`a2a.sendMessage`, etc.) are definitive.
4. **Body structure** (body peek) — Presence of `"tool_calls"`, `"function_call"`, or `"functions"` fields indicates OpenAI format at high confidence.

If global detection is ambiguous, the router consults each registered adapter's `Detect` method and picks the highest-confidence match. Minimum threshold is medium confidence.

### Confidence Levels

| Level | Value | Meaning |
|-------|-------|---------|
| `ConfidenceLow` | 0 | Guess / fallback |
| `ConfidenceMedium` | 1 | Partial signals (e.g., Content-Type only) |
| `ConfidenceHigh` | 2 | Strong structural match (e.g., path prefix) |
| `ConfidenceDefinitive` | 3 | Unambiguous signal (e.g., JSON-RPC method) |

## Canonical Data Model

All adapters translate to/from a shared canonical representation:

```go
// CanonicalRequest — protocol-agnostic tool call.
type CanonicalRequest struct {
    Protocol  Protocol           // Which protocol originated this
    ToolName  string             // Normalized tool name
    Arguments map[string]any     // Tool input
    RequestID string             // Caller's request ID
    Auth      *AuthContext       // Optional identity
    Metadata  map[string]string  // Protocol-specific round-trip data
}

// CanonicalResponse — protocol-agnostic tool result.
type CanonicalResponse struct {
    Success   bool
    Content   []ContentPart
    Error     *ErrorInfo
    RequestID string
    Metadata  map[string]string
}
```

Content parts support text, JSON, image, and raw data types. Error codes (`ErrInvalidParams`, `ErrNotFound`, `ErrUnauthorized`, `ErrForbidden`, `ErrRateLimit`, `ErrTimeout`, `ErrInternal`) map to appropriate HTTP status codes and protocol-specific error formats.

## Adapter API Reference

### MCPAdapter

Handles MCP JSON-RPC 2.0 requests. Supports `tools/call` as tool invocations and passes through lifecycle methods (`initialize`, `ping`, `tools/list`) as metadata-only canonical requests.

```go
adapter := multi.NewMCPAdapter()
router.Register(adapter)
```

**Decode behavior:**
- `tools/call` — extracts `params.name` as `ToolName`, `params.arguments` as `Arguments`, JSON-RPC `id` as `RequestID`
- Lifecycle methods — decoded with empty `ToolName`, method stored in `Metadata["jsonrpc.method"]`
- Progress token preserved in `Metadata["mcp.progressToken"]`

**Encode behavior:**
- Success — JSON-RPC result with `{content: [...], isError: false}`
- Tool error — JSON-RPC result with `isError: true`
- Protocol error (invalid params, not found) — JSON-RPC error response with standard codes

### A2AAdapter

Handles A2A JSON-RPC requests. Translates `a2a.sendMessage` into tool calls using the same skill extraction strategy as `bridge/a2a`.

```go
adapter := multi.NewA2AAdapter()
router.Register(adapter)
```

**Decode behavior:**
- `a2a.sendMessage` — extracts skill name and arguments from message parts:
  1. **DataPart** (preferred): looks for `{"skill": "tool_name", "arguments": {...}}`
  2. **TextPart** fallback: parses JSON with optional `"skill"` field
- Non-sendMessage methods — decoded with empty `ToolName`, method in `Metadata["a2a.method"]`
- Context ID and Task ID preserved in metadata

**Encode behavior:**
- Success — JSON-RPC result containing an A2A `Task` with `COMPLETED` status and artifact parts
- Error — JSON-RPC result with `FAILED` task status and error message

### OpenAIAdapter

Handles OpenAI function calling format. Extracts tool calls from chat completion messages. No OpenAI SDK dependency — pure JSON parsing.

```go
adapter := multi.NewOpenAIAdapter(
    multi.WithModelName("my-gateway"),  // default: "mcpkit-gateway"
)
router.Register(adapter)
```

**Decode behavior:**
- Finds the last assistant message with `tool_calls` (or legacy `function_call`)
- Extracts the first tool call's `function.name` as `ToolName` and parses `function.arguments` JSON string as `Arguments`
- Bearer token from `Authorization` header stored in `Auth`

**Encode behavior:**
- Wraps the result as a chat completion response with a `tool` role message
- Content parts concatenated into a single string
- Model name configurable via `WithModelName`

## Router

The `Router` is an `http.Handler` that composes adapters:

```go
router := multi.NewRouter(reg,
    multi.WithLogger(slog.Default()),  // structured logging
)

router.Register(multi.NewMCPAdapter())
router.Register(multi.NewA2AAdapter())
router.Register(multi.NewOpenAIAdapter())

// Use as standard http.Handler.
http.ListenAndServe(":9090", router)
```

Key behaviors:
- Body is peeked (512 bytes max) once and reconstructed for full reading by the adapter
- If no adapter matches, returns 400 with a JSON error listing supported protocols
- Decode failures return 400 in the matched protocol's error format
- Tool not found returns the adapter's error format with `ErrNotFound`
- Thread-safe: adapters can be registered or queried concurrently

## Adding Custom Adapters

Implement the `Adapter` interface:

```go
type Adapter interface {
    Protocol() Protocol
    Detect(r *http.Request, bodyPeek []byte) (matches bool, confidence Confidence)
    Decode(r *http.Request) (*CanonicalRequest, error)
    Encode(resp *CanonicalResponse) (body []byte, contentType string, err error)
}
```

Contract:
- `Detect` must not consume the request body (only inspect the pre-buffered `bodyPeek`)
- `Decode` may fully read the request body (it has been buffered)
- `Encode` returns the serialized response; the router handles HTTP status and headers
- All methods must be safe for concurrent use

Register with `router.Register(myAdapter)`. The router's detection pipeline will include your adapter's `Detect` method in the fallback chain.

## Performance

- Protocol detection uses byte scanning, not full JSON parsing — the `"method"` field is extracted from the 512-byte peek without `json.Unmarshal`
- Body is peeked once via `io.ReadAtLeast` and reconstructed with `io.MultiReader` — no double buffering
- Adapters are stateless (except OpenAI's configurable model name) — zero per-request allocation beyond the canonical types
- Router uses `sync.RWMutex` — concurrent request handling with lock-free reads when adapters are stable

## Limitations

- **Single tool call per request** — The OpenAI adapter extracts only the first `tool_call`. Batch tool calls require multiple requests or a batching layer.
- **No streaming** — All adapters use request-response semantics. MCP SSE streaming and A2A streaming responses are not supported.
- **No lifecycle passthrough** — MCP lifecycle methods (`initialize`, `ping`, `tools/list`) and non-sendMessage A2A methods are decoded but not executed; they require custom handling.
- **No auth bridging** — Each protocol's auth is extracted but not translated between protocols.

## Roadmap

| Feature | Status |
|---------|--------|
| MCP adapter | Done |
| A2A adapter | Done |
| OpenAI adapter | Done |
| Protocol auto-detection | Done |
| Canonical request/response model | Done |
| Custom adapter interface | Done |
| Batch tool calls (OpenAI multi-tool) | Planned |
| MCP SSE streaming | Planned |
| A2A streaming responses | Planned |
| Auth bridging between protocols | Planned |
| Metrics and tracing per-protocol | Planned |

## Dependencies

- [mcpkit/registry](../../registry/) — Tool registration, lookup, and invocation
- [a2aproject/a2a-go/v2](https://github.com/a2aproject/a2a-go) — A2A protocol types for the A2A adapter
