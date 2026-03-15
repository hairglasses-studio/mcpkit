# mcpkit Roadmap

Last updated: 2026-03-15.

## Status Summary

- **Spec coverage**: 100% (all MCP 2025-11-25 features implemented)
- **Tier 1**: Complete (resources, prompts, examples pending, SDK migration path pending, streamable HTTP tests pending)
- **Tier 2**: Complete (dispatcher, OAuth, sampling, logging, observability tests pending)
- **Tier 3**: Complete (discovery, DPoP, gateway, ralph, workload identity)
- **Tier 4**: New — ecosystem leadership items (see below)

See [RESEARCH.md](RESEARCH.md) for detailed analysis and evidence.

---

## Tier 3 (Complete)

All Tier 3 items delivered: discovery, DPoP, gateway, ralph enhancements, workload identity.

---

## Tier 4: Ecosystem Leadership

| # | Item | Description | Effort | New Package |
|---|------|-------------|--------|-------------|
| T4-1 | **A2A Protocol Bridge** | Google A2A v0.3 compliance — Agent Cards, task lifecycle, push notifications, gRPC transport | High (3wk) | `a2a/` |
| T4-2 | **Multi-Agent Orchestrator** | Go-native patterns: fan-out/fan-in, pipeline, swarm mesh, hierarchical delegation | High (3wk) | `orchestrator/` |
| T4-3 | **Agent Memory Registry** | Episodic/semantic/procedural memory tiers, pluggable storage backends, thread-safe registry | Medium (2wk) | `memory/` |
| T4-4 | **Workflow Engine** | Cyclical graph execution, state machines, conditional branching, YAML definitions | High (3wk) | `workflow/` |
| T4-5 | **Extensions Framework** | MCP Extensions negotiation, capability handshake, version-gated features | Medium (2wk) | `extensions/` |
| T4-6 | **Agent Handoff Protocol** | Manager/agent-as-tool + peer handoff patterns (OpenAI Agents SDK style) | Medium (2wk) | `handoff/` |
| T4-7 | **Skills & Context-Aware Loading** | Skill bundles, context-aware lazy loading, ~95% context reduction | Medium (10d) | `skills/` |
| T4-8 | **FinOps / Cost Tracking** | Token accounting per tool/sampling/agent/workflow, budget policies, Prometheus export | Low (1wk) | `finops/` |
| T4-9 | **Agent Bootstrap Framework** | Workspace init, context reports, capability matrix, multi-session state | Medium (10d) | `bootstrap/` |

---

## Implementation Phases

### Phase 5: Ralph Evolution + Foundations (Weeks 1-3)

- All ralph enhancements (multi-tool, cost, hooks, validation, DAG, resume, templates)
- `extensions/` package (foundation for protocol evolution)
- `memory/` package (foundation for agent state)
- `finops/` package (cross-cutting cost middleware)

**Parallelization**: 3 agents — ralph-agent, memory-agent, extensions-finops-agent.

### Phase 6: Multi-Agent + A2A (Weeks 4-7) — COMPLETE

- `a2a/` package (A2A v0.3 compliance) — **deferred** (spec not stable, no internal consumer)
- ~~`orchestrator/` package~~ (fan-out, pipeline, select) — **DONE**
- ~~`handoff/` package~~ (manager + peer patterns, AgentAsTool) — **DONE**
- ~~`skills/` package~~ (context-aware lazy loading, triggers, middleware) — **DONE**

### Phase 7: Workflow Engine + Bootstrap + Polish (Weeks 8-10) — COMPLETE

- ~~`workflow/` package~~ (cyclical graph engine, checkpoints, adapters) — **DONE**
- ~~`extensions/` package~~ (capability negotiation, offer/accept/reject) — **DONE**
- ~~`bootstrap/` package~~ (context reports, capability matrix, formatting) — **DONE**
- ~~`auth/workload.go`~~ (workload identity — GCP metadata + AWS IMDSv2) — **DONE**

### Phase 8: Security Hardening — COMPLETE

- ~~Output sanitization middleware~~ (`sanitize/output.go`, `sanitize/patterns.go`) — secret/PII/injection redaction — **DONE**
- ~~URI validation~~ (`sanitize/uri.go`, `resources/uri_middleware.go`) — path traversal + SSRF protection — **DONE**
- ~~Tool integrity verification~~ (`registry/integrity.go`) — SHA-256 fingerprinting, tamper detection — **DONE**
- ~~Tenant context propagation~~ (`security/tenant.go`) — multi-tenant identity middleware — **DONE**

### Phase 9: Testing Infrastructure + FinOps v2 — COMPLETE

