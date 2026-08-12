# Discovery-First MCP Context Efficiency Guidelines

This document establishes guidelines and best practices for building context-efficient Model Context Protocol (MCP) servers using `mcpkit`. Adopting a discovery-first architecture keeps system prompt token overhead low, maximizes prompt caching efficiency, and scales MCP servers to hundreds of tools without bloating LLM context windows.

---

## 1. Executive Summary & Rationale

### The Token Overhead Problem
In traditional MCP server implementations, every registered tool's complete JSON schema is serialized and injected into the LLM's system prompt on every turn. As an MCP server or fleet grows:
- **Context Bloat**: Registering 100+ tools consumes 20,000–50,000+ tokens of prompt space *before* any conversation or code context is added.
- **Cache Invalidations**: Changing a single tool schema invalidates prompt caches for the entire session.
- **Latency & Cost Inflation**: Increased input token volume causes higher Time-To-First-Token (TTFT) and significantly higher API costs per turn.

### The Discovery-First Solution
Discovery-First MCP decouples **tool registration** from **eager schema exposure**:
1. Eagerly expose only a lightweight **"Front Door"** surface (typically 4 meta-tools: `tool_catalog`, `tool_search`, `tool_schema`, `server_health`).
2. Keep domain tools **deferred** in server-side registries.
3. Allow LLM agents to search for tools by intent (`tool_search`), inspect parameters (`tool_schema`), or dynamically load required tool groups (`load_tool_group`) on demand.

**Key Result**: **80–90% reduction in initial system prompt context overhead** and up to 95% prompt caching cost savings across Claude, OpenAI, and Gemini models.

---

## 2. Core Architectural Principles

```
┌────────────────────────────────────────────────────────────────────────┐
│                        Eager Initial Surface                           │
│   (frontdoor: tool_catalog, tool_search, tool_schema, server_health)   │
└────────────────────────────────────────────────────────────────────────┘
                                     │
           1. Query catalog/search   │   2. Inspect schema or load group
           ▼                         ▼
┌────────────────────────┐      ┌────────────────────────┐
│  Catalog Resources     │      │ Deferred Tool Registry │
│  (catalog://tools,     │      │ (Domain Modules: DB,   │
│   catalog://groups)    │      │  K8s, FinOps, Git)     │
└────────────────────────┘      └────────────────────────┘
                                     │
                                     │ 3. Execute target tool
                                     ▼
                        ┌────────────────────────┐
                        │ Output Truncation &    │
                        │ Paging Middleware      │
                        └────────────────────────┘
```

### 1. Eager Surface Minimization
Keep the eagerly exposed tool count below **5 tools** per server or gateway. The default front-door surface should contain only:
- `tool_catalog`: List available tool groups, categories, and summary counts.
- `tool_search`: Keyword/regex search over deferred tool names and descriptions.
- `tool_schema`: Return the full JSON schema for specific deferred tools.
- `server_health`: Diagnostic check for server readiness and upstream dependencies.

### 2. Deferred Group Loading
Organize tools into logical, domain-scoped modules or tool groups (e.g., `git`, `database`, `cloud_aws`, `security`). Register them in `registry.ToolRegistry` as deferred tools so their full schemas remain unloaded until explicitly requested.

### 3. Static Catalog Resources
Expose read-only MCP resources (e.g., `catalog://tools`, `catalog://tool-groups`) for ultra-cheap context reads when clients support resource inspection.


---

## 3. Implementation in `mcpkit`

### Setting Up a Front-Door Starter

`mcpkit/frontdoor` provides a pre-built, production-ready discovery front door:

```go
package main

import (
    "log"

    "github.com/hairglasses-studio/mcpkit/frontdoor"
    "github.com/hairglasses-studio/mcpkit/registry"
)

func main() {
    reg := registry.NewToolRegistry()

    // 1. Register domain modules (deferred internally)
    reg.RegisterModule(&DatabaseModule{})
    reg.RegisterModule(&KubernetesModule{})

    // 2. Attach frontdoor discovery tools
    fd := frontdoor.New(reg,
        frontdoor.WithPrefix("studio_"),
        frontdoor.WithHealthChecker(myHealthCheckFunc),
    )
    fd.Register(reg)

    // 3. Expose eager surface on MCPServer
    s := registry.NewMCPServer("discovery-server", "1.0.0")
    reg.RegisterWithServer(s)

    if err := registry.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

### Result Payload Truncation & Paging

Even with discovery-first tool loading, large tool execution responses (e.g. listing 10,000 log lines or database rows) can exhaust context windows. Use `mcpkit/handler` and `mcpkit/middleware` primitives:

- **Truncation**: `middleware.Truncate(100*1024)` caps output at 100 KB and appends a truncation marker.
- **Cursor Pagination**: `handler.Paginate[T]` provides token-efficient cursor-based result paging.

---

## 4. Context Efficiency Metrics & Targets

| Metric | Eager Monolithic Server | Discovery-First MCP (`mcpkit`) | Savings / Improvement |
|---|---|---|---|
| **Initial Tool Count in System Prompt** | 100+ tools | 4 front-door tools | **96% reduction** in schema count |
| **System Prompt Token Overhead** | 25,000–45,000 tokens | 1,200–2,000 tokens | **80–90% token reduction** |
| **Prompt Caching Hit Rate** | 20–50% (frequent invalidation) | 90–98% (stable system prefix) | **2x–3x cache efficiency** |
| **Time-To-First-Token (TTFT)** | High (large prompt processing) | Sub-second | **50–80% lower latency** |

---

## 5. Migration Checklist for Legacy Servers

1. **Audit Current Surface**: Measure current system prompt token consumption using `mcpkit/toolindex` or log telemetry.
2. **Group Tools by Domain**: Reorganize handwritten tool definitions into `registry.ToolModule` implementations.
3. **Mount `frontdoor.New`**: Replace direct tool exports with `frontdoor` starter tools.
4. **Update Agent Guidance**: Configure agent instructions (`AGENTS.md` / `CLAUDE.md`) to instruct agents to use `tool_search` or `catalog://tool-groups` before invoking deferred tools.
5. **Verify Context Reduction**: Verify with `mcptest` that initial `tools/list` returns only the front door tools while search returns all deferred tools.
