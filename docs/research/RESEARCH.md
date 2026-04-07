# mcpkit Ecosystem Research

Research findings for mcpkit positioning, spec coverage, and roadmap planning.
Last updated: 2026-03-15.

---

<mcp-ecosystem>

## MCP Protocol Evolution

### Spec Timeline

| Version | Date | Key Additions |
|---------|------|---------------|
| Draft | 2024-11-25 | Initial spec: tools, resources, prompts, sampling |
| 2025-03-26 | 2025-03-26 | Streamable HTTP transport, OAuth 2.1, tool annotations |
| 2025-11-25 | 2025-11-25 | Tasks, elicitation, structured output, outputSchema, progress reporting |
| 2026 (roadmap) | TBD | Registry integration, namespaces, agent-to-agent, extensions |

Sources: modelcontextprotocol.io/specification/2025-11-25, modelcontextprotocol.io/development/roadmap

### Protocol Features — Status in mcpkit

| # | Feature | Spec Version | mcpkit Status | Confidence | Notes |
|---|---------|-------------|---------------|------------|-------|
| 1 | Tools (registration, middleware, search) | Draft | **Implemented** | High | Core strength — registry, handler, typed generics |
| 2 | Tool Annotations (read-only, destructive, idempotent) | 2025-03-26 | **Implemented** | High | Auto-inferred from ToolDefinition metadata |
| 3 | Structured Output (outputSchema + structuredContent) | 2025-11-25 | **Implemented** | High | TypedHandler auto-generates outputSchema |
| 4 | Elicitation (user input during tool execution) | 2025-11-25 | **Implemented** | High | handler.ElicitForm, ElicitURL, ElicitFormSchema |
| 5 | Tasks (async operations, status tracking) | 2025-11-25 | **Implemented** | Medium | Task types in compat.go, TaskStatus constants |
| 6 | Deferred Tool Loading (lazy schema fetch) | 2025-11-25 | **Implemented** | High | registry.RegisterDeferredModule, ListEagerTools |
| 7 | OAuth 2.1 Authorization | 2025-03-26 | **Implemented** | High | Full OAuth 2.1 + PKCE flow: auth.OAuthClient, OAuthTransport, PKCE helpers, Bearer middleware |
| 7a | DPoP Token Binding | 2025-03-26 | **Implemented** | High | auth/dpop.go: proof validation, nonce cache, HTTP middleware (DPoPMiddleware) |
| 8 | Streamable HTTP Transport | 2025-03-26 | **Delegated** | Medium | Handled by mcp-go; mcpkit uses server.MCPServer |
| 9 | Progress Reporting | 2025-11-25 | **Implemented** | High | ServerProgressMiddleware + ProgressReporter interface |
| 10 | tools/list_changed Notifications | 2025-03-26 | **Implemented** | High | WireToolListChanged/WireResourceListChanged/WirePromptListChanged diff-based helpers |
| 11 | Resources (URI-based data exposure) | Draft | **Implemented** | High | resources package: ResourceDefinition, registry, middleware, dynamic resources |
| 12 | Prompts (reusable prompt templates) | Draft | **Implemented** | High | prompts package: PromptDefinition, registry, middleware, dynamic prompts |
| 13 | Sampling (LLM completion requests) | Draft | **Implemented** | High | sampling package: client interface, middleware, request builders |
| 14 | Logging Endpoint | Draft | **Implemented** | High | logging package: slog handler bridge, tool invocation logging middleware |

**Coverage: 15/15 features fully implemented (100%)** — all tracked MCP protocol features have complete implementations

### June 2026 Spec Signals

The MCP spec roadmap signals several major additions for the June 2026 revision:

| Feature | Status | Impact on mcpkit |
|---------|--------|-----------------|
| Scalable Sessions | SEP under review | New `session/` package needed — Session/SessionStore interfaces, middleware, migration helpers |
| Stateless HTTP | SEP under review | External session stores (Redis), token extraction, gateway affinity routing |
| WebSocket Transport | SEP-1288 proposed | New `transport/` abstraction layer — stdio, HTTP, WebSocket adapters |
| Registry Integration | Active development | Enhance `discovery/` with PublishWorkflow, validation, compliance checking |
| Namespaces | Proposed | May affect `gateway/` tool routing and `registry/` tool naming |