- ~~Session replay~~ (`mcptest/replay.go`) — record/save/load/replay MCP sessions — **DONE**
- ~~Snapshot testing~~ (`mcptest/snapshot.go`) — golden file assertions with UPDATE_SNAPSHOTS — **DONE**
- ~~Benchmark helpers~~ (`mcptest/benchmark.go`) — BenchmarkTool, BenchmarkToolParallel, BenchmarkSuite — **DONE**
- ~~Dollar-cost estimation~~ (`finops/cost.go`) — ModelPricing, CostPolicy, dollar budgets — **DONE**
- ~~Scoped budgets~~ (`finops/scope.go`) — per-tenant/user/session budget tracking — **DONE**
- ~~Time-windowed tracking~~ (`finops/window.go`) — lazy rotation, hourly/daily/weekly/monthly windows — **DONE**
- ~~Metadata enhancement~~ (`discovery/metadata.go`) — MetadataFromConfig with resources + prompts extraction — **DONE**
- ~~Publish convenience~~ (`discovery/publisher.go`) — Publish/Unpublish one-call wrappers — **DONE**

### Phase 10: Production Hardening — COMPLETE

- ~~Gateway resilience~~ (`gateway/resilience.go`) — per-upstream circuit breaker, rate limiter, call timeout via UpstreamPolicy — **DONE**
- ~~Orchestrator middleware~~ (`orchestrator/middleware.go`) — StageMiddleware, WrapStage, WrapStages — **DONE**
- ~~Handoff middleware~~ (`handoff/middleware.go`) — DelegateMiddleware, WrapDelegate, Config.WithMiddleware — **DONE**
- ~~Workflow middleware~~ (`workflow/middleware.go`) — NodeMiddleware, WrapNodeFunc, EngineConfig.NodeMiddleware — **DONE**

### Phase 11: DX Sprint — COMPLETE

- ~~README overhaul~~ — complete rewrite reflecting 30+ packages, 100% spec coverage — **DONE**
- ~~Godoc examples~~ — 14 `example_test.go` files with 28 runnable `Example*` functions — **DONE**
- ~~Package doc comments~~ — added to orchestrator, workflow, dispatcher, extensions, health, mcptest, ralph, roots, sampling, skills, gateway — **DONE**

### Phase 12: Observability Integration + Lifecycle — COMPLETE

- ~~Gateway tracing~~ (`gateway/observability.go`) — TracingMiddleware with upstream/tool attributes — **DONE**
- ~~Orchestrator tracing~~ (`orchestrator/observability.go`) — TracingMiddleware for stage spans — **DONE**
- ~~Workflow tracing~~ (`workflow/observability.go`) — TracingMiddleware for node spans — **DONE**
- ~~Server lifecycle~~ (`lifecycle/lifecycle.go`) — signal handling, graceful drain, LIFO shutdown hooks — **DONE**
- ~~Health readiness~~ (`health/health.go`) — SetStatus/IsReady, 503 on draining — **DONE**

### Phase 13: Integration Completeness — COMPLETE

- ~~FinOps→Observability bridge~~ (`finops/context.go`, `observability/middleware.go`) — mutable TokenUsageHolder pattern so inner finops middleware propagates token counts back to outer observability spans — **DONE**
- ~~Handoff tracing~~ (`handoff/observability.go`) — TracingMiddleware(tracer) DelegateMiddleware with agent/status/duration attributes — **DONE**
- ~~Ralph tracing~~ (`ralph/observability.go`) — TracingHooks(tracer) with per-iteration spans, task ID, error recording — **DONE**
- ~~Full example rewrite~~ (`examples/full/main.go`) — demonstrates lifecycle, observability, finops, sanitize, logging, health with correct middleware ordering — **DONE**

### Phase 14: Server Cards, Tool Signing, Eval Framework — COMPLETE

- ~~Server cards~~ (`discovery/wellknown.go`) — `ServerCardHandler` and `StaticServerCardHandler` for `.well-known/mcp.json` — **DONE**
- ~~Tool versioning~~ (`registry/registry.go`, `discovery/discovery.go`) — `Version` field on ToolDefinition and ToolSummary — **DONE**
- ~~Tool signing~~ (`registry/signing.go`, `registry/signing_middleware.go`) — Ed25519 `SignTool`/`VerifyToolSignature`, `SignatureStore`, `SignatureVerificationMiddleware` — **DONE**
- ~~Eval framework~~ (`eval/`) — `Case`, `Suite`, `Summary`, `Scorer` interface, 6 built-in scorers, `Run`/`RunT` runner — **DONE**

### Phase 15: Audit Export, Eval Hardening, Godoc Examples — COMPLETE

- ~~Audit export~~ (`security/export.go`) — `AuditExporter` interface, `JSONLExporter` (JSONL to io.Writer), `StreamExporter` (filtered export), integrated into `AuditLogger.Log()` — **DONE** (T5-4)
- ~~Eval hardening~~ (`eval/loader.go`, `eval/scorers.go`, `eval/runner.go`) — `LoadSuiteJSON`, `NotEmpty` scorer, `ResultScorer` interface, `Latency` scorer — **DONE**
- ~~Godoc examples~~ — `eval/example_test.go`, `registry/signing_example_test.go`, `discovery/wellknown_example_test.go` — **DONE**

