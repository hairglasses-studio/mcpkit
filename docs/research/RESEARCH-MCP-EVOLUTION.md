# MCP Evolution & Future Direction Research

Research on the Model Context Protocol's evolution, ecosystem, and trajectory.
Compiled: 2026-03-14.

---

## 1. MCP Specification History

### Version Timeline

| Version | Release Date | Tag |
|---------|-------------|-----|
| 2024-10-07 | Oct 2024 | Pre-release draft |
| 2024-11-05 | Nov 2024 | Initial public release (coincided with Anthropic announcement) |
| 2024-11-05-final | Late 2024 | Final revision of initial spec with backwards-compatible fixes |
| 2025-03-26 | Mar 2025 | Major transport and auth overhaul |
| 2025-06-18 | Jun 2025 | Elicitation, structured output, OAuth resource servers |
| 2025-11-25-RC | Late 2025 | Release candidate |
| 2025-11-25 | Nov 2025 | Current stable release: tasks, icons, enhanced auth |

### What Changed Between Versions

#### 2024-11-05 (Initial Release)
The founding spec established the core architecture:
- JSON-RPC 2.0 message format over stateful connections
- Three-tier architecture: Hosts, Clients, Servers
- **Server features**: Tools, Resources, Prompts
- **Client features**: Sampling (server-initiated LLM calls)
- **Transports**: stdio and HTTP+SSE (separate endpoints for SSE stream and POST)
- **Utilities**: Progress tracking, cancellation, logging, error reporting
- Capability negotiation during initialization
- Pagination via opaque cursors

No authentication was specified in the initial release. The transport model used separate SSE and POST endpoints.

#### 2025-03-26 (Transport & Auth Overhaul)
Major additions:
- **Streamable HTTP transport** replaced HTTP+SSE. Single MCP endpoint handles both POST and GET. Server can respond with either JSON or SSE stream. Supports session management via `Mcp-Session-Id` header, resumability via SSE event IDs, and explicit session termination via DELETE.
- **OAuth 2.1 authorization** with PKCE (required), Dynamic Client Registration (RFC 7591), Authorization Server Metadata discovery (RFC 8414), third-party authorization delegation.
- **Tool annotations** for describing tool behavior (read-only, destructive, idempotent).
- Backwards compatibility guidance for clients/servers supporting both old and new transports.
- DNS rebinding protections (Origin header validation, localhost binding).

#### 2025-06-18 (Elicitation & Structured Output)
Major additions:
1. **Elicitation** -- servers can request structured user input during tool execution. Supports string, number, boolean, and enum schema types. Three-action response model (accept/decline/cancel).
2. **Structured tool output** -- tools can declare output schemas and return structured content alongside human-readable content.
3. **Resource links** in tool call results.
4. **OAuth Resource Server classification** -- MCP servers classified as OAuth Resource Servers with protected resource metadata discovery.
5. **Resource Indicators (RFC 8707)** -- required for clients to prevent malicious servers from obtaining tokens.
6. **`title` field** added for human-friendly display names (separate from programmatic `name`).
7. **`_meta` field** standardized across interface types.
8. **`context` field** in CompletionRequest for previously-resolved variables.
9. **Removed JSON-RPC batching** support.
10. **`MCP-Protocol-Version` header** required on HTTP requests.
11. Security best practices page added.

#### 2025-11-25 (Tasks, Enhanced Auth, Governance)
Major additions:
1. **Tasks** (experimental) -- durable state machines for long-running operations. Two-phase response: immediate CreateTaskResult, then poll via tasks/get, retrieve via tasks/result. Supports working/input_required/completed/failed/cancelled statuses. Tool-level negotiation (required/optional/forbidden). Task cancellation and listing. Both clients and servers can be requestors or receivers.
2. **OpenID Connect Discovery 1.0** support for auth server discovery.
3. **Icons** as metadata for tools, resources, resource templates, and prompts.
4. **Incremental scope consent** via WWW-Authenticate.
5. **OAuth Client ID Metadata Documents** as recommended client registration mechanism.
6. **URL mode elicitation** for requesting URLs from users.
7. **Tool calling in sampling** via `tools` and `toolChoice` parameters.
8. **Enhanced elicitation enums** with titled, untitled, single-select, and multi-select support.
9. **Default values** for all primitive types in elicitation schemas.
10. **JSON Schema 2020-12** as default dialect.
11. **Governance formalization** -- working groups, interest groups, SDK tiering system.
12. **Tool naming guidance** standardized.
13. Input validation errors clarified as Tool Execution Errors (not Protocol Errors).
14. Polling SSE streams support (servers can disconnect at will).