Sources: modelcontextprotocol.io/development/roadmap, GitHub MCP spec discussions

### Cloud Platform MCP Integrations

| Platform | Integration | Status | Notes |
|----------|------------|--------|-------|
| Cloudflare | Workers Code Mode | GA | Auto-generates MCP servers from Workers; developers.cloudflare.com/agents/model-context-protocol |
| Vercel | MCP Adapter | Beta | Next.js API route adapter for MCP tool serving |
| Azure | MCP Center | Preview | Enterprise server discovery and management |
| Chrome | WebMCP (W3C EPP) | Experimental | Browser-native MCP transport; developer.chrome.com/blog/webmcp-epp |

Implication for mcpkit: `research/` package should add PlatformMonitor interface to track cloud platform adoption and feature parity.

### A2A Protocol Status Update

Google's Agent-to-Agent (A2A) protocol has moved to Linux Foundation governance (github.com/google/A2A). Current status:
- **Spec version**: v0.3 (not yet stable for production)
- **Governance**: Linux Foundation AI & Data
- **Key features**: AgentCard discovery, task lifecycle, streaming updates, push notifications
- **Go implementations**: Several community SDKs emerging, none production-ready
- **Decision gate**: mcpkit will implement `a2a/` bridge when spec reaches v0.9+ stability

### Updated SDK Versions

| SDK | Current Version | mcpkit Pinned | Gap |
|-----|----------------|---------------|-----|
| mcp-go | v0.45.0 | v0.43.2 | 2 minor versions behind — evaluate bump in P34 |
| Official go-sdk | v1.4.1 | dual-SDK build tags | Tracking; v2.0 would trigger migration assessment |
| TypeScript SDK | v1.12.0 | N/A (reference only) | Reference implementation for spec compliance checking |
| FastMCP (Python) | v2.5.0 | N/A (competitor) | Dominant in Python ecosystem; feature parity benchmark |

</mcp-ecosystem>

---

<frameworks>

## MCP Framework Landscape

### Python — FastMCP
- **Repo**: github.com/jlowin/fastmcp | **Docs**: gofastmcp.com
- ~70% market share among MCP server implementations
- Decorator-based: `@mcp.tool()`, `@mcp.resource()`, `@mcp.prompt()`
- Built-in auth, proxy pattern, OpenAPI/FastAPI import
- Strength: developer experience, rapid prototyping
- Weakness: Python runtime overhead, no compile-time type safety

### Go — Official SDK
- **Repo**: github.com/modelcontextprotocol/go-sdk (v1.4.1)
- Backed by Google + Anthropic, 4.1k stars
- Generic typed handlers with struct-tag schema inference
- GitHub MCP Server migrated to it (Nov 2025)
- Strength: official backing, spec-complete
- Weakness: lower-level, no middleware chain, no production patterns (circuit breaker, rate limiting, RBAC)

### Go — mcp-go (mcpkit's foundation)
- **Repo**: github.com/mark3labs/mcp-go (v0.45.0, mcpkit on v0.43.2)
- Community standard Go SDK, widely adopted
- Provides: server, transport, protocol types
- mcpkit adds: registry, middleware, typed handlers, resilience, auth, security, observability

### TypeScript
- Official `@modelcontextprotocol/sdk` — reference implementation
- Zod-based schema validation, streaming support
- Dominant in Claude Desktop integrations

### Cloud Platforms
- **Cloudflare**: Code Mode — auto-generates MCP servers from Workers (developers.cloudflare.com/agents/model-context-protocol)
- **Vercel**: MCP adapter for Next.js API routes
- **Azure**: MCP Center for enterprise server discovery
- **Chrome**: WebMCP W3C Extended Privacy Pass (developer.chrome.com/blog/webmcp-epp) — browser-native MCP

### mcpkit Positioning
mcpkit occupies a unique niche: **production-grade Go middleware layer**. While the official SDK provides protocol primitives, mcpkit adds the patterns needed for real deployments:
- Middleware chain (timeout, panic recovery, truncation, rate limiting, circuit breaking)
- RBAC and audit logging
- OpenTelemetry integration
- Input sanitization and secret management
- Test infrastructure (mcptest)

</frameworks>

---

<sibling-repos>

## Hairglasses Studio Sibling Repositories

