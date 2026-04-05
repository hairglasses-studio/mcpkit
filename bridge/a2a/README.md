# bridge/a2a

The first production-quality Go library bridging the [MCP](https://modelcontextprotocol.io/) (Model Context Protocol) and [A2A](https://google.github.io/A2A/) (Agent-to-Agent) protocols. Bidirectional.

This package enables mcpkit-based MCP servers to participate in A2A agent networks, and A2A agents to be consumed as MCP tools. MCP tools are exposed as A2A skills via the `Bridge` and `BridgeExecutor`. Remote A2A agents are wrapped as MCP tool modules via `RemoteAgent`.

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

The bridge is bidirectional. The MCP-to-A2A direction exposes your tools as A2A skills. The A2A-to-MCP direction wraps remote A2A agents as MCP tools.

### Bidirectional Bridge Diagram

```
  MCP-to-A2A (expose tools as skills)         A2A-to-MCP (consume agents as tools)
  ====================================         =====================================

  +----------------+                           +------------------+
  | ToolRegistry   |                           | Remote A2A Agent |
  | (mcpkit)       |                           | (any A2A server) |
  +-------+--------+                           +--------+---------+
          |                                             |
  tool definitions                             AgentCard (HTTP discovery)
  + handlers                                   + skills list
          |                                             |
          v                                             v
  +-------+--------+                           +--------+---------+
  | AgentCard      |                           | RemoteAgent      |
  | Generator      |                           | (a2aclient)      |
  +-------+--------+                           +--------+---------+
          |                                             |
  AgentCard with                               MCP ToolDefinition[]
  skills list                                  (one per A2A skill)
          |                                             |
          v                                             v
  +-------+--------+                           +--------+---------+
  | BridgeExecutor |                           | ToolRegistry     |
  | (a2asrv iface) |                           | .RegisterModule  |
  +-------+--------+                           +--------+---------+
          |                                             |
  A2A Message in ->                            MCP tool call ->
  MCP tool call ->                             A2A sendMessage ->
  CallToolResult ->                            Task result ->
  A2A events out                               CallToolResult
          |                                             |
    Translator                                   Translator
    (zero-cost,                                  (zero-cost,
     deterministic)                               deterministic)
```

### MCP-to-A2A Detail

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

## RemoteAgent (A2A-to-MCP)

`RemoteAgent` wraps a remote A2A agent as an mcpkit `ToolModule`. It discovers the agent card, creates an A2A client, and generates one MCP tool per agent skill. When an MCP client calls the tool, `RemoteAgent` sends an `a2a.sendMessage` to the remote agent and translates the response back to an MCP `CallToolResult`.

### Quick Start

```go
// Discover a remote A2A agent and register its skills as MCP tools.
agent, err := a2a.NewRemoteAgent(ctx, "http://research-agent:8080",
    a2a.WithRemotePrefix("research"),    // tools: research_summarize, research_search, etc.
    a2a.WithRemoteTimeout(60*time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer agent.Close()

// Register as a module — each A2A skill becomes an MCP tool.
reg := registry.NewToolRegistry()
reg.RegisterModule(agent)
```

### Options

| Option | Description |
|--------|-------------|
| `WithRemotePrefix(s)` | Prefix prepended to each skill ID for the MCP tool name. `"research"` + skill `"summarize"` = tool `"research_summarize"`. |
| `WithRemoteTimeout(d)` | Maximum duration for a single A2A call. Default: 60s. |
| `WithRemoteTranslator(t)` | Override the default Translator for custom tag/skill mapping. |
| `WithRemoteFactoryOptions(...)` | Pass additional `a2aclient.FactoryOption` values to the underlying A2A client. |

### From Pre-resolved Card

If you already have the agent card (from a registry, config file, etc.), skip HTTP discovery:

```go
agent, err := a2a.NewRemoteAgentFromCard(ctx, card,
    a2a.WithRemotePrefix("research"),
)
```

### How It Works

1. `NewRemoteAgent` fetches `/.well-known/agent-card.json` from the target URL
2. Each `AgentSkill` in the card becomes an MCP `ToolDefinition` with a `"message"` string parameter
3. When the tool is called, the handler builds an A2A message:
   - If the `message` argument is valid JSON, it is sent as a `DataPart` with `{"skill": "...", "arguments": {...}}`
   - Otherwise, it is sent as a `TextPart`
4. The A2A `Task` response is translated: `COMPLETED` artifacts become text results; `FAILED`/`REJECTED` become error results

## Full API Reference

### Bridge (high-level wiring)

`Bridge` is the all-in-one type that wires a `ToolRegistry` into a running A2A HTTP server. It creates the `Translator`, `AgentCardGenerator`, and `BridgeExecutor` internally.

```go
bridge, err := a2a.NewBridge(reg, a2a.BridgeConfig{
    Name:        "my-agent",
    Description: "MCP tools as an A2A agent",
    URL:         "http://localhost:8080",
    Addr:        ":8080",           // listen address (default: ":8080")
    Timeout:     60 * time.Second,  // per-tool timeout (default: 30s)
    Logger:      slog.Default(),
    Middleware:   []registry.Middleware{myMiddleware},
})

// Option A: Run the built-in HTTP server.
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
go bridge.Start(ctx)

// Option B: Embed in your own server.
http.Handle("/", bridge.Handler())

// Read the generated agent card.
card := bridge.AgentCard()
```

**BridgeConfig fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Name` | `string` | `""` | Human-readable agent name |
| `Description` | `string` | `""` | Agent purpose |
| `Version` | `string` | `"1.0.0"` | Semantic version |
| `URL` | `string` | `""` | Base URL for the agent card |
| `Addr` | `string` | `":8080"` | HTTP listen address |
| `Timeout` | `time.Duration` | 30s | Max tool execution duration |
| `Logger` | `*slog.Logger` | `slog.Default()` | Structured logger |
| `Middleware` | `[]registry.Middleware` | nil | Bridge-level middleware chain |

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

**Execution lifecycle:**

1. Emit `SubmittedTask` if this is a new task
2. Extract skill ID and arguments from the A2A message (via Translator)
3. Look up the MCP tool in the registry
4. Emit `WORKING` status
5. Build `CallToolRequest` and apply middleware chain
6. Execute with timeout
7. Emit artifact + `COMPLETED` status (or `FAILED` on error)

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

Deterministic, zero-cost translation between MCP and A2A data types. No LLM involvement. The zero value is ready to use.

```go
tr := &a2a.Translator{SkillTags: []string{"mcpkit"}}
```

**Methods:**

| Method | Description |
|--------|-------------|
| `ToolToSkill(td)` | Convert MCP `ToolDefinition` to A2A `AgentSkill` |
| `CallResultToArtifact(result)` | Convert MCP `CallToolResult` to A2A `Artifact` |
| `ErrorToTaskStatus(code, msg)` | Convert MCP error code to A2A `TaskStatus` |
| `MessageToCallToolRequest(msg, skill)` | Extract tool name and args from A2A `Message` |
| `CallResultToEvents(taskInfo, result, err)` | Convert tool result to A2A event iterator |

### RemoteAgent

Wraps a remote A2A agent as an mcpkit `ToolModule`. Implements `registry.ToolModule` with `Name()`, `Description()`, and `Tools()`.

```go
agent, err := a2a.NewRemoteAgent(ctx, "http://agent:8080",
    a2a.WithRemotePrefix("agent"),
    a2a.WithRemoteTimeout(60 * time.Second),
)
// Or from a pre-resolved card:
agent, err := a2a.NewRemoteAgentFromCard(ctx, card,
    a2a.WithRemotePrefix("agent"),
)

// Use as a module.
reg.RegisterModule(agent)

// Clean up when done.
defer agent.Close()
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

### A2A to MCP (RemoteAgent responses)

| A2A state | MCP result |
|-----------|------------|
| `COMPLETED` with artifacts | Text from artifact parts joined with newlines |
| `FAILED` / `REJECTED` | `IsError: true` with status message text |
| Task with no artifacts | `"(no output)"` |

## Limitations

- **No streaming** -- Tool execution is synchronous. A2A streaming responses and SSE subscriptions are not yet supported through the bridge.
- **No auth bridge** -- MCP and A2A have different authentication models. The bridge does not translate between them; configure auth separately on each side.
- **Synchronous execution** -- Each A2A task maps to a single MCP tool call. Multi-step agent workflows are not handled.
- **RemoteAgent input** -- Remote tools accept a single `"message"` string parameter. Structured multi-field input requires JSON encoding in the message.

## Roadmap

| Feature | Status |
|---------|--------|
| MCP to A2A (tools as skills) | Done |
| A2A to MCP (agents as tools) | Done |
| Agent card generation from registry | Done |
| Bridge-level middleware support | Done |
| Tool filtering for selective exposure | Done |
| Bidirectional bridge | Done |
| Streaming result forwarding | Planned |
| Auth model bridging (OAuth/JWT) | Planned |
| Multi-step task orchestration | Planned |
| Structured input passthrough for RemoteAgent | Planned |

## Dependencies

- [a2aproject/a2a-go/v2](https://github.com/a2aproject/a2a-go) -- Official A2A Go SDK for protocol types, server infrastructure, and client
- [mcpkit/registry](../../registry/) -- MCP tool registration and middleware chain
- [mcpkit/handler](../../handler/) -- Error code constants for status translation