---

## 2. MCP Roadmap & Future Directions

Based on GitHub issues and discussions on modelcontextprotocol/specification:

### Active Feature Discussions

**Security & Authorization**
- Agent Identity & Delegation (#2404) -- identity management for tool calls
- Gateway-Based Authorization Model (#804) -- comprehensive security framework (16 votes)
- Tool Integrity (#2402) -- consolidation of 15 security proposals
- Scope-filtered tool discovery (SEP-1881) -- restricted tool access by permissions
- Step-up authorization behavior clarification

**Discovery & Registry**
- DNS-native Discovery (#2368) -- zero-infrastructure alternative
- DNS TXT Records for Registry (#2334) -- organization-scoped discovery
- Lazy tool hydration for large tool sets

**Architecture & Grouping**
- Primitive Groups (#1567) -- grouping tools, resources, and prompts (100+ comments, highly active)
- Prompts as Invokable Skills (#1779) -- enhancing prompt functionality
- Extension negotiation capabilities
- Wait/await operations to reduce polling

**Tool Enhancements**
- Tool Annotation Semantics (#2382) -- "egressHint" and "reversibleHint" proposals
- Response Size Limits (#2211) -- preventing context overflow (7 votes)
- Standard Feedback Mechanism (#1879) -- feedback for tool responses
- Free-text message responses for elicitation
- Standardized cache hints in tool results

**Governance & Compliance**
- Inter-agent Compliance Semantics (#2367)
- Governance Layer (#2379) -- pre-execution semantic objects for authorization

**Operations**
- Server Health Checks (#2403) -- diagnostics tooling
- Server Analytics (#1236) -- usage analytics

### Likely Near-Term Additions
Based on discussion activity and specification trajectory:
1. Primitive groups (very high activity)
2. Enhanced tool annotations (egressHint, reversibleHint)
3. Agent identity/delegation
4. DNS-based discovery
5. Response size management
6. Tasks graduating from experimental to stable

---

## 3. MCP SDK Landscape

### Official SDKs

| SDK | Stars | Spec Version | Transports | Key Notes |
|-----|-------|-------------|------------|-----------|
| **TypeScript SDK** | High | 2025-11-25 | Streamable HTTP, stdio | v2 pre-alpha in development (Q1 2026). v1.x stable. Runs on Node.js, Bun, Deno. Split architecture (@modelcontextprotocol/server + /client). Zod v4 for schema validation. Framework adapters for Express, Hono. |
| **Python SDK** | 22.1k | 2025-11-25 | stdio, SSE, Streamable HTTP, ASGI | v2 pre-alpha on main branch. FastMCP high-level decorator API. Full typing support. Pydantic models, TypedDicts, dataclasses for structured output. |
| **Go SDK** (official) | 4.1k | 2025-11-25 | stdio, command | Maintained with Google collaboration. 91 contributors. Client-side OAuth experimental. Backward compat with 2025-06-18, 2025-03-26, 2024-11-05. |

### Community SDKs

| SDK | Stars | Spec Version | Key Notes |
|-----|-------|-------------|-----------|
| **mcp-go** (mark3labs) | 8.4k | 2025-11-25 | Community-originated, inspired official Go SDK. Backward compat with earlier specs. Session management, per-session tool filtering, request hooks, middleware. Under active development. |

### SDK Maturity Assessment

**TypeScript SDK**: Reference implementation. Most mature. All spec features typically land here first.

**Python SDK**: Very popular (22.1k stars). FastMCP abstraction layer is influential -- the decorator-based API pattern has been widely adopted. ASGI integration for embedding in existing web frameworks.

**Go SDK (official)**: Newer than mcp-go but gaining traction. Google collaboration adds credibility. 611 commits, 20 releases. Acknowledges mcp-go as inspiration.

**mcp-go (mark3labs)**: Community standard for Go before the official SDK. Higher star count (8.4k vs 4.1k). More battle-tested in production. The official SDK explicitly credits it.

### SDK-Specific vs Spec-Defined Features

**Spec-defined** (must be consistent across SDKs):
- All JSON-RPC message formats and methods
- Capability negotiation
- Transport protocols (stdio, Streamable HTTP)
- Authorization flows (OAuth 2.1, PKCE, resource indicators)
- Tool/Resource/Prompt schemas
- Sampling, elicitation, tasks protocols

**SDK-specific** (implementation varies):
- High-level APIs (FastMCP decorators, TypedHandler generics)
- Middleware patterns (per-SDK design)
- Framework integration (Express, Hono, ASGI)
- Session management abstractions
- Test utilities
- Error handling ergonomics

---

## 4. MCP Adoption

### Launch & Early Adoption (Nov 2024)
Anthropic open-sourced MCP on November 25, 2024. Initial adoption came from:
- **Block** and **Apollo** as early enterprise adopters
- **Development tools**: Zed, Replit, Codeium, Sourcegraph integrated MCP for AI-assisted coding
- **Claude Desktop** supported local MCP servers on all plans
- Pre-built servers: Google Drive, Slack, GitHub, Git, Postgres, Puppeteer

### Current Ecosystem (as of early 2026)
The MCP clients page lists 60+ applications supporting MCP, spanning categories:
- **AI assistants**: Claude Desktop, ChatGPT (via plugins), various open-source assistants
- **IDEs & dev tools**: VS Code, Cursor, Zed, Windsurf, Cline, Continue
- **Agent frameworks**: AgenticFlow, AgentAI (Rust), various orchestration platforms
- **Enterprise platforms**: Multiple SaaS integrations
- **Libraries**: SDKs in Go, Python, TypeScript, Rust, Java, Swift, C#

### Feature Support Distribution
The client feature matrix shows varying levels of adoption:
- **Tools**: Near-universal support (the entry point for most integrations)
- **Resources, Prompts**: Common but not universal
- **Discovery, Instructions**: Growing adoption
- **Sampling, Roots, Elicitation**: More advanced; fewer clients
- **CIMD, DCR**: Auth features; enterprise-focused
- **Tasks, Apps**: Newest features; limited adoption so far

### Adoption Patterns Observed
1. **Tools-first**: Nearly all adopters start with tool support, then add resources/prompts
2. **Stdio dominance**: Local development heavily favors stdio transport
3. **Hub-and-spoke**: One host (e.g., Claude Desktop, VS Code) connecting to many servers
4. **Community servers**: Large ecosystem of community-built MCP servers (registries, databases, APIs)
5. **Enterprise caution**: OAuth/auth features adopted more slowly, typically by SaaS providers

---

## 5. MCP vs Alternatives

### MCP vs OpenAI Function Calling
| Dimension | MCP | OpenAI Function Calling |
|-----------|-----|------------------------|
| **Scope** | Full protocol (transport, auth, lifecycle) | API-level tool definition |
| **Architecture** | Client-server with stateful sessions | Stateless request/response |
| **Discovery** | Dynamic (tools/list, notifications) | Static (defined per API call) |
| **Resources** | First-class URI-based data exposure | Not supported |
| **Prompts** | First-class templates | Not supported |
| **Sampling** | Server can request LLM completions | Not supported |
| **Auth** | Full OAuth 2.1 with PKCE, DPoP | API key based |
| **Transport** | stdio, Streamable HTTP | HTTPS only |
| **Multi-modal** | Text, image, audio content types | JSON schemas only |
| **Adoption** | Growing ecosystem, multi-vendor | Dominant in OpenAI ecosystem |

MCP is significantly more ambitious in scope. OpenAI function calling is simpler to adopt but limited to the OpenAI API surface.

### MCP vs Google A2A
A2A (Agent-to-Agent) and MCP address fundamentally different problems:

| Dimension | MCP | A2A |
|-----------|-----|-----|
| **Purpose** | Connect LLM apps to tools/data | Connect agents to agents |
| **Relationship** | Client-server (host to tool) | Peer-to-peer (agent to agent) |
| **Primitives** | Tools, Resources, Prompts | Tasks, Messages, Agent Cards |
| **Identity** | Server provides capabilities | Agent describes skills |
| **Communication** | JSON-RPC, synchronous/async | Task-based, fully async |
| **Auth** | OAuth 2.1 | Agent authentication |

**Complementary, not competitive**: MCP defines how an agent accesses tools and data. A2A defines how agents talk to each other. A production system would use both -- MCP for tool access, A2A for agent coordination.

### MCP vs Tool Use Protocol Proposals
Various proposals for standardized tool use exist:
- **OpenAPI/Swagger**: API description, not a protocol. MCP servers can wrap OpenAPI endpoints.
- **LangChain Tools**: Framework-specific, not a protocol. Can use MCP as a backend.
- **Semantic Kernel**: Microsoft's abstraction. Plugin model, not network protocol.

MCP's advantage: it is the only protocol-level standard with broad multi-vendor adoption for LLM-tool integration.

### Where MCP Excels
- Standardized lifecycle (init, operate, shutdown)
- Capability negotiation prevents feature mismatches
- Multi-modal content types (text, image, audio)
- Server-initiated interactions (sampling, elicitation)
- Dynamic tool/resource discovery with change notifications
- Production auth story (OAuth 2.1, PKCE, resource indicators)
- Tasks for long-running operations

### Where MCP Falls Short
- Complexity: OAuth 2.1 + Streamable HTTP + tasks is a lot to implement correctly
- No native agent-to-agent communication (addressed by A2A)
- No built-in workflow/orchestration primitives
- Stateful sessions add operational complexity (session affinity, reconnection)
- SDK maturity varies across languages
- Primitive groups (tool organization at scale) still under discussion
- No built-in rate limiting or quota management in the spec

---

## 6. Transport Layer

### Current State

**stdio** (since 2024-11-05):
- Client launches server as subprocess
- JSON-RPC over stdin/stdout, logs on stderr
- Newline-delimited messages
- Simplest transport; recommended for local development
- No auth needed (process isolation)

**Streamable HTTP** (since 2025-03-26, replacing HTTP+SSE):
- Single MCP endpoint handles POST and GET
- POST for client-to-server messages; response can be JSON or SSE stream
- GET opens SSE stream for server-initiated messages
- Session management via `Mcp-Session-Id` header
- Resumability via SSE event IDs and `Last-Event-ID`
- Explicit session termination via DELETE
- Must validate Origin header (DNS rebinding protection)
- Must bind to localhost when running locally
- Backwards compatibility path from old HTTP+SSE

**HTTP+SSE** (2024-11-05, deprecated):
- Separate SSE endpoint and POST endpoint
- Still in use by older implementations
- Backwards compatibility guidance provided

### Transport Gaps
- **WebSocket**: Not in spec. Discussed but not planned. Streamable HTTP covers the use case.
- **gRPC**: Not in spec. Could be a custom transport.
- **QUIC/HTTP3**: Not discussed.

### What's Coming
- Polling SSE streams (2025-11-25): servers can disconnect SSE at will, clients poll
- Improved resumability patterns
- Session affinity documentation for stateful deployments
- The trend is toward making Streamable HTTP more robust rather than adding new transports

---

## 7. Authentication & Authorization

### What's Specified (2025-11-25)

**OAuth 2.1** (IETF Draft):
- PKCE required for all clients
- Authorization Code grant (human users) and Client Credentials grant (machine-to-machine)
- Dynamic Client Registration (RFC 7591) -- recommended
- Authorization Server Metadata discovery (RFC 8414) -- required for clients
- Protected Resource Metadata (RFC 9728) -- MCP servers as OAuth Resource Servers
- Resource Indicators (RFC 8707) -- required for clients to prevent token theft
- Third-party authorization delegation (MCP server proxies to external IdP)
- Client ID Metadata Documents -- recommended registration mechanism

**OpenID Connect Discovery 1.0** (2025-11-25):
- Alternative to RFC 8414 for auth server discovery

**Incremental Scope Consent** (2025-11-25):
- Step-up authorization via WWW-Authenticate
- Scope accumulation across sessions

**Token Handling**:
- Bearer tokens in Authorization header on every request
- Token rotation recommended
- Refresh token support
- Lifetime management

### What's Missing for Production

**DPoP (Demonstration of Proof-of-Possession)**:
- Not in the MCP spec itself
- Implemented by some libraries (mcpkit has auth/dpop.go)
- Important for token binding in zero-trust environments

**Workload Identity**:
- GCP IAM, AWS IAM, Azure AD workload identity
- Not specified; each deployment handles differently
- Critical for cloud-native deployments

**mTLS**:
- Not specified
- Needed for service-to-service in enterprise environments

**API Key Authentication**:
- Not specified (OAuth only)
- Many simple deployments want API keys

**RBAC/Permissions**:
- Spec mentions scopes but doesn't define a permission model
- Tool-level access control left to implementation
- Scope-filtered tool discovery under discussion (SEP-1881)

---

## 8. MCP Extensions & Capabilities Negotiation

### Extension Pattern
MCP's capability negotiation during initialization provides a natural extension point:
- Client and server exchange `capabilities` objects
- Both can include `experimental` capabilities for non-standard features
- Extensions can be negotiated without spec changes

### Known Extension Patterns

**Tool Annotations** (spec-defined):
- `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`
- Trust considerations: annotations from untrusted servers should not be relied upon

**Icons** (2025-11-25):
- Metadata icons for tools, resources, resource templates, and prompts
- Enables richer UI presentation

**Task-Augmented Tools** (2025-11-25):
- `execution.taskSupport` on tools (required/optional/forbidden)
- Fine-grained per-tool task negotiation

**_meta Field**:
- Standardized across interface types (2025-06-18)
- Extensible metadata container
- Used for `io.modelcontextprotocol/related-task` associations

### Community Extension Patterns

**Health Checks**:
- Under discussion (#2403)
- Not yet standardized
- Libraries implement their own (mcpkit has health package)

**Observability**:
- OpenTelemetry tracing/metrics not in spec
- Libraries add middleware (mcpkit has observability package)

**Rate Limiting / FinOps**:
- Not in spec
- Token accounting, budget policies at library level

**Agent Memory**:
- Not in spec
- Pluggable storage backends at library level

**Gateway / Multi-Server Aggregation**:
- Not in spec
- Namespaced tool routing at library level

### Extensions Under Discussion
- Extension negotiation capabilities (GitHub issues)
- Primitive groups (tool organization)
- Server analytics
- Compliance semantics
- Governance layers

### Assessment
The MCP extension model is implicit rather than explicit. There is no formal extension registry or negotiation protocol beyond `experimental` capabilities. The `_meta` field and namespaced keys (e.g., `io.modelcontextprotocol/*`) provide a convention but not a framework. This is an area where the community is actively building ahead of the spec.

---

## Summary: Key Takeaways

1. **MCP is maturing rapidly** -- five spec versions in 13 months, with each adding substantial capabilities. The protocol has moved from basic tool calling to a comprehensive integration framework.

2. **Tasks are the biggest recent addition** -- experimental in 2025-11-25, they enable async operations, polling, and deferred results. This is critical for production workloads with long-running operations.

3. **Auth is production-ready but complex** -- OAuth 2.1 with PKCE, resource indicators, and dynamic client registration. Missing DPoP and workload identity for advanced deployments.

4. **Streamable HTTP is the future** -- HTTP+SSE is deprecated. The new transport is more capable but more complex. Session management and resumability are well-designed.

5. **SDK ecosystem is fragmented but consolidating** -- official SDKs in TS, Python, Go. Community SDKs remain viable (mcp-go has more stars than official Go SDK). SDK tiering system being formalized.

6. **MCP and A2A are complementary** -- MCP for tool/data access, A2A for agent-to-agent. Both needed for full agent architectures.

7. **Extensions are ad-hoc** -- no formal extension framework. Community builds ahead of spec (health, observability, rate limiting, memory). Primitive groups under heavy discussion.

8. **Adoption is broad but shallow** -- 60+ clients support MCP, but most only implement tools. Advanced features (sampling, elicitation, tasks) have limited client support.

---

## Sources

- modelcontextprotocol.io/specification (2024-11-05, 2025-03-26, 2025-06-18, 2025-11-25)
- github.com/modelcontextprotocol/specification (releases, issues, discussions)
- github.com/modelcontextprotocol/go-sdk
- github.com/modelcontextprotocol/python-sdk
- github.com/modelcontextprotocol/typescript-sdk
- github.com/mark3labs/mcp-go
- anthropic.com/news/model-context-protocol
- anthropic.com/research/building-effective-agents
- modelcontextprotocol.io/clients
