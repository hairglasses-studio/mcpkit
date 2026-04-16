# mcpkit Roadmap

Last updated: 2026-03-15.

## Status Summary

- **Spec coverage**: 100% (all MCP 2025-11-25 features implemented)
- **Tiers 1–4**: Complete (A2A deferred — spec not stable)
- **Test coverage**: All Phase 1–30 packages at 90%+ coverage. `transport` (added Phase 33) at 16% — pending test hardening
- **Documentation**: 37 packages have `doc.go`, 36 `example_test.go` files (as of 2026-04-03)

See [RESEARCH.md](RESEARCH.md) for detailed analysis and evidence.

---

## Ecosystem Resources

| Resource | URL |
|----------|-----|
| MCP Spec (current) | https://modelcontextprotocol.io/specification/2025-11-25 |
| MCP Roadmap | https://modelcontextprotocol.io/development/roadmap |
| mcp-go | https://github.com/mark3labs/mcp-go |
| Official go-sdk | https://github.com/modelcontextprotocol/go-sdk |
| FastMCP | https://github.com/jlowin/fastmcp |
| TypeScript SDK | https://github.com/modelcontextprotocol/typescript-sdk |
| A2A Protocol | https://github.com/google/A2A |
| Anthropic Blog | https://www.anthropic.com/news |

---

## Completed Phases (1–30)

### Phases 1–4: Foundation (COMPLETE)

Registry, handler, resilience, auth, resources, prompts, security, observability, health, sanitize, secrets, client, discovery. Full MCP 2025-11-25 spec coverage including DPoP, JWKS, PKCE, RBAC, audit logging, and gateway aggregation.

### Phases 5–7: Ralph + Multi-Agent + Workflow (COMPLETE)

Ralph autonomous loop runner with DAG enforcement, multi-tool selection, cost tracking, YAML specs. `extensions/`, `memory/`, `finops/`, `orchestrator/`, `handoff/`, `skills/`, `workflow/`, `bootstrap/` packages. Workload identity (GCP/AWS).

### Phases 8–10: Security + Testing + Production (COMPLETE)

Output sanitization, URI validation, tool integrity verification, tenant context propagation. Session replay, snapshot testing, benchmark helpers, FinOps v2 (cost estimation, scoped budgets, time-windowed tracking). Per-upstream gateway resilience, orchestrator/handoff/workflow middleware.

### Phases 11–16: DX + Observability + Eval (COMPLETE)

README overhaul, 28 runnable Godoc examples, full OTel tracing across gateway/orchestrator/workflow/handoff/ralph, server lifecycle manager, health readiness. Server cards (`.well-known/mcp.json`), Ed25519 tool signing, eval framework (6 scorers, JSON suite loading, ResultScorer), SIEM/audit export (JSONL + stream), security middleware tests.

### Phases 17–24: Coverage + R&D Tools + Workflow Engine (COMPLETE)

Dispatcher/sampling unit tests, core registry/workflow/auth test coverage, discovery/handler/memory/finops/resilience test coverage. `roadmap/` machine-readable types, `research/` GitHub activity + diff analysis, `rdcycle/` R&D orchestration tools. Workflow fork nodes (parallel branches), compensation/saga rollback, dynamic gateway upstream registry. Autonomous loop guardrails, budget profiles, model tier config, improvement notes, YAML spec support.

### Phases 25–30: Coverage Hardening + Documentation (COMPLETE)

All Phase 1–30 packages raised to 90%+ coverage. All 33 packages at that time documented with `doc.go`. Phase 30 pushed 11 packages past the 90% threshold (auth, eval, ralph, research, gateway, mcptest, secrets/providers, handler, rdcycle, observability, roadmap). Note: `transport` (Phase 33) and utility packages added later require separate test hardening.

---

## Planned Phases (31–42)

<roadmap-tier id="T5" name="Spec Forward-Compatibility">

<roadmap-phase id="P31" status="planned" name="Session Management Foundation">

<roadmap-item id="P31-1" package="session" status="planned">
Session and SessionStore interfaces — define core session lifecycle types used across all session middleware.
</roadmap-item>

<roadmap-item id="P31-2" package="session" status="planned">
In-memory session store — thread-safe default SessionStore implementation with map-backed storage.
</roadmap-item>

<roadmap-item id="P31-3" package="session" status="planned">
Session middleware — attach/read session from MCP request context, create on first contact.
</roadmap-item>

