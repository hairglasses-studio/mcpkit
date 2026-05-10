# mcpkit Roadmap

Last updated: 2026-05-09.

## Status Summary

- **Spec coverage**: 100% (all MCP 2025-11-25 features implemented)
- **Tiers 1–4**: Complete; A2A bridge work has moved into T9 with push-notification endpoints still pending.
- **Test coverage**: All Phase 1–30 packages at 90%+ coverage. `transport` (added Phase 33) at 16% — pending test hardening
- **SDK migration**: P34 core dual-SDK hardening is in progress; `make build-official` and `make test-official` gate the supported official-SDK package set.
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

<roadmap-phase id="P31" status="complete" name="Session Management Foundation">

<roadmap-item id="P31-1" package="session" status="complete">
Session and SessionStore interfaces — define core session lifecycle types used across all session middleware.
</roadmap-item>

<roadmap-item id="P31-2" package="session" status="complete">
In-memory session store — thread-safe default SessionStore implementation with map-backed storage.
</roadmap-item>

<roadmap-item id="P31-3" package="session" status="complete">
Session middleware — attach/read session from MCP request context, create on first contact.
</roadmap-item>

<roadmap-item id="P31-4" package="session" status="complete">
Session migration helpers — utilities to migrate session identity across transport reconnects.
</roadmap-item>

<roadmap-item id="P31-5" package="session" status="complete">
Session TTL and eviction — configurable expiry with background eviction goroutine.
</roadmap-item>

<roadmap-item id="P31-6" package="gateway" status="complete">
Gateway session affinity — route requests with a session token to the same upstream backend.
</roadmap-item>

<roadmap-item id="P31-7" package="mcptest" status="complete">
Session integration tests — mcptest helpers for asserting session lifecycle across tool calls.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P32" status="complete" name="Stateless HTTP">

<roadmap-item id="P32-1" package="session" status="complete">
External session store / Redis adapter — shipped as `session.ExternalStore`, `RedisAdapter`, `RedisStore`, and `RedisStringStore`.
</roadmap-item>

<roadmap-item id="P32-2" package="session" status="complete">
Session token extraction middleware — shipped as `session.TokenMiddleware` plus transport-level `SessionExtractor` for Authorization, header, cookie, and query sources.
</roadmap-item>

<roadmap-item id="P32-3" package="gateway" status="complete">
Stateless gateway routing — shipped as `gateway.AffinityRouter` and `AffinityMiddleware`, using extracted session IDs for consistent-hash upstream routing.
</roadmap-item>

<roadmap-item id="P32-4" package="session" status="complete">
Session serialization (JSON + gob) — shipped as `MarshalSession`/`UnmarshalSession` and gob variants for external-store round-trips.
</roadmap-item>

<roadmap-item id="P32-5" package="session" status="complete">
Load balancer compatibility tests — covered by `gateway/affinity_test.go`, `transport/session_extract_test.go`, and session token middleware tests.
</roadmap-item>

<roadmap-item id="P32-6" package="health" status="complete">
Session store health checks — shipped as `health.WithSessionStore` over any `Ping(ctx)` external store.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P33" status="complete" name="WebSocket Transport Prep">

<roadmap-item id="P33-1" package="transport" status="complete">
Transport abstraction interfaces — shipped as `transport.Transport`, `Message`, middleware chain, and concrete connection adapters.
</roadmap-item>

<roadmap-item id="P33-2" package="transport" status="complete">
Stdio transport adapter — shipped as `StdioTransport` and `NewStdioTransportFromRW` over the shared `ReadWriteTransport`.
</roadmap-item>

<roadmap-item id="P33-3" package="transport" status="complete">
HTTP transport adapter — shipped as `HTTPTransport` with request/response receive-channel delivery and metadata capture.
</roadmap-item>

<roadmap-item id="P33-4" package="transport" status="complete">
WebSocket transport stub — shipped as dependency-free `WebSocketTransport` over caller-provided `WebSocketConn`, with security tests.
</roadmap-item>

<roadmap-item id="P33-5" package="transport" status="complete">
Transport middleware chain — shipped via `transport.Chain`, `LoggingMiddleware`, and `MetricsMiddleware`.
</roadmap-item>

<roadmap-item id="P33-6" package="transport" status="complete">
Transport integration tests — covered by `transport/*_test.go` for HTTP, middleware, base transport, WebSocket, Unix socket, and session extraction.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T6" name="SDK Migration">

