# a2a

A2A (Agent-to-Agent) Protocol v1.0 bridge for mcpkit. This package enables bidirectional communication between MCP (Model Context Protocol) tool servers and A2A agent networks.

## Overview

The A2A protocol ([github.com/a2aproject/A2A](https://github.com/a2aproject/A2A)) enables vendor-agnostic communication between AI agents. This package bridges MCP tool calls with A2A task delegation, allowing mcpkit-based servers to both send tasks to and receive tasks from A2A agents.

For the production bridge built on the official A2A Go SDK, see [`bridge/a2a/`](../bridge/a2a/). This package (`a2a/`) provides the foundational types, client, server, and compatibility layer.

## Quick Start

```go
package main

import (
    "log"
    "net/http"

    "github.com/hairglasses-studio/mcpkit/a2a"
    "github.com/hairglasses-studio/mcpkit/registry"
)

func main() {
    // 1. Create the MCP tool registry.
    reg := registry.NewToolRegistry()
    reg.RegisterModule(&MyModule{})

    // 2. Generate an A2A agent card from registered tools.
    card := a2a.AgentCardFromRegistry(reg,
        a2a.WithName("my-agent"),
        a2a.WithDescription("MCP tools exposed as an A2A agent"),
        a2a.WithURL("http://localhost:8080"),
        a2a.WithStreaming(),
    )

    // 3. Create the A2A server.
    srv := a2a.NewServer(reg, card)

    // 4. Serve.
    log.Println("A2A agent listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", srv.Handler()))
}
```

### Sending Tasks to Remote Agents

```go
client := a2a.NewClient("http://remote-agent:8080",
    a2a.WithAuthToken("${API_KEY}"),
)

// Discover the agent.
card, err := client.GetAgentCard(ctx)

// Send a task and wait for completion.
task, err := client.SendTask(ctx, a2a.TaskSendParams{
    ID:       "task-1",
    Messages: []a2a.Message{
        {Role: "user", Parts: []a2a.Part{a2a.TextPart("summarize this document")}},
    },
})
```

## Architecture

```
+------------------+                              +------------------+
| MCP Tool         |                              | Remote A2A Agent |
| Registry         |                              |                  |
+--------+---------+                              +--------+---------+
         |                                                 |
    tool definitions                                JSON-RPC / HTTP
    + handlers                                      A2A protocol
         |                                                 |
         v                                                 v
+--------+---------+                              +--------+---------+
| AgentCard        |          A2A Bridge          | Client           |
| Generator        |<---------+-------+---------->| (sends tasks)    |
+--------+---------+           |       |          +--------+---------+
         |                     |       |                   |
    AgentCard with        Server    Client          GetAgentCard()
    skills list          (receives) (sends)         SendTask()
         |                tasks     tasks           GetTask()
         v                     |       |            CancelTask()
+--------+---------+           |       |
| Server           |<----------+       |
| (A2A JSON-RPC)   |                   |
+--------+---------+                   |
         |                             |
    tasks/send                    tasks/send
    tasks/get                     tasks/get
    tasks/cancel                  tasks/cancel
         |                             |
         v                             v
+--------+-----------------------------+---------+
|            Interceptors (composable)            |
|                                                 |
|  AuthInterceptor    - per-agent credentials     |
|  RateLimitInterceptor - token-bucket per agent  |
|  TracingClient      - OpenTelemetry spans       |
+-------------------------------------------------+
```

### Package Components

| Component | Description |
|-----------|-------------|
| `AgentCard` / `AgentCardFromRegistry` | Generate A2A agent cards from MCP tool registries |
| `Server` | Accept A2A JSON-RPC tasks and dispatch to MCP tools |
| `Client` | Send tasks to remote A2A agents |
| `AuthInterceptor` | Per-agent credential management (Bearer, API key, OAuth2) |
| `RateLimitInterceptor` | Token-bucket rate limiting per agent URL |
| `TracingClient` | OpenTelemetry distributed tracing across MCP-A2A boundaries |
| `ToSDKAgentCard` / `FromSDKAgentCard` | Compatibility with the official [a2a-go](https://github.com/a2aproject/a2a-go) SDK types |
| `NewBridgeTool` | MCP tool that delegates work to any A2A agent |

## Relationship to bridge/a2a

This package (`a2a/`) provides:
- Core A2A types (`Task`, `Message`, `Part`, `Artifact`, `AgentCard`)
- A self-contained JSON-RPC server and client
- Interceptors (auth, rate limiting, tracing)
- Compatibility layer with the official a2a-go SDK

The [`bridge/a2a/`](../bridge/a2a/) package provides:
- Production bridge built on the official `a2aproject/a2a-go/v2` SDK
- `BridgeExecutor` implementing `a2asrv.AgentExecutor`
- `Translator` for deterministic MCP-to-A2A data type conversion
- `RemoteAgent` wrapping A2A agents as MCP `ToolModule`
- Streaming progress translation (MCP progress notifications to A2A SSE)
- Auth middleware for cross-protocol token propagation

Use `bridge/a2a/` for production deployments. Use `a2a/` when you need lightweight A2A client/server capabilities or SDK type compatibility.

## Official A2A Go SDK

This package integrates with the official A2A Go SDK at [github.com/a2aproject/a2a-go](https://github.com/a2aproject/a2a-go). The `ToSDKAgentCard` and `FromSDKAgentCard` functions convert between mcpkit's native types and the SDK types, enabling interoperability with any A2A-compatible implementation.

## Testing

```bash
go test ./a2a/... -count=1
go test ./a2a/... -count=1 -race
```

## Dependencies

- [a2aproject/a2a-go/v2](https://github.com/a2aproject/a2a-go) -- Official A2A Go SDK
- [mcpkit/registry](../registry/) -- MCP tool registration
- [mcpkit/handler](../handler/) -- Typed handler and error codes
- [google/uuid](https://github.com/google/uuid) -- Task ID generation
- [go.opentelemetry.io/otel](https://opentelemetry.io/) -- Distributed tracing (TracingClient)