<roadmap-item id="P31-4" package="session" status="planned">
Session migration helpers — utilities to migrate session identity across transport reconnects.
</roadmap-item>

<roadmap-item id="P31-5" package="session" status="planned">
Session TTL and eviction — configurable expiry with background eviction goroutine.
</roadmap-item>

<roadmap-item id="P31-6" package="gateway" status="planned">
Gateway session affinity — route requests with a session token to the same upstream backend.
</roadmap-item>

<roadmap-item id="P31-7" package="mcptest" status="planned">
Session integration tests — mcptest helpers for asserting session lifecycle across tool calls.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P32" status="planned" name="Stateless HTTP">

<roadmap-item id="P32-1" package="session" status="planned">
External session store / Redis adapter — pluggable SessionStore backed by Redis for stateless deployments.
</roadmap-item>

<roadmap-item id="P32-2" package="session" status="planned">
Session token extraction middleware — extract session tokens from headers, cookies, or query params.
</roadmap-item>

<roadmap-item id="P32-3" package="gateway" status="planned">
Stateless gateway routing — gateway mode that reads session affinity from token without local state.
</roadmap-item>

<roadmap-item id="P32-4" package="session" status="planned">
Session serialization (JSON + gob) — encode/decode session values for external store round-trips.
</roadmap-item>

<roadmap-item id="P32-5" package="session" status="planned">
Load balancer compat tests — verify session consistency under simulated round-robin routing.
</roadmap-item>

<roadmap-item id="P32-6" package="health" status="planned">
Session store health checks — health.Checker integration for external session store liveness.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P33" status="planned" name="WebSocket Transport Prep">

<roadmap-item id="P33-1" package="transport" status="planned">
Transport abstraction interfaces — Transport, Conn, and Message interfaces decoupling protocol from tool dispatch.
</roadmap-item>

<roadmap-item id="P33-2" package="transport" status="planned">
Stdio transport adapter — wrap existing stdio server path behind the Transport interface.
</roadmap-item>

<roadmap-item id="P33-3" package="transport" status="planned">
HTTP transport adapter — wrap StreamableHTTP server path behind the Transport interface.
</roadmap-item>

<roadmap-item id="P33-4" package="transport" status="planned">
WebSocket transport stub — placeholder WebSocket Transport implementation gated on spec stabilization.
</roadmap-item>

<roadmap-item id="P33-5" package="transport" status="planned">
Transport middleware chain — apply registry middleware at the transport boundary before dispatch.
</roadmap-item>

<roadmap-item id="P33-6" package="transport" status="planned">
Transport integration tests — end-to-end tests exercising tool calls through each transport adapter.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T6" name="SDK Migration">

<roadmap-phase id="P34" status="planned" name="Dual-SDK Test Hardening">

<roadmap-item id="P34-1" package="registry" status="planned">
Audit compat.go aliases — verify all public adapter functions (MakeTextContent, MakeErrorResult, ExtractArguments) compile under both build tags.
</roadmap-item>

<roadmap-item id="P34-2" package="registry" status="planned">
_official_test.go files — parallel test files under the official_sdk build tag covering compat.go paths.
</roadmap-item>

<roadmap-item id="P34-3" status="planned">
Dual-SDK CI matrix — GitHub Actions matrix builds with and without the official_sdk tag on every PR.
</roadmap-item>

<roadmap-item id="P34-4" status="planned">
Migration guide document — step-by-step guide for projects moving from mcp-go to go-sdk with mcpkit.
</roadmap-item>

<roadmap-item id="P34-5" status="planned">
Bump mcp-go to latest — update to latest mcp-go minor; validate no compat regressions.
</roadmap-item>

<roadmap-item id="P34-6" package="registry" status="planned">
go-sdk v2.0 compat assessment — evaluate breaking changes once v2.0 is announced; plan compat.go updates.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T7" name="Research Enhancement">

<roadmap-phase id="P35" status="planned" name="Cloud Platform Monitoring">

<roadmap-item id="P35-1" package="research" status="planned">
PlatformMonitor interface — common interface for cloud-platform MCP activity monitors.
</roadmap-item>

<roadmap-item id="P35-2" package="research" status="planned">
Cloudflare Workers monitor — poll Cloudflare blog and Workers changelog for MCP-adjacent announcements.
</roadmap-item>

<roadmap-item id="P35-3" package="research" status="planned">
Vercel adapter monitor — track Vercel AI SDK changelog for MCP transport and adapter updates.
</roadmap-item>