<roadmap-phase id="P34" status="in_progress" name="Dual-SDK Test Hardening">

<roadmap-item id="P34-1" package="registry" status="complete">
Audit compat.go aliases — `registry`, `handler`, and `mcptest` now compile and test under both default and `official_sdk` build tags, covering schema adapters, content helpers, argument extraction, and public test helpers.
</roadmap-item>

<roadmap-item id="P34-2" package="registry" status="complete">
_official_test.go files — added official-SDK fixtures for registry schema compatibility, mcptest benchmark helpers, sampling request helpers, and in-memory server/client coverage.
</roadmap-item>

<roadmap-item id="P34-3" status="complete">
Dual-SDK CI gate — `.github/workflows/ci.yml` now runs the default reusable Go test job plus `make build-official test-official` for the supported official-SDK package set.
</roadmap-item>

<roadmap-item id="P34-4" status="complete">
Migration guide document — `docs/sdk-migration-guide.md` now documents the mcp-go to official Go SDK path, compatibility-layer rules, package gates, CI commands, and remaining unsupported fixtures.
</roadmap-item>

<roadmap-item id="P34-5" status="complete">
Bump mcp-go to latest — updated `github.com/mark3labs/mcp-go` from v0.47.0 to v0.52.0 and validated default core tests plus official-SDK build/test gates. Official `github.com/modelcontextprotocol/go-sdk` was also updated from v1.5.0 to v1.6.0.
</roadmap-item>

<roadmap-item id="P34-6" package="registry" status="planned">
go-sdk v2.0 compat assessment — still pending upstream v2.0; live module version check on 2026-05-09 showed latest stable `github.com/modelcontextprotocol/go-sdk` is v1.6.0.
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

<roadmap-phase id="P37" status="complete" name="Server Registry Publishing">

<roadmap-item id="P37-1" package="discovery" status="complete">
PublishWorkflow + validation — `discovery.RunPublishWorkflow` validates metadata, records workflow steps, supports validate-only runs, then registers or updates through `Publisher`.
</roadmap-item>

<roadmap-item id="P37-2" package="discovery" status="complete">
Schema compliance checker — `ValidateServerMetadata` returns structured issues and warnings for required identity fields, URL fields, transports, auth, tags, tools, resources, and prompts before publishing.
</roadmap-item>

<roadmap-item id="P37-3" package="cmd" status="complete">
CLI publishing helper — `cmd/mcpkit-publish` reads server-card JSON, runs `RunPublishWorkflow`, supports validate-only/register/update modes, and uses `-token` or `MCP_REGISTRY_TOKEN` for CI.
</roadmap-item>

<roadmap-item id="P37-4" package="discovery" status="complete">
Registry auth flow — `ClientCredentialsTokenSource` implements OAuth2 client-credentials token fetch/cache, `Publisher` accepts dynamic token sources, and `cmd/mcpkit-publish` exposes OAuth flags/env vars.
</roadmap-item>

<roadmap-item id="P37-5" package="discovery" status="complete">
Publish integration tests — `discovery/publish_integration_test.go` exercises validate/register, update, and unpublish against a stateful httptest MCP registry mock.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P38" status="complete" name="Performance Benchmarking">

<roadmap-item id="P38-1" package="mcptest" status="complete">
Baseline benchmark suite — shipped as `mcptest.BenchmarkTool`, `BenchmarkToolParallel`, `BenchmarkSuite`, `BaselineBenchmark`, and cross-protocol benchmarks under `testing/benchmark`.
</roadmap-item>

<roadmap-item id="P38-2" package="mcptest" status="complete">
Middleware overhead measurement — shipped as `BenchmarkMiddlewareOverhead` plus benchmark cases for no middleware, 1/5/10 no-op middleware layers, context middleware, and parallel middleware throughput.
</roadmap-item>

<roadmap-item id="P38-3" package="mcptest" status="complete">
Memory profiling helpers — `mcptest/allocs.go`: `AssertMaxAllocs(tb, maxAllocs, runs, fn)` wraps `testing.AllocsPerRun` with a readable failure message; `ReportAllocDelta(fn)` measures a single invocation via `runtime.MemStats`; `BenchmarkAllocLimit(b, maxAllocs, fn)` runs a benchmark and fails if the mean allocs-per-op exceeds a threshold. Accepts `testing.TB` so it works from tests and benchmarks. 6 unit tests.
</roadmap-item>