### Phase 16: Security Middleware Tests, Eval Completeness, Godoc Examples Batch 2 — COMPLETE

- ~~Security middleware tests~~ (`security/middleware_test.go`) — 8 tests covering AuditMiddleware and RBACMiddleware (allow, deny, nil logger, combined stack) — **DONE**
- ~~Security examples~~ (`security/example_test.go`) — ExampleNewRBAC, ExampleAuditMiddleware, ExampleNewAuditLogger_withExporter — **DONE**
- ~~ErrorRate scorer~~ (`eval/scorers.go`) — ResultScorer that passes (1.0) on non-error, fails (0.0) on error — **DONE**
- ~~Runner ResultScorer test~~ (`eval/runner_test.go`) — end-to-end test exercising both Scorer and ResultScorer in a suite — **DONE**
- ~~Godoc examples batch 2~~ — `auth/`, `observability/`, `sanitize/`, `health/`, `lifecycle/`, `logging/` example_test.go files — **DONE**

### Phase 17: Godoc Examples Batch 3 + Dispatcher/Sampling Test Coverage — COMPLETE

- ~~Godoc examples batch 3~~ — `dispatcher/`, `ralph/`, `roots/`, `sampling/`, `secrets/`, `bootstrap/`, `client/`, `extensions/`, `research/` example_test.go files — **DONE**
- ~~Dispatcher unit tests~~ — `groups_test.go`, `job_test.go`, `middleware_test.go`, `queue_test.go` for internal coverage — **DONE**
- ~~Sampling unit tests~~ — `middleware_test.go`, `helpers_test.go` for internal coverage — **DONE**

### Phase 18: Core Package Test Coverage — COMPLETE

- ~~Registry tests~~ — `search_test.go` (14), `dynamic_test.go` (21), `deferred_test.go` (14), `annotations_test.go` (12) — **DONE**
- ~~Workflow tests~~ — `engine_test.go` (17), `checkpoint_test.go`, `hooks_test.go` — **DONE**
- ~~Auth tests~~ — `pkce_test.go`, `context_test.go`, `metadata_test.go`, `config_test.go` — **DONE**
- ~~Discovery examples~~ — `example_test.go` (5 Example functions) — **DONE**
- ~~mcptest tests~~ — `assert_test.go`, `recorder_test.go` — **DONE**

### Phase 19: Remaining Test Coverage — COMPLETE

- ~~Discovery tests~~ — `client_test.go` (13), `publisher_test.go` (12) — httptest mocks, caching, error mapping — **DONE**
- ~~Handler tests~~ — `result_test.go` (22), `structured_test.go` (18) — all result builders — **DONE**
- ~~Memory tests~~ — `store_mem_test.go` (19) — full InMemoryStore lifecycle, concurrent safety — **DONE**
- ~~FinOps tests~~ — `tracker_test.go` (9), `estimate_test.go` (12) — tracker lifecycle, estimation — **DONE**
- ~~Resilience tests~~ — `middleware_test.go` (9) — rate limit + circuit breaker middleware — **DONE**

### Decision Points

- **After Phase 5**: Evaluate A2A spec stability — if v1.0 ships, fast-track `a2a/`; otherwise prototype only
- **After Phase 6**: Assess orchestrator patterns against real-world usage from hg-mcp/jobb migrations
- **After Phase 7**: Re-evaluate WebMCP bridge and Chrome integration based on adoption signals

---

## Tier 5: Bleeding Edge

| # | Item | Description | Priority |
|---|------|-------------|----------|
| T5-1 | **June 2026 Spec Prep** | Stateless HTTP, session management, server cards, tool versioning | High |
| T5-2 | ~~**Agent Evaluation**~~ | Go-native eval framework (benchmark tool accuracy, latency, cost) | ~~High~~ **DONE** |
| T5-3 | ~~**Tool Signing**~~ | Ed25519 signatures on ToolDefinition, registry-level verification | ~~Medium~~ **DONE** |
| T5-4 | ~~**SIEM/Audit Export**~~ | Pluggable AuditExporter interface, JSONL + stream exporters | ~~Medium~~ **DONE** |
| T5-5 | ~~**Server Cards**~~ | `.well-known/mcp.json` generation from registry metadata | ~~Medium~~ **DONE** |
| T5-6 | **WebMCP Bridge** | Browser transport adapter (when spec stabilizes) | Low |
| T5-7 | **A2A Bridge** | Google A2A v1.0 (when spec ships) | Low |
| T5-8 | **Temporal Integration** | Durable execution adapter for workflow engine | Low |

---

## Updated Dependency Layers (post-Phase 14)

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`, `lifecycle`, `eval`
- **Layer 3** (depend on Layer 2): `security`, `gateway`, `ralph`, `skills`, `a2a`
- **Layer 4** (depend on Layer 3): `orchestrator`, `handoff`, `workflow`, `bootstrap`