<roadmap-item id="P35-4" package="research" status="planned">
Azure MCP Center monitor — track Azure AI Foundry MCP Center releases and breaking changes.
</roadmap-item>

<roadmap-item id="P35-5" package="research" status="planned">
Platform activity aggregation — aggregate PlatformMonitor results into a unified SummaryOutput for rdcycle scan.
</roadmap-item>

<roadmap-item id="P35-6" package="research" status="planned">
Cloud platform tests — httptest mocks and unit tests for each platform monitor.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P36" status="planned" name="A2A Tracking + Competitive Analysis">

<roadmap-item id="P36-1" package="research" status="planned">
A2AMonitor — poll Google A2A repository for spec version bumps, new agent card examples, and breaking changes.
</roadmap-item>

<roadmap-item id="P36-2" package="research" status="planned">
SDKCompare analysis — diff feature matrices across mcp-go, go-sdk, FastMCP, and TypeScript SDK.
</roadmap-item>

<roadmap-item id="P36-3" package="research" status="planned">
Template reports — pre-built report templates for competitive analysis and gap summaries.
</roadmap-item>

<roadmap-item id="P36-4" package="research" status="planned">
Competitive dashboard data — export structured JSON suitable for a monitoring dashboard.
</roadmap-item>

<roadmap-item id="P36-5" package="rdcycle" status="planned">
rdcycle scan integration — wire A2AMonitor and SDKCompare into the rdcycle_scan tool output.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T8" name="Production DX">

<roadmap-phase id="P37" status="planned" name="Server Registry Publishing">

<roadmap-item id="P37-1" package="discovery" status="planned">
PublishWorkflow + validation — multi-step publish: validate server card, check schema compliance, then publish.
</roadmap-item>

<roadmap-item id="P37-2" package="discovery" status="planned">
Schema compliance checker — validate a ServerCard struct against the MCP registry JSON schema before publishing.
</roadmap-item>

<roadmap-item id="P37-3" package="cmd" status="planned">
CLI publishing helper — thin CLI wrapper around PublishWorkflow for use in CI pipelines.
</roadmap-item>

<roadmap-item id="P37-4" package="discovery" status="planned">
Registry auth flow — OAuth2 client credentials flow for authenticating with the MCP registry API.
</roadmap-item>

<roadmap-item id="P37-5" package="discovery" status="planned">
Publish integration tests — httptest mock of the MCP registry API; exercise full publish + unpublish lifecycle.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P38" status="planned" name="Performance Benchmarking">

<roadmap-item id="P38-1" package="mcptest" status="planned">
Baseline benchmark suite — BenchmarkTool baselines for registry, handler, and no-op middleware overhead.
</roadmap-item>

<roadmap-item id="P38-2" package="mcptest" status="planned">
Middleware overhead measurement — per-middleware latency measurement using BenchmarkSuite with named layers.
</roadmap-item>

<roadmap-item id="P38-3" package="mcptest" status="planned">
Memory profiling helpers — allocs-per-op assertions in benchmark helpers to catch allocation regressions.
</roadmap-item>

<roadmap-item id="P38-4" status="planned">
CI regression thresholds — fail CI if any benchmark regresses beyond a configured latency or alloc budget.
</roadmap-item>

<roadmap-item id="P38-5" package="mcptest" status="planned">
Benchmark comparison tool — compare two BenchmarkSuite runs and report per-tool delta percentages.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T9" name="Agent Protocol Evolution">

<roadmap-phase id="P39" status="planned" name="A2A Protocol Bridge">

<roadmap-item id="P39-1" package="a2a" status="planned">
A2A types — AgentCard, Task, TaskStatus, Artifact, and Message structs aligned to A2A spec v0.9+.
</roadmap-item>

<roadmap-item id="P39-2" package="a2a" status="planned">
AgentCard generation — derive AgentCard from registry.Registry metadata and server card fields.
</roadmap-item>

<roadmap-item id="P39-3" package="a2a" status="planned">
Task lifecycle — submit, update, cancel, and poll task state with A2A-compliant status transitions.
</roadmap-item>

<roadmap-item id="P39-4" package="a2a" status="planned">
MCP-to-A2A bridge — translate MCP tool calls into A2A Task submissions and relay results back.
</roadmap-item>

<roadmap-item id="P39-5" package="a2a" status="planned">
A2A-to-MCP bridge — expose an A2A agent endpoint that dispatches to mcpkit tool handlers.
</roadmap-item>