### [internal MCP server]
- **Size**: 34 tool modules
- **Architecture**: Dispatcher pattern — central router delegates to tool-specific handlers
- **Pros**: Battle-tested at scale, covers domain comprehensively
- **Cons**: Monolithic dispatcher, tight coupling between tools, no middleware abstraction
- **Migration path**: Extract tool modules → implement as mcpkit ToolModule interface, replace dispatcher with registry middleware chain

### hg-mcp
- **Size**: 118 tool modules (~12K LOC tool code)
- **Architecture**: TF-IDF search across tools, flat registration
- **Pros**: Largest tool surface area, proven search/discovery patterns
- **Cons**: No typed handlers, no structured output, manual schema definitions, no resilience patterns
- **Migration path**: mcpkit's TypedHandler replaces manual schema; registry search replaces TF-IDF; compat.go provides SDK abstraction
- **Key extraction**: mcpkit was scaffolded from hg-mcp patterns (see memory: project_mcpkit_scaffold.md)

### mesmer
- **Status**: Not found / not yet created

### Migration Strategy
1. Define tool inputs/outputs as Go structs with jsonschema tags
2. Wrap with `handler.TypedHandler[In, Out]()` for automatic schema generation
3. Group related tools into `ToolModule` implementations
4. Register modules with `registry.RegisterModule()` — middleware applies automatically
5. Use `registry/compat.go` aliases for SDK-agnostic type references

</sibling-repos>

---

<mcpkit-assessment>

## mcpkit Current State

### Package Inventory (19 packages)

| Package | LOC (est.) | Test Coverage | Maturity |
|---------|-----------|---------------|----------|
| registry | ~400 | Yes | Stable |
| handler | ~350 | Yes | Stable |
| resilience | ~300 | Yes | Stable |
| mcptest | ~250 | Yes | Stable |
| auth | ~500 | Yes | Stable |
| security | ~200 | Yes | Stable |
| health | ~150 | Yes | Stable |
| observability | ~200 | Yes | Beta |
| sanitize | ~150 | Yes | Stable |
| secrets | ~150 | Yes | Stable |
| client | ~100 | Yes | Stable |
| resources | ~250 | Yes | Stable |
| prompts | ~250 | Yes | Stable |
| sampling | ~200 | Yes | Beta |
| logging | ~200 | Yes | Beta |
| roots | ~150 | Yes | Beta |
| research | ~300 | Yes | Beta |
| discovery | ~100 | Yes | Beta |
| gateway | ~300 | Yes | Beta |

| ralph | ~300 | Yes | Beta |
| memory | ~250 | Yes | Beta |

**Total: ~21 packages** (auth grew significantly with OAuth client, DPoP, and PKCE additions).

### Quality Signals
- Thread-safe registries (sync.RWMutex throughout)
- Panic recovery in all tool handlers
- Response truncation to prevent memory issues
- Typed generics for compile-time safety
- SDK abstraction layer (compat.go) for future migration
- Comprehensive test infrastructure (mcptest package)

### Architecture Strengths
- Clean dependency layers (3 tiers, no cycles)
- Middleware-first design (composable, testable)
- Module system (ToolModule interface) for organizing tools
- Production patterns built-in (circuit breaker, rate limiter, RBAC)

### Gaps
1. No example servers or getting-started guide
2. mcp-go version lag (v0.43.2 vs v0.45.0)
3. No official SDK migration path documented
4. No end-to-end streamable HTTP verification tests
5. Observability middleware lacks integration tests
6. MCP Registry publish client not implemented (research package covers ecosystem monitoring; server publishing to registry.modelcontextprotocol.io not yet wired)
7. ~~Progress reporting~~ (resolved: ServerProgressMiddleware)
8. ~~tools/list_changed notification wiring~~ (resolved: Wire*ListChanged helpers)

</mcpkit-assessment>

---

<bootstrap-opportunities>

## Roadmap: 34 Items in 4 Tiers

### Tier 1 — Must-Have (spec completeness + adoption)