<roadmap-item id="P38-4" status="complete">
CI regression thresholds — `tools/benchguard`, `make bench-guard`, and the PR benchmark-regression workflow compare base/head benchmark output and fail when latency, memory, or allocation budgets are exceeded.
</roadmap-item>

<roadmap-item id="P38-5" package="mcptest" status="complete">
Benchmark comparison tool — shipped as `mcptest.ParseBenchmarkOutput`, `CompareBenchmarkResults`, and `BenchmarkDelta.Regressed`, parsing `go test -bench -benchmem` output and reporting per-benchmark percentage deltas.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T9" name="Agent Protocol Evolution">

<roadmap-phase id="P39" status="complete" name="A2A Protocol Bridge">

<roadmap-item id="P39-1" package="a2a" status="complete">
A2A types — shipped in `a2a/types.go` with AgentCard, Task, TaskState, Message, Part, Artifact, JSON-RPC envelopes, capability flags, and lifecycle tests.
</roadmap-item>

<roadmap-item id="P39-2" package="a2a" status="complete">
AgentCard generation — shipped via `a2a.AgentCardFromRegistry` and `bridge/a2a.AgentCardGenerator`, deriving skills from `registry.ToolRegistry` metadata with filtering and cache invalidation tests.
</roadmap-item>

<roadmap-item id="P39-3" package="a2a" status="complete">
Task lifecycle — shipped in `a2a.Server`, `a2a.Client`, and `bridge/a2a.BridgeExecutor` with send/get/cancel, terminal-state handling, timeout, error, and round-trip tests.
</roadmap-item>

<roadmap-item id="P39-4" package="a2a" status="complete">
MCP-to-A2A bridge — shipped as `a2a.NewBridgeTool` and `bridge/a2a.NewBridge`, exposing mcpkit tools as A2A skills and translating tool results into A2A task artifacts.
</roadmap-item>

<roadmap-item id="P39-5" package="a2a" status="complete">
A2A-to-MCP bridge — shipped via `bridge/a2a.RemoteAgent`, which wraps remote A2A agent skills as `registry.ToolModule` tools and relays responses back as MCP tool results.
</roadmap-item>

<roadmap-item id="P39-6" package="a2a" status="complete">
Push notifications — `a2a.Server` now supports per-task push notification config create/get/list/delete operations, REST endpoints under `/tasks/{taskID}/pushNotificationConfigs`, optional send-time webhook config, and `StreamResponse` webhook delivery on task transitions.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P40" status="complete" name="Enhanced Orchestration">

<roadmap-item id="P40-1" package="orchestrator" status="complete">
Swarm mesh — `orchestrator.NewSwarm` provides peer-to-peer broadcast and unicast message routing with concurrency limits, timeouts, fail-fast cancellation, deterministic responses, and error aggregation.
</roadmap-item>

<roadmap-item id="P40-2" package="orchestrator" status="complete">
Hierarchical delegation — `HierarchicalDelegation` executes nested manager/worker trees, passes parent outputs to children, enforces depth/concurrency controls, and supports per-node result aggregation.
</roadmap-item>

<roadmap-item id="P40-3" package="orchestrator" status="complete">
Dynamic pattern selector — `SelectDynamicPattern` and `RunDynamicPattern` choose fan-out, pipeline, or select from runtime metadata with explicit overrides and conservative defaults.
</roadmap-item>

<roadmap-item id="P40-4" package="orchestrator" status="complete">
Performance benchmarks — `orchestrator/bench_test.go` covers fan-out at 1/10/100 agents, swarm broadcast, hierarchical delegation, and a conservative 100-agent latency budget test.
</roadmap-item>

<roadmap-item id="P40-5" package="workflow" status="complete">
Multi-agent workflow templates — `workflow` now provides reusable node builders and single-node graph templates for fan-out, pipeline, select, dynamic orchestration, swarm broadcast, and hierarchical delegation.
</roadmap-item>

</roadmap-phase>

</roadmap-tier>

<roadmap-tier id="T10" name="Community">

<roadmap-phase id="P41" status="complete" name="Example Gallery + Migration Guides">

<roadmap-item id="P41-1" status="complete">
Example gallery index — `examples/README.md` lists all 13 runnable examples grouped by theme (basics / discovery+catalog / safety+lifecycle / transport / agent protocols) with one-line summaries plus run+authoring guidance.
</roadmap-item>