<roadmap-item id="P39-6" package="a2a" status="planned">
Push notifications — Server-Sent Events stream for A2A task status updates.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P40" status="planned" name="Enhanced Orchestration">

<roadmap-item id="P40-1" package="orchestrator" status="planned">
Swarm mesh — peer-to-peer agent communication pattern with broadcast and unicast routing.
</roadmap-item>

<roadmap-item id="P40-2" package="orchestrator" status="planned">
Hierarchical delegation — nested manager/worker trees with result aggregation at each level.
</roadmap-item>

<roadmap-item id="P40-3" package="orchestrator" status="planned">
Dynamic pattern selector — choose fan-out, pipeline, or select pattern based on runtime task metadata.
</roadmap-item>

<roadmap-item id="P40-4" package="orchestrator" status="planned">
Performance benchmarks — BenchmarkSuite covering fan-out at 1/10/100 agents with latency assertions.
</roadmap-item>

<roadmap-item id="P40-5" package="workflow" status="planned">
Multi-agent workflow templates — pre-built workflow graphs for common multi-agent topologies.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T10" name="Community">

<roadmap-phase id="P41" status="planned" name="Example Gallery + Migration Guides">

<roadmap-item id="P41-1" status="planned">
Example gallery index — `examples/README.md` linking every `examples/*/main.go` with a one-line summary.
</roadmap-item>

<roadmap-item id="P41-2" status="planned">
FastMCP migration guide — side-by-side FastMCP Python → mcpkit Go translation for the most common patterns.
</roadmap-item>

<roadmap-item id="P41-3" status="planned">
Docker-compose example — `examples/docker/` with compose file, health checks, and lifecycle integration.
</roadmap-item>

<roadmap-item id="P41-4" status="planned">
CONTRIBUTING.md — contributor guide: branch naming, test requirements, middleware conventions, review checklist.
</roadmap-item>

<roadmap-item id="P41-5" status="planned">
Tutorial content outline — structured outline for a multi-part "Build your first MCP server" tutorial series.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P42" status="planned" name="User Feedback + Telemetry">

<roadmap-item id="P42-1" package="feedback" status="planned">
Feedback tool — MCP tool that collects structured user feedback and writes to a configured sink.
</roadmap-item>

<roadmap-item id="P42-2" package="feedback" status="planned">
Anonymous telemetry — opt-in usage telemetry (package usage counts, error rates) with explicit consent gate.
</roadmap-item>

<roadmap-item id="P42-3" package="rdcycle" status="planned">
rdcycle integration — wire feedback summaries into rdcycle_scan output as a signal source.
</roadmap-item>

<roadmap-item id="P42-4" package="feedback" status="planned">
Telemetry dashboard export — export aggregated telemetry as JSON suitable for a Grafana data source.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

---

## Decision Gates

| Gate | Condition |
|------|-----------|
| Before P33 | Evaluate the June 2026 spec draft for WebSocket transport details before implementing the WebSocket stub beyond a placeholder. |
| Before P39 | A2A spec must reach v0.9+ under Linux Foundation governance before any A2A bridge work begins. |
| Before P34-6 | Wait for an official go-sdk v2.0 announcement before scoping compat.go updates. |
| P38 benchmark threshold | Define middleware overhead threshold: no single middleware layer may add more than 5% p99 latency. |

---

## Ralph Loop Execution Strategy

- **Parallel streams**: P31 (session), P32 (stateless HTTP), and P33 (transport) can run concurrently — the packages are independent.
- **P34** (dual-SDK hardening) is independent of P31–P33 and can run in a separate stream.
- **Budget profiles**: Use `PersonalProfile` for P35–P36 (research-heavy, lower token budget); use `WorkAPIProfile` for P31–P34 (implementation-heavy, higher throughput budget).
- **Self-improvement**: `rdcycle_improve` runs every 10 cycles and may inject lessons into the next `rdcycle_schedule` spec.

---