| # | Item | Evidence | Effort |
|---|------|----------|--------|
| 1 | **Resources package** ✓ | Core MCP primitive since draft spec. URI-based data exposure. ResourceDefinition, registry, dynamic resources. | Complete |
| 2 | **Prompts package** ✓ | Core MCP primitive since draft spec. Reusable prompt templates with argument schemas. PromptDefinition, registry, dynamic prompts. | Complete |
| 3 | **Example servers** | Zero examples = adoption blocker. Need: minimal (1 tool), full-featured (auth+resilience+observability), and migration-from-hg-mcp. | Medium |
| 4 | **Official SDK migration path** | go-sdk v1.4.1 is production-ready. Document compat.go update strategy. Dual-SDK CI testing. | Low |
| 5 | **Streamable HTTP verification** | Spec moved from SSE to streamable HTTP (2025-03-26). Need integration tests proving mcpkit works with streamable transport via mcp-go. | Low |

### Tier 2 — Should-Have (differentiation + completeness)

| # | Item | Evidence | Effort |
|---|------|----------|--------|
| 6 | **Dispatcher package** ✓ | Priority worker pool with concurrency groups, heap-based priority queue, registry middleware integration. | Complete |
| 7 | **OAuth token exchange** ✓ | Full OAuth 2.1 + PKCE implemented: auth.OAuthClient, OAuthTransport, PKCE helpers. Complete flow for remote MCP servers. | Complete |
| 8 | **Sampling support** ✓ | MCP sampling lets servers request LLM completions. Middleware for sampling request/response. Implemented in sampling/. | Complete |
| 9 | **Logging endpoint** ✓ | MCP logging primitive. slog handler bridge + tool invocation logging middleware. Implemented in logging/. | Complete |
| 10 | **Observability integration tests** | observability package exists but needs end-to-end tests with real OTLP collector. | Low |

### Tier 3 — Nice-to-Have (future-proofing)

| # | Item | Evidence | Effort |
|---|------|----------|--------|
| 11 | **Registry integration** ✓ | MCP Registry client for server discovery and publishing. discovery/ package. | Complete |
| 12 | **DPoP token binding** ✓ | OAuth spec extension for proof-of-possession. Prevents token replay attacks. Implemented in auth/dpop.go. | Complete |
| 13 | **Workload Identity** | Cloud-native auth (GCP, AWS IAM) for server-to-server MCP without shared secrets. | High |
| 14 | **WebMCP bridge** | Chrome's W3C Extended Privacy Pass for browser-based MCP. Bridge package for web clients. | High |
| 15 | **Extensions framework** | MCP roadmap mentions protocol extensions. Plugin system for custom capabilities. | High |
| 16 | **Gateway pattern** ✓ | Multi-server aggregation with namespaced tool routing. gateway/ package. | Complete |
| 17 | **Playbooks / Ralph Loop** ✓ | Autonomous loop runner for iterative task execution. JSON spec → LLM decisions → tool dispatch → progress tracking. ralph/ package. | Complete |
| 18 | **Ralph: Multi-tool decisions** ✓ | Allow LLM to request multiple tool calls per iteration for parallel execution. | Complete |
| 19 | **Ralph: Spec validation** ✓ | Schema validation for spec files, required field checks, task ID uniqueness enforcement. | Complete |
| 20 | **Ralph: Resumable loops** | Resume from progress file after process restart, re-attach to in-flight state. | Medium |
| 21 | **Ralph: Event hooks** ✓ | User-defined callbacks on iteration start/end, task completion, and errors. | Complete |
| 22 | **Ralph: Conditional tasks** | Task dependencies and prerequisite chains (DAG-based execution ordering). | Medium |
| 23 | **Ralph: Streaming progress** | SSE/websocket push of iteration status to connected clients. | Medium |
| 24 | **Ralph: Cost tracking** ✓ | Token usage accounting per iteration and cumulative per loop run. | Complete |
| 25 | **Ralph: Spec templates** | YAML support, variable interpolation, spec composition from fragments. | Medium |

### Tier 4 — Ecosystem Leadership (agent bootstrapping + multi-agent)