<roadmap-item id="P41-2" status="complete">
FastMCP migration guide — `docs/fastmcp-migration-guide.md` provides side-by-side Python FastMCP to mcpkit translations for tools, resources, prompts, startup wiring, and test gates.
</roadmap-item>

<roadmap-item id="P41-3" status="complete">
Docker-compose example — `examples/docker/` provides two StreamableHTTP MCP servers behind nginx with compose health checks, identity smoke tests, and lifecycle-driven readiness/drain handling.
</roadmap-item>

<roadmap-item id="P41-4" status="complete">
CONTRIBUTING.md — contributor guide covers branch naming, test requirements, middleware conventions, dual-SDK gates, and reviewer checklist.
</roadmap-item>

<roadmap-item id="P41-5" status="complete">
Tutorial content outline — `docs/tutorial-outline.md` lays out an eight-part "Build your first MCP server" series with outcomes, code milestones, validation gates, and asset requirements.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P42" status="complete" name="User Feedback + Telemetry">

<roadmap-item id="P42-1" package="feedback" status="complete">
Feedback tool — `feedback.NewModule` exposes `feedback_submit` with typed validation, consent-aware contact capture, and pluggable `MemorySink` / JSONL file sinks.
</roadmap-item>

<roadmap-item id="P42-2" package="feedback" status="complete">
Anonymous telemetry — `feedback.TelemetryCollector` requires explicit opt-in, aggregates package/tool usage counts and error rates, and exposes registry middleware plus sorted snapshots.
</roadmap-item>

<roadmap-item id="P42-3" package="rdcycle" status="complete">
rdcycle integration — `WithFeedbackTelemetry` accepts dashboard JSON providers such as `feedback.TelemetryCollector` and folds feedback usage/error summaries into `rdcycle_scan` output, artifacts, and action items.
</roadmap-item>

<roadmap-item id="P42-4" package="feedback" status="complete">
Telemetry dashboard export — feedback telemetry snapshots now export stable dashboard JSON rows with target names, counts, error rates, timestamps, and an HTTP handler.
</roadmap-item>

</roadmap-phase>

<roadmap-phase id="P43" status="complete" name="Shared Embedding Primitives">

<roadmap-item id="P43-1" package="embedding" status="complete">
Sparse-vector and TF-IDF helpers — factor reusable tokenization, TF-IDF, cosine, and top-k ranking primitives from downstream repos without introducing dense-model dependencies.
</roadmap-item>

<roadmap-item id="P43-2" package="embedding" status="complete">
Embedding provider interface — define small interfaces for optional dense/vector providers while keeping sparse local defaults deterministic and dependency-light.
</roadmap-item>

<roadmap-item id="P43-3" package="embedding" status="complete">
Roadmap consolidation adapters — document migration paths for ralphglasses semantic cache/GraphRAG scaffolds and shielddd TF-IDF evidence search so consumers do not duplicate primitives.
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

- **Parallel streams**: P31 (session), P32 (stateless HTTP), and P33 (transport) are complete.
- **P34** (dual-SDK hardening) is independent of P31–P33 and can run in a separate stream.
- **Budget profiles**: Use `PersonalProfile` for P35–P36 (research-heavy, lower token budget); use `WorkAPIProfile` for remaining P34 implementation-heavy work.
- **Self-improvement**: `rdcycle_improve` runs every 10 cycles and may inject lessons into the next `rdcycle_schedule` spec.

---

## Dependency Layers (including planned packages)

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`, `embedding`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`, `lifecycle`, `eval`, `roadmap`, `session`, `transport`, `feedback`
- **Layer 3** (depend on Layer 2): `security`, `gateway`, `ralph`, `skills`, `a2a`, `rdcycle`
- **Layer 4** (depend on Layer 3): `orchestrator`, `handoff`, `workflow`, `bootstrap`

_Note: `session` and `transport` depend only on Layer 1 packages. `feedback` has no internal deps beyond `registry`; `embedding` is stdlib-only._

<!-- whiteclaw-rollout:start -->
## Whiteclaw-Derived Overhaul (2026-04-08)

This tranche applies the highest-value whiteclaw findings that fit this repo's real surface: engineer briefs, bounded skills/runbooks, searchable provenance, scoped MCP packaging, and explicit verification ladders.

### Strategic Focus
- Use whiteclaw patterns here as reusable framework features, not one-off repo-local patches.
- The best transfer is a productized explorer/front-door starter that downstream repos can adopt without rewriting transport and contract code.
- Keep public docs and verification aligned with the framework's role as shared infrastructure.