## Dependency Layers (including planned packages)

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`, `lifecycle`, `eval`, `roadmap`, `session`, `transport`, `feedback`
- **Layer 3** (depend on Layer 2): `security`, `gateway`, `ralph`, `skills`, `a2a`, `rdcycle`
- **Layer 4** (depend on Layer 3): `orchestrator`, `handoff`, `workflow`, `bootstrap`

_Note: `session` and `transport` depend only on Layer 1 packages. `feedback` has no internal deps beyond `registry`._

<!-- whiteclaw-rollout:start -->
## Whiteclaw-Derived Overhaul (2026-04-08)

This tranche applies the highest-value whiteclaw findings that fit this repo's real surface: engineer briefs, bounded skills/runbooks, searchable provenance, scoped MCP packaging, and explicit verification ladders.

### Strategic Focus
- Use whiteclaw patterns here as reusable framework features, not one-off repo-local patches.
- The best transfer is a productized explorer/front-door starter that downstream repos can adopt without rewriting transport and contract code.
- Keep public docs and verification aligned with the framework's role as shared infrastructure.

### Recommended Work
- [ ] [Starter surface] Ship an opinionated explorer/front-door starter that covers catalog/search/schema/health for downstream repos.
- [ ] [Docs] Publish a migration guide showing when to use `.mcp.json`, a discovery-first contract layer, or a standalone sidecar package.
- [ ] [Verification] Add a transport and launcher smoke matrix for the public examples and starter surfaces.
- [ ] [Typed boundaries] Keep new tool/command/workflow surfaces on typed contracts rather than handwritten JSON-RPC or loose maps.
- [ ] [Public examples] Expand example coverage around self-explorer, server metadata, and publishable MCP packaging patterns.

### Rationale Snapshot
- Tier / lifecycle: `tier-1` / `active`
- Language profile: `Go`
- Visibility / sensitivity: `PUBLIC` / `public`
- Surface baseline: AGENTS=yes, skills=yes, codex=yes, mcp_manifest=configured, ralph=yes, roadmap=yes
- Whiteclaw transfers in scope: explorer/front-door starter, migration guide, transport smoke matrix, typed contracts
- Live repo notes: AGENTS, skills, Codex config, configured .mcp.json, .ralph, 1 workflow(s), multi-module/workspace, nested roadmaps

<!-- whiteclaw-rollout:end -->

---

## Gap Research: Framework Enhancements (2026-04-16)

Identified from GitHub MCP ecosystem research (30+ repos, 150K+ combined stars). See `docs/research/mcp/github-mcp-gap-research-2026-04-16.md`.

### Tier 1 — High Priority

- [x] [P1][M] server.json for all public MCP servers — blocks registry visibility and MCP directory discovery. Generate `.well-known/mcp.json` with tool categories, version, and discovery metadata for mcpkit, systemd-mcp, tmux-mcp, process-mcp (spec gap analysis). Implemented in `discovery` package: `WriteFile`, `HandleContractWrite`, `ContractWriteFlag`, `ErrContractWritten`, `InstallInfo` struct, and `Categories`/`License`/`Homepage` fields on `ServerMetadata` and `MetadataConfig`.
- [x] [P1][M] Go module security scanning example — wrap govulncheck + OSV API as mcpkit example server: scan go.sum/go.mod, report vulns with severity, suggest upgrades. Implemented in `vuln` package: `Scanner` (govulncheck -format json wrapper), `OSVClient` (OSV API v1/query), `Module` (vuln_scan + vuln_osv_query MCP tools), and `examples/vuln-scanner` runnable server. Severity classification from CVE aliases + keyword heuristics. 30 tests.

### Tier 2 — Medium Priority

- [x] [P2][S] Wire server card + --contract-write into HTTP example — examples/http now mounts `/.well-known/mcp.json` via `ServerCardHandler` and supports `--contract-write` for CI. Canonical reference for downstream adoption.
- [ ] [P2][M] Bounded-write safety middleware — Stripe-style confirmation pattern for MCP tools with financial/destructive side effects. Tool declares `confirm_required: true`, middleware intercepts and requires explicit `confirm` param. Ref: stripe/ai agent-toolkit (1.5K stars)
- [ ] [P2][L] Performance benchmarks — mcpkit middleware chain overhead vs raw mcp-go, p99 latency per middleware layer, throughput under load. Reference threshold: no single middleware layer may add >5% p99 latency
- [ ] [P2][M] Token-efficient schema-first patterns — document and add helpers for dbhub-style schema-before-data, response truncation, pagination for large result sets. Ref: bytebase/dbhub (2.6K stars)
- [ ] [P2][M] Explorer/front-door starter — opinionated catalog/search/schema/health surface for downstream repos to adopt without rewriting transport. Include discovery-first guidance as the documented happy path

<!-- whiteclaw-rollout:end -->