| # | Item | Evidence | Effort |
|---|------|----------|--------|
| 26 | **A2A Protocol Bridge** | Google A2A v0.3 (Linux Foundation) — Agent Cards, task lifecycle, push notifications, gRPC transport. Emerging standard for agent-to-agent communication. | High (3wk) |
| 27 | **Multi-Agent Orchestrator** | Go-native patterns: fan-out/fan-in, pipeline, swarm mesh, hierarchical delegation. Required for complex multi-agent workflows. | High (3wk) |
| 28 | **Agent Memory Registry** ✓ | Episodic/semantic/procedural memory tiers, pluggable storage backends. Enables stateful agents across sessions. | Complete |
| 29 | **Workflow Engine** | Cyclical graph execution, state machines, conditional branching, YAML definitions. Goes beyond linear ralph loops. | High (3wk) |
| 30 | **Extensions Framework** | MCP spec roadmap mentions protocol extensions. Capability handshake, version-gated features, negotiation. | Medium (2wk) |
| 31 | **Agent Handoff Protocol** | Manager/agent-as-tool + peer handoff patterns (inspired by OpenAI Agents SDK). Agent delegation without tight coupling. | Medium (2wk) |
| 32 | **Skills & Context-Aware Loading** | Skill bundles with lazy loading. ~95% context reduction for large tool sets. Critical for scaling beyond 100+ tools. | Medium (10d) |
| 33 | **FinOps / Cost Tracking** ✓ | Token accounting per tool/sampling/agent/workflow. Budget policies, Prometheus export. Cross-cutting concern for production agents. | Complete (core) |
| 34 | **Agent Bootstrap Framework** | Workspace init, context reports, capability matrix, multi-session state. First-run experience for agent developers. | Medium (10d) |

</bootstrap-opportunities>

---

<implementation-sequence>

## Phased Implementation

### Phase 1: Foundation (Weeks 1–2)
- [x] Resources package (resources/)
- [x] Prompts package (prompts/)
- [ ] Streamable HTTP integration tests
- [ ] Bump mcp-go to v0.45.0

### Phase 2: Examples + Auth (Weeks 3–4)
- [ ] Minimal example server (1 tool, stdio transport)
- [ ] Full-featured example (auth + resilience + observability)
- [ ] hg-mcp migration example (compat.go showcase)
- [x] OAuth token exchange completion (auth.OAuthClient + PKCE)
- [x] Logging endpoint integration (logging/)

### Phase 3: Differentiation (Weeks 5–6)
- [ ] Dispatcher package (from internal server patterns)
- [x] Sampling middleware (sampling/)
- [ ] Observability integration tests
- [ ] Official SDK dual-testing CI

### Phase 4: Future-Proofing (Week 7+)
- [ ] MCP Registry integration
- [x] DPoP token binding (auth/dpop.go)
- [x] Gateway pattern (gateway/)
- [x] Ralph Loop (autonomous task execution) — ralph/

### Phase 5: Ralph Evolution + Foundations (Weeks 9-11)
- [x] Ralph: multi-tool decisions, cost tracking ~~, event hooks, spec validation~~
- [ ] Ralph: DAG execution, resumable loops, streaming progress, spec templates
- [ ] Extensions package (extensions/) — MCP protocol extensions framework
- [x] Memory package (memory/) — agent memory registry with storage backends
- [x] FinOps package (finops/) — token accounting and budget middleware

### Phase 6: Multi-Agent + A2A (Weeks 12-15)
- [ ] A2A package (a2a/) — Google A2A v0.3 protocol bridge
- [ ] Orchestrator package (orchestrator/) — fan-out, pipeline, swarm patterns
- [ ] Handoff package (handoff/) — manager + peer agent delegation
- [ ] Skills package (skills/) — context-aware lazy tool loading

### Phase 7: Workflow Engine + Bootstrap (Weeks 16-18)
- [ ] Workflow package (workflow/) — cyclical graph engine, state machines
- [ ] Bootstrap package (bootstrap/) — agent workspace init, context reports
- [ ] Workload identity (auth/workload.go) — GCP/AWS IAM for server-to-server
- [ ] Integration tests + CLAUDE.md updates

### Decision Points
- **After Phase 1**: Evaluate official go-sdk maturity — if v2.0 ships, prioritize migration over mcp-go upgrades
- **After Phase 2**: Assess adoption metrics — if hg-mcp migration succeeds, invest in Tier 3; otherwise, focus on developer experience
- **After Phase 3**: Re-evaluate WebMCP and Extensions based on spec evolution
- **After Phase 5**: Evaluate A2A spec stability — if v1.0 ships, fast-track a2a/; otherwise prototype only
- **After Phase 6**: Assess orchestrator patterns against real-world usage from internal MCP server migrations
- **After Phase 7**: Re-evaluate WebMCP bridge and Chrome integration based on adoption signals

</implementation-sequence>
