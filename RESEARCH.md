# mcpkit Ecosystem Research

Research findings for mcpkit positioning, spec coverage, and roadmap planning.
Last updated: 2026-03-13.

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
| 7 | OAuth 2.1 Authorization | 2025-03-26 | **Partial** | Medium | auth.OAuthMetadata exists, missing token exchange flow |
| 8 | Streamable HTTP Transport | 2025-03-26 | **Delegated** | Medium | Handled by mcp-go; mcpkit uses server.MCPServer |
| 9 | Progress Reporting | 2025-11-25 | **Partial** | Low | No dedicated progress middleware yet |
| 10 | tools/list_changed Notifications | 2025-03-26 | **Partial** | Medium | Dynamic tools exist, notification not wired |
| 11 | Resources (URI-based data exposure) | Draft | **Not implemented** | — | Planned: Tier 1 priority |
| 12 | Prompts (reusable prompt templates) | Draft | **Not implemented** | — | Planned: Tier 1 priority |
| 13 | Sampling (LLM completion requests) | Draft | **Not implemented** | — | Planned: Tier 2 |
| 14 | Logging Endpoint | Draft | **Not implemented** | — | Planned: Tier 2 |

**Coverage: 10/14 features implemented or partially implemented (~72%)**

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

### jobb
- **Size**: 34 tool modules
- **Architecture**: Dispatcher pattern — central router delegates to tool-specific handlers
- **Pros**: Battle-tested at scale, covers job search/tracking domain comprehensively
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

### Package Inventory (11 packages)

| Package | LOC (est.) | Test Coverage | Maturity |
|---------|-----------|---------------|----------|
| registry | ~400 | Yes | Stable |
| handler | ~350 | Yes | Stable |
| resilience | ~300 | Yes | Stable |
| mcptest | ~250 | Yes | Stable |
| auth | ~200 | Yes | Stable |
| security | ~200 | Yes | Stable |
| health | ~150 | Yes | Stable |
| observability | ~200 | Yes | Beta |
| sanitize | ~150 | Yes | Stable |
| secrets | ~150 | Yes | Stable |
| client | ~100 | Yes | Stable |

**Total: ~8,300 LOC** (8,262 measured), 11 packages, all with tests.

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
1. No Resources or Prompts support (core MCP primitives)
2. No example servers or getting-started guide
3. mcp-go version lag (v0.43.2 vs v0.45.0)
4. No official SDK migration path documented
5. No end-to-end streamable HTTP verification tests
6. Observability middleware lacks integration tests
7. OAuth flow incomplete (metadata only, no token exchange)

</mcpkit-assessment>

---

<bootstrap-opportunities>

## Roadmap: 17 Items in 3 Tiers

### Tier 1 — Must-Have (spec completeness + adoption)

| # | Item | Evidence | Effort |
|---|------|----------|--------|
| 1 | **Resources package** | Core MCP primitive since draft spec. URI-based data exposure. Mirror registry.ToolDefinition pattern for ResourceDefinition. | Medium |
| 2 | **Prompts package** | Core MCP primitive since draft spec. Reusable prompt templates with argument schemas. Mirror ToolModule pattern. | Medium |
| 3 | **Example servers** | Zero examples = adoption blocker. Need: minimal (1 tool), full-featured (auth+resilience+observability), and migration-from-hg-mcp. | Medium |
| 4 | **Official SDK migration path** | go-sdk v1.4.1 is production-ready. Document compat.go update strategy. Dual-SDK CI testing. | Low |
| 5 | **Streamable HTTP verification** | Spec moved from SSE to streamable HTTP (2025-03-26). Need integration tests proving mcpkit works with streamable transport via mcp-go. | Low |

### Tier 2 — Should-Have (differentiation + completeness)

| # | Item | Evidence | Effort |
|---|------|----------|--------|
| 6 | **Dispatcher package** | jobb's dispatcher pattern is proven. Generic task routing with priority queues, concurrency limits. | Medium |
| 7 | **OAuth token exchange** | auth package has metadata but no token flow. Complete OAuth 2.1 with PKCE for remote MCP servers. | Medium |
| 8 | **Sampling support** | MCP sampling lets servers request LLM completions. Middleware for sampling request/response. | Medium |
| 9 | **Logging endpoint** | MCP logging primitive. Integrate with slog, add structured log forwarding to client. | Low |
| 10 | **Observability integration tests** | observability package exists but needs end-to-end tests with real OTLP collector. | Low |

### Tier 3 — Nice-to-Have (future-proofing)

| # | Item | Evidence | Effort |
|---|------|----------|--------|
| 11 | **Registry integration** | MCP Registry (registry.modelcontextprotocol.io) for server discovery. Publish mcpkit servers automatically. | Medium |
| 12 | **DPoP token binding** | OAuth spec extension for proof-of-possession. Prevents token replay attacks. | Medium |
| 13 | **Workload Identity** | Cloud-native auth (GCP, AWS IAM) for server-to-server MCP without shared secrets. | High |
| 14 | **WebMCP bridge** | Chrome's W3C Extended Privacy Pass for browser-based MCP. Bridge package for web clients. | High |
| 15 | **Extensions framework** | MCP roadmap mentions protocol extensions. Plugin system for custom capabilities. | High |
| 16 | **Gateway pattern** | Multi-server aggregation, routing, and load balancing for MCP server fleets. | High |
| 17 | **Playbooks** | Guided multi-step workflows composed from tools. Declarative YAML/JSON definition. | Medium |

</bootstrap-opportunities>

---

<implementation-sequence>

## Phased Implementation

### Phase 1: Foundation (Weeks 1–2)
- [ ] Resources package (Layer 1, mirrors registry pattern)
- [ ] Prompts package (Layer 1, mirrors registry pattern)
- [ ] Streamable HTTP integration tests
- [ ] Bump mcp-go to v0.45.0

### Phase 2: Examples + Auth (Weeks 3–4)
- [ ] Minimal example server (1 tool, stdio transport)
- [ ] Full-featured example (auth + resilience + observability)
- [ ] hg-mcp migration example (compat.go showcase)
- [ ] OAuth token exchange completion
- [ ] Logging endpoint integration

### Phase 3: Differentiation (Weeks 5–6)
- [ ] Dispatcher package (from jobb patterns)
- [ ] Sampling middleware
- [ ] Observability integration tests
- [ ] Official SDK dual-testing CI

### Phase 4: Future-Proofing (Week 7+)
- [ ] MCP Registry integration
- [ ] DPoP token binding
- [ ] Gateway pattern exploration
- [ ] Playbooks prototype

### Decision Points
- **After Phase 1**: Evaluate official go-sdk maturity — if v2.0 ships, prioritize migration over mcp-go upgrades
- **After Phase 2**: Assess adoption metrics — if hg-mcp migration succeeds, invest in Tier 3; otherwise, focus on developer experience
- **After Phase 3**: Re-evaluate WebMCP and Extensions based on spec evolution

</implementation-sequence>
