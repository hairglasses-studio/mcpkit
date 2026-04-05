# bridge/a2a

The first production-quality Go library bridging the [MCP](https://modelcontextprotocol.io/) (Model Context Protocol) and [A2A](https://google.github.io/A2A/) (Agent-to-Agent) protocols.

This package enables mcpkit-based MCP servers to participate in A2A agent networks. MCP tools are exposed as A2A skills, allowing remote A2A clients to discover and invoke deterministic tool handlers through the standard A2A task protocol.

## Quick Start

```go
// Register MCP tools as usual.
reg := registry.NewToolRegistry()
reg.RegisterModule(&MyModule{})

// Create the bridge executor.
executor := a2a.NewBridgeExecutor(reg, a2a.ExecutorConfig{})

// Generate an A2A agent card from registered tools.
cardGen := a2a.NewAgentCardGenerator(reg, nil, a2a.CardConfig{
    Name:        "my-agent",
    Description: "MCP tools exposed as an A2A agent",
    URL:         "http://localhost:8080",
})
card := cardGen.Generate()

// Serve over HTTP using the official A2A SDK.
handler := a2asrv.NewHandler(executor)
mux := http.NewServeMux()
mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))
mux.Handle("/", a2asrv.NewJSONRPCHandler(handler))
http.ListenAndServe(":8080", mux)
```

See [examples/a2a-bridge/](../../examples/a2a-bridge/) for a complete working example.

## Architecture

```
                         MCP side                          A2A side
                    +----------------+              +------------------+
                    | ToolRegistry   |              | A2A Client       |
                    | (mcpkit)       |              | (remote agent)   |
                    +-------+--------+              +--------+---------+
                            |                                |
                    tool definitions                   A2A Message
                    + handlers                        (JSON-RPC/HTTP)
                            |                                |
                            v                                v
                    +-------+--------+              +--------+---------+
                    | AgentCard      |              | a2asrv.Handler   |
                    | Generator      |              | (A2A Go SDK)     |
                    +-------+--------+              +--------+---------+
                            |                                |
                    AgentCard with                   ExecutorContext
                    skills list                      with Message
                            |                                |
                            v                                v
                    +-------+--------------------------------+---------+
                    |                BridgeExecutor                     |
                    |                                                   |
                    |  1. Extract skill ID + args from A2A Message      |
                    |  2. Look up MCP tool in registry                  |
                    |  3. Apply middleware chain                         |
                    |  4. Execute tool handler                          |
                    |  5. Translate CallToolResult to A2A events        |
                    +--------------------------------------------------+
                                          |
                                    Translator
                                    (zero-cost,
                                     deterministic)
```

## API Reference

### BridgeExecutor

`BridgeExecutor` implements the `a2asrv.AgentExecutor` interface from the official A2A Go SDK. It translates incoming A2A task messages into mcpkit tool calls, routing each request through the registry and middleware chain.

```go
executor := a2a.NewBridgeExecutor(reg, a2a.ExecutorConfig{
    Translator:  &a2a.Translator{SkillTags: []string{"mcpkit"}},
    Logger:      slog.Default(),
    Middleware:  []registry.Middleware{myMiddleware},
    TaskTimeout: 60 * time.Second, // default: 30s
})
```

**ExecutorConfig fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Translator` | `*Translator` | zero-value | Custom translation settings |
| `Logger` | `*slog.Logger` | `slog.Default()` | Structured logger |
| `Middleware` | `[]registry.Middleware` | nil | Bridge-level middleware chain |
| `TaskTimeout` | `time.Duration` | 30s | Max duration per tool execution |

### AgentCardGenerator

Produces A2A `AgentCard` manifests from the mcpkit registry. The card is cached and can be invalidated when tools change.

```go
gen := a2a.NewAgentCardGenerator(reg, translator, a2a.CardConfig{
    Name:        "my-agent",
    Description: "What this agent does",
    Version:     "1.0.0",
    URL:         "http://localhost:8080",
    Provider:    &a2atypes.AgentProvider{Org: "my-org"},
    ToolFilter:  func(name string, td registry.ToolDefinition) bool {
        return !td.IsWrite  // expose read-only tools only
    },
})

card := gen.Card()       // returns cached or generates
gen.Invalidate()         // force regeneration on next Card()
```

### Translator

Deterministic, zero-cost translation between MCP and A2A data types. No LLM involvement.

```go
tr := &a2a.Translator{SkillTags: []string{"mcpkit"}}
```

## Translation Rules

### MCP to A2A (tool registration)

| MCP field | A2A field | Notes |
|-----------|-----------|-------|
| `Tool.Name` | `AgentSkill.ID`, `AgentSkill.Name` | Direct mapping |
| `Tool.Description` | `AgentSkill.Description` | Direct mapping |
| `ToolDefinition.Category` | First tag in `AgentSkill.Tags` | |
| `ToolDefinition.Tags` | Appended to `AgentSkill.Tags` | |
| `ToolDefinition.IsWrite` | `"write"` or `"read"` tag | |
| `Tool.InputSchema` | `AgentSkill.Examples[0]` | JSON-serialized schema |

### MCP to A2A (tool results)

| MCP content type | A2A part type | Notes |
|------------------|---------------|-------|
| `TextContent` | `TextPart` | Direct text mapping |
| `ImageContent` | `RawPart` | Base64-decoded, `mediaType` preserved |
| `EmbeddedResource` | `DataPart` | Serialized as structured data |
| Error result (`IsError: true`) | `TaskStateFailed` | Error text in status message |

### MCP error codes to A2A task states

| MCP error code | A2A task state |
|----------------|----------------|
| `ErrInvalidParam` | `TASK_STATE_FAILED` |
| `ErrNotFound` | `TASK_STATE_FAILED` |
| `ErrPermission` | `TASK_STATE_REJECTED` |
| `ErrTimeout` | `TASK_STATE_FAILED` |
| `ErrRateLimited` | `TASK_STATE_FAILED` |
| `ErrInternal` | `TASK_STATE_FAILED` |

### A2A to MCP (incoming requests)

The translator extracts tool name and arguments from A2A messages using two strategies:

1. **DataPart (preferred):** Looks for `{"skill": "<tool_name>", "arguments": {...}}` in a DataPart.
2. **TextPart fallback:** If a skill hint is provided, parses the first TextPart as JSON arguments, or wraps plain text as `{"input": "<text>"}`.

## Limitations

This is a proof-of-concept bridge (MCP to A2A direction only):

- **MCP to A2A only** -- MCP tools are exposed as A2A skills. The reverse direction (consuming A2A agents as MCP tools) is planned but not yet implemented in this package.
- **No streaming** -- Tool execution is synchronous. A2A streaming responses and SSE subscriptions are not yet supported through the bridge.
- **No auth bridge** -- MCP and A2A have different authentication models. The bridge does not translate between them; configure auth separately on each side.
- **Synchronous execution** -- Each A2A task maps to a single MCP tool call. Multi-step agent workflows are not handled.

## Roadmap

| Feature | Status |
|---------|--------|
| MCP to A2A (tools as skills) | Done |
| Agent card generation from registry | Done |
| Bridge-level middleware support | Done |
| Tool filtering for selective exposure | Done |
| A2A to MCP (agents as tools) | Planned |
| Streaming result forwarding | Planned |
| Auth model bridging (OAuth/JWT) | Planned |
| Multi-step task orchestration | Planned |

## Dependencies

- [a2aproject/a2a-go/v2](https://github.com/a2aproject/a2a-go) -- Official A2A Go SDK for protocol types and server infrastructure
- [mcpkit/registry](../../registry/) -- MCP tool registration and middleware chain
- [mcpkit/handler](../../handler/) -- Error code constants for status translation