### Recommended Work
- [x] [Starter surface] Ship an opinionated explorer/front-door starter that covers catalog/search/schema/health for downstream repos. Implemented in `frontdoor/` package: `Module` with `New(reg, opts...)`, `WithPrefix`/`WithHealthChecker` options, and four TypedHandler tools (`tool_catalog`, `tool_search`, `tool_schema`, `server_health`). 15 unit tests, race-clean.
- [x] [Docs] Publish a migration guide showing when to use `.mcp.json`, a discovery-first contract layer, or a standalone sidecar package.
- [x] [Verification] Add a transport and launcher smoke matrix for the public examples and starter surfaces.
- [ ] [Typed boundaries] Keep new tool/command/workflow surfaces on typed contracts rather than handwritten JSON-RPC or loose maps.
- [x] [Public examples] Expand example coverage — examples/pagination/ demonstrates Paginate + TruncateResult + SchemaFirstResult composition on a 500-row synthetic catalog. examples/frontdoor/ demonstrates `frontdoor.New(reg, WithPrefix("fd_"), WithHealthChecker(chk))` on a tiny inventory demo, giving downstream repos a copy-paste-ready integration reference.

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
- [x] [P2][M] Bounded-write safety middleware — Stripe-style confirmation pattern for MCP tools with financial/destructive side effects. Tool declares `confirm_required: true`, middleware intercepts and requires explicit `confirm` param. Ref: stripe/ai agent-toolkit (1.5K stars). Implemented in `middleware/boundedwrite`: `Middleware()`, `RequireConfirmation()`, `ConfirmTag` constant, 10 unit tests, and `examples/bounded-write` runnable server.
- [ ] [P2][L] Performance benchmarks — mcpkit middleware chain overhead vs raw mcp-go, p99 latency per middleware layer, throughput under load. Reference threshold: no single middleware layer may add >5% p99 latency
- [x] [P2][M] Token-efficient schema-first patterns — `handler/pagination.go` provides `Paginate[T]` (generic cursor-based paging), `TruncateResult` (byte-budget enforcement), `SchemaFirstResult` (deferred-data closure pattern). 16 unit tests. Ref: bytebase/dbhub (2.6K stars)
- [x] [P2][M] Explorer/front-door starter — `frontdoor` package wraps an existing `registry.ToolRegistry` with four discovery tools (`tool_catalog`, `tool_search`, `tool_schema`, `server_health`), with optional name prefix and health.Checker wiring. Downstream servers register it with one line and get a consistent explorer UX without rewriting catalog/search plumbing. 15 unit tests.

<!-- whiteclaw-rollout:end -->
---

## Crosspollinate Suggestion: Adopt go-mcp-server pattern

> **Source:** `~/hairglasses-studio/crosspollinate/patterns/go-mcp-server.md`
> **Proposed:** 2026-05-07 (cycle 0, refined cycle 13)
> **How to dismiss:** delete this section. Future crosspollinate cycles will detect the deletion and downgrade the recommendation.
> **Updated 2026-05-08:** cluster members reduced post-cycle-28 consolidate-repos: go-mcp-servers (was 12-member; now 8 active after process-mcp/geminikit/mcp-catalog/terraform-docs hard-deleted, systemd-mcp/tmux-mcp archive-only). See `crosspollinate/patterns/<topic>.md` for the current canonical active list.

The crosspollinate loop synthesized a canonical pattern for Go MCP servers across the 12-member cluster (hg-mcp, process-mcp, github-runner-mcp, systemd-mcp, tmux-mcp, codexkit, geminikit, jobb, mcp-catalog, terraform-docs, jellyfin-mcp-deluxe, mcpkit) based on context7 docs (mcp-go + official Go SDK + MCP spec) and exemplar code in ralphglasses.

Key recommendations relevant to this repo:

- **Dual-SDK build tags** with separate handler files (`handler_mcpgo.go` vs `handler_officialsdk.go`) — the two SDK signatures differ and cannot share handler bodies.
- **mcp-go error pattern**: validation/business errors → `mcp.NewToolResultError(msg), nil`; system errors → `nil, fmt.Errorf(...)`. Three cases, not one.
- **Deferred-loading tool group registry** instead of eager registration. Keeps cold-start memory bounded.
- **Discovery surfaces are MCP resources**, not tools (`<server>:///catalog/server`).

See the pattern doc for the full `# Adoption checklist` and `# Anti-patterns` sections.
