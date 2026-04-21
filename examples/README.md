# mcpkit Examples

Runnable MCP servers demonstrating mcpkit packages and patterns. Every example is `go run`-able from the repo root.

## Basics

| Example | Summary |
|---|---|
| [minimal](minimal/) | Simplest possible server: two TypedHandler tools over stdio. Start here. |
| [elicitation](elicitation/) | Request additional input from the client mid-call via `ElicitForm`, `ElicitURL`, and `ElicitFormSchema`. |

## Discovery + catalog

| Example | Summary |
|---|---|
| [frontdoor](frontdoor/) | Mount `frontdoor.New(reg, WithPrefix, WithHealthChecker)` to expose `tool_catalog`, `tool_search`, `tool_schema`, and `server_health` on any existing registry. |
| [pagination](pagination/) | Token-efficient patterns: `handler.Paginate[T]` cursor paging, `handler.TruncateResult` byte budgets, and `handler.SchemaFirstResult` deferred data. |
| [truncate-demo](truncate-demo/) | Truncation middleware caps oversized responses at a configurable byte ceiling and appends a marker for the client. |

## Safety + lifecycle

| Example | Summary |
|---|---|
| [bounded-write](bounded-write/) | Stripe-style confirmation gate: tools tagged `boundedwrite.ConfirmTag` require `confirm=true` before execution. |
| [vuln-scanner](vuln-scanner/) | `vuln_scan` (govulncheck wrapper) and `vuln_osv_query` demonstrate supply-chain scanning as an MCP server. |
| [full](full/) | Production-grade stack: lifecycle, observability, finops, truncate, sanitize, security, resilience middleware chained in one server. |

## Transport

| Example | Summary |
|---|---|
| [http](http/) | StreamableHTTP transport on `:8080/mcp` with `.well-known/mcp.json` server card. |
| [stateless-http](stateless-http/) | Horizontally scalable HTTP server with Redis-backed sessions, multi-source session extraction, and health checks. |
| [gateway](gateway/) | Aggregate tools from multiple upstream MCP servers behind namespaced routing with per-upstream resilience. |

## Agent protocols

| Example | Summary |
|---|---|
| [a2a-bridge](a2a-bridge/) | Expose mcpkit tools as an A2A (Agent-to-Agent) agent over HTTP, with skill discovery via agent card. |
| [rdcycle](rdcycle/) | R&D cycle workflow: research + roadmap + rdcycle modules wired through ralph's `WorkflowLoop`. |

## Running an example

```bash
go run ./examples/minimal
```

stdio examples communicate over stdin/stdout — pair them with an MCP client (Claude Desktop, mcpkit's `mcptest`, or any inspector).

HTTP examples bind a port printed on startup. Hit `/.well-known/mcp.json` for the server card.

## Adding a new example

1. Create `examples/<name>/main.go` with a `// Command <name> ...` package comment describing the what+why in 3-5 lines.
2. Keep each example self-contained in a single `main.go` unless it needs more.
3. Add a row to the appropriate section above.
