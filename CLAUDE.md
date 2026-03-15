# mcpkit

MCP toolkit for building production-grade MCP servers. Built on `github.com/mark3labs/mcp-go`.

## Commands

```bash
go build ./...           # Build all packages
go vet ./...             # Static analysis
go test ./... -count=1   # Run all tests (no cache)
make check               # All three above
make build-official      # Verify official SDK build
make check-dual          # Full check + official SDK build
```

## Package Map

| Package | Purpose | Internal Deps |
|---------|---------|---------------|
| `registry` | Tool registration, middleware chain, server integration, tool integrity verification | none |
| `handler` | TypedHandler generics, param extraction, result builders, elicitation | `registry` |
| `resilience` | CircuitBreaker, RateLimiter, CacheEntry generics, middleware | `registry` |
| `mcptest` | Test server/client, assertion helpers, HTTP pool, session replay, snapshot testing, benchmark helpers | `registry` |
| `auth` | JWT/JWKS validation, OAuth discovery + client flow, Bearer middleware, DPoP proof validation + HTTP middleware, workload identity (GCP/AWS), context identity | `registry`, `client` |
| `security` | RBAC, audit logging middleware, audit export (JSONL/stream), tenant context propagation | `registry`, `auth` |
| `health` | Health check endpoint and checker registry | none |
| `observability` | OpenTelemetry tracing/metrics middleware | `registry` |
| `sanitize` | Input/output sanitization, secret/PII redaction, URI validation | none |
| `secrets` | Secret provider interface, env/file providers, sanitizer | none |
| `client` | HTTP pool and client utilities | none |
| `discovery` | MCP Registry client for server discovery and publishing, multi-registry metadata extraction, server card HTTP handler | `registry`, `client`, `resources`, `prompts` |
| `resources` | Resource registry, middleware chain, server integration for URI-based data, URI validation middleware | `registry` |
| `prompts` | Prompt registry, middleware chain, server integration for reusable templates | `registry` |
| `logging` | slog.Handler bridge to MCP clients, tool invocation logging middleware | `registry` |
| `sampling` | Sampling client interface, context injection middleware, request builders | `registry` |
| `roots` | Client workspace root discovery, caching, context helpers | `registry` |
| `research` | MCP ecosystem monitoring and viability assessment tools, GitHub activity monitoring, diff analysis | `registry`, `handler`, `client` |
| `gateway` | Multi-server aggregation with namespaced tool routing, per-upstream resilience (circuit breaker, rate limit, timeout), dynamic upstream registration | `registry`, `client`, `resilience` |
| `dispatcher` | Priority worker pool with concurrency groups, middleware integration | `registry` |
| `ralph` | Autonomous loop runner for iterative task execution (Ralph Loop pattern), workflow-backed loop | `registry`, `handler`, `sampling`, `finops`, `workflow` |
| `finops` | Token accounting, budget policies, usage tracking middleware, dollar-cost estimation, scoped budgets, time-windowed tracking | `registry` |
| `memory` | Agent memory registry with pluggable storage backends | `registry` |
| `skills` | Context-aware lazy tool loading with skill bundles and triggers | `registry` |
| `handoff` | Agent delegation protocol with manager/agent-as-tool patterns, delegate middleware | `registry`, `sampling`, `finops` |
| `orchestrator` | Multi-agent execution patterns: fan-out, pipeline, select, stage middleware | none |
| `workflow` | Cyclical graph engine with conditional branching, checkpoints, state machines, node middleware, fork nodes for parallel branches, compensation/saga rollback | `orchestrator`, `registry`, `sampling` |
| `extensions` | MCP Extensions negotiation and capability handshake | none |
| `lifecycle` | Production server lifecycle: signal handling, graceful drain, shutdown hooks | none |
| `bootstrap` | Agent workspace init, context reports, capability matrix | `registry`, `resources`, `prompts`, `extensions` |
| `eval` | Evaluation framework: cases, scorers (exact/contains/regex/jsonpath/custom/not-empty/latency), JSON suite loading, runner | `registry` |
| `roadmap` | Machine-readable roadmap management, XML-tagged markdown rendering, gap analysis, query functions | `registry`, `handler` |
| `rdcycle` | R&D cycle orchestration tools: scan, plan, verify, commit, report, schedule, notes, improve, workflow graph, budget profiles, model tiers | `registry`, `handler`, `research`, `roadmap`, `workflow`, `finops` |

## Dependency Layers

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`, `lifecycle`, `eval`, `roadmap`
- **Layer 3** (depend on Layer 2): `security`, `gateway`, `ralph`, `skills`, `a2a`, `rdcycle`
- **Layer 4** (depend on Layer 3): `orchestrator`, `handoff`, `workflow`, `bootstrap`

## Coding Conventions

- Middleware signature: `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`
- Error codes: `handler.CodedErrorResult(handler.ErrInvalidParam, err)` — codes defined in `handler/result.go`
- Param extraction: `handler.GetStringParam(req, "name")`, `handler.GetIntParam(req, "name", default)`
- Result builders: `handler.TextResult()`, `handler.JSONResult()`, `handler.ErrorResult()`, `handler.StructuredResult()`
- Thread safety: all registries use `sync.RWMutex` — `RLock` for reads, `Lock` for writes
- SDK compat: import MCP types through `registry/compat.go` aliases when building tool modules
- Dual-SDK: `//go:build !official_sdk` tags on mcp-go specific files; `//go:build official_sdk` for go-sdk variants
- Adapter functions: use `registry.MakeTextContent()`, `registry.MakeErrorResult()`, `registry.ExtractArguments()` instead of SDK-specific constructors

## Testing

- Test files: `*_test.go` in same package
- Use `mcptest.NewServer()` for integration tests, stdlib `testing` for unit tests
- Assertions: `mcptest/assert.go` helpers or stdlib `t.Errorf`/`t.Fatalf`
- Each package's tests must pass in isolation: `go test ./handler/ -count=1`

## Parallel Agent Rules

- **One agent per package** — don't edit files in another agent's package directory
- Import direction: lower layers never import upper layers (see Dependency Layers)
- Feature branches: each agent works on its own branch, merged after CI passes
- New package checklist: add CLAUDE.md if complex, ensure `_test.go` files exist, follow middleware signature if applicable

## Roadmap

Current spec coverage: **100%** (all MCP 2025-11-25 features implemented).
All Tier 1 and Tier 2 roadmap items complete. Tier 3 mostly complete.

See [ROADMAP.md](ROADMAP.md) for detailed phased plan and [RESEARCH.md](RESEARCH.md) for full analysis.

### Phase 5 — Ralph Evolution + Foundations (COMPLETE)
- ~~Ralph enhancements~~ (multi-tool, cost, hooks, validation, DAG, resume, streaming, templates)
- ~~`extensions/`~~ — MCP Extensions negotiation and capability handshake
- ~~`memory/`~~ — Agent memory registry with pluggable storage backends
- ~~`finops/`~~ — Token accounting, budget policies, Prometheus export

### Phase 6 — Multi-Agent + A2A (COMPLETE)
- `a2a/` — Google A2A v0.3 protocol bridge (deferred — spec not stable)
- ~~`orchestrator/`~~ — Fan-out/fan-in, pipeline, select patterns
- ~~`handoff/`~~ — Manager/agent-as-tool + peer delegation
- ~~`skills/`~~ — Context-aware lazy tool loading

### Phase 7 — Workflow Engine + Bootstrap (COMPLETE)
- ~~`workflow/`~~ — Cyclical graph engine, state machines, checkpoints
- ~~`extensions/`~~ — MCP Extensions negotiation and capability handshake
- ~~`bootstrap/`~~ — Agent workspace init, context reports
- ~~`auth/workload.go`~~ — Workload identity (GCP/AWS IAM)

### Phase 8 — Security Hardening (COMPLETE)
- ~~`sanitize/output.go`~~ — Output sanitization middleware (secret/PII/injection redaction)
- ~~`sanitize/uri.go`~~ — URI validation (path traversal, SSRF protection)
- ~~`registry/integrity.go`~~ — Tool integrity verification (SHA-256 fingerprinting, tamper detection)
- ~~`resources/uri_middleware.go`~~ — URI validation middleware for resource handlers
- ~~`security/tenant.go`~~ — Tenant context propagation for multi-tenant servers

### Phase 9 — Testing Infrastructure + FinOps v2 (COMPLETE)
- ~~`mcptest/replay.go`~~ — Session record/replay for regression testing
- ~~`mcptest/snapshot.go`~~ — Golden file snapshot assertions
- ~~`mcptest/benchmark.go`~~ — Tool benchmark helpers (sequential, parallel, suite)
- ~~`finops/cost.go`~~ — Dollar-cost estimation with model pricing
- ~~`finops/scope.go`~~ — Per-tenant/user/session scoped budget tracking
- ~~`finops/window.go`~~ — Time-windowed tracking with lazy rotation
- ~~`discovery/metadata.go`~~ — MetadataFromConfig with resources + prompts extraction

### Phase 10 — Production Hardening (COMPLETE)
- ~~`gateway/resilience.go`~~ — Per-upstream circuit breaker, rate limiter, call timeout
- ~~`orchestrator/middleware.go`~~ — StageMiddleware, WrapStage, WrapStages
- ~~`handoff/middleware.go`~~ — DelegateMiddleware, WrapDelegate, Config.WithMiddleware
- ~~`workflow/middleware.go`~~ — NodeMiddleware, WrapNodeFunc, EngineConfig.NodeMiddleware

### Phase 11 — DX Sprint (COMPLETE)
- ~~README.md~~ — Complete rewrite (30+ packages, 100% spec coverage)
- ~~14 example_test.go files~~ — 28 runnable Example* functions across all key packages
- ~~Package doc comments~~ — Added to 10+ packages

### Phase 12 — Observability Integration + Lifecycle (COMPLETE)
- ~~`gateway/observability.go`~~ — TracingMiddleware with upstream/tool span attributes
- ~~`orchestrator/observability.go`~~ — TracingMiddleware for stage spans
- ~~`workflow/observability.go`~~ — TracingMiddleware for node spans
- ~~`lifecycle/`~~ — Server lifecycle manager with signal handling, graceful drain, LIFO shutdown hooks
- ~~`health/health.go`~~ — SetStatus, IsReady, 503 on draining for Kubernetes readiness

### Phase 13 — Integration Completeness (COMPLETE)
- ~~`finops/context.go`~~ — TokenUsageHolder mutable bridge for finops→observability span attributes
- ~~`observability/middleware.go`~~ — Reads from TokenUsageHolder (inner finops) with context fallback
- ~~`handoff/observability.go`~~ — TracingMiddleware for delegation spans (agent, status, duration)
- ~~`ralph/observability.go`~~ — TracingHooks for iteration spans (iteration, task_id, status)
- ~~`examples/full/main.go`~~ — Full middleware stack demo (lifecycle, observability, finops, sanitize, logging)

### Phase 14 — Server Cards, Tool Signing, Eval Framework (COMPLETE)
- ~~`discovery/wellknown.go`~~ — ServerCardHandler + StaticServerCardHandler for `.well-known/mcp.json`
- ~~`registry/registry.go`~~ — `Version` field on ToolDefinition; `discovery/discovery.go` — Version on ToolSummary
- ~~`registry/signing.go`~~ — Ed25519 SignTool/VerifyToolSignature, SignatureStore, SignatureVerificationMiddleware
- ~~`eval/`~~ — Evaluation framework: Case, Suite, Summary, 6 built-in Scorers, Run/RunT runner

### Phase 15 — Audit Export, Eval Hardening, Godoc Examples (COMPLETE)
- ~~`security/export.go`~~ — AuditExporter interface, JSONLExporter, StreamExporter with FilterFunc (T5-4)
- ~~`eval/loader.go`~~ — LoadSuiteJSON for JSON suite loading
- ~~`eval/scorers.go`~~ — NotEmpty scorer, ResultScorer interface, Latency scorer
- ~~Godoc examples~~ — eval, registry/signing, discovery/wellknown example_test.go files

### Phase 16 — Security Middleware Tests, Eval Completeness, Godoc Examples Batch 2 (COMPLETE)
- ~~`security/middleware_test.go`~~ — 8 tests for AuditMiddleware and RBACMiddleware
- ~~`security/example_test.go`~~ — ExampleNewRBAC, ExampleAuditMiddleware, ExampleNewAuditLogger_withExporter
- ~~`eval/scorers.go`~~ — ErrorRate ResultScorer (passes on non-error, fails on error)
- ~~`eval/runner_test.go`~~ — End-to-end TestRun_WithResultScorers exercising both Scorer and ResultScorer
- ~~Godoc examples~~ — auth, observability, sanitize, health, lifecycle, logging example_test.go files

### Phase 17 — Godoc Examples Batch 3 + Dispatcher/Sampling Test Coverage (COMPLETE)
- ~~Godoc examples batch 3~~ — dispatcher, ralph, roots, sampling, secrets, bootstrap, client, extensions, research example_test.go files
- ~~Dispatcher unit tests~~ — groups_test.go, job_test.go, middleware_test.go, queue_test.go
- ~~Sampling unit tests~~ — middleware_test.go, helpers_test.go

### Phase 18 — Core Package Test Coverage (COMPLETE)
- ~~Registry tests~~ — search_test.go (14), dynamic_test.go (21), deferred_test.go (14), annotations_test.go (12)
- ~~Workflow tests~~ — engine_test.go (17), checkpoint_test.go, hooks_test.go
- ~~Auth tests~~ — pkce_test.go, context_test.go, metadata_test.go, config_test.go
- ~~Discovery examples~~ — example_test.go (5 Example functions)
- ~~mcptest tests~~ — assert_test.go, recorder_test.go

### Phase 19 — Remaining Test Coverage (COMPLETE)
- ~~Discovery tests~~ — client_test.go (13), publisher_test.go (12) — httptest mocks, caching, error mapping
- ~~Handler tests~~ — result_test.go (22), structured_test.go (18) — all result builders covered
- ~~Memory tests~~ — store_mem_test.go (19) — full InMemoryStore lifecycle, concurrent safety
- ~~FinOps tests~~ — tracker_test.go (9), estimate_test.go (12) — tracker lifecycle, estimation formulas
- ~~Resilience tests~~ — middleware_test.go (9) — rate limit + circuit breaker middleware

### Phase 20 — Structured Roadmap + R&D Cycle Tools (COMPLETE)
- ~~`roadmap/`~~ — Machine-readable roadmap types, XML-tagged markdown, gap analysis, query functions, tool module
- ~~`research/github.go`~~ — GitHub activity monitoring tool (commits, issues, releases)
- ~~`research/diff.go`~~ — Diff analysis tool for comparing SummaryOutput snapshots
- ~~`rdcycle/`~~ — R&D cycle orchestration: scan, plan, verify, artifacts tools + InMemoryArtifactStore

### Phase 21 — Workflow Graph + Parallel Branches (COMPLETE)
- ~~`workflow/fork.go`~~ — AddForkNode with parallel goroutine branches, MergeFunc, MergeAll, MergeKeyed
- ~~`rdcycle/workflow.go`~~ — NewRDCycleGraph: scan→plan→gate→implement→verify→gate_quality→END
- ~~`rdcycle/specs/rd_cycle.json`~~ — Ralph Spec template with TemplateVars for autonomous R&D

### Phase 22 — Close the Loop: Self-Updating + Example (COMPLETE)
- ~~`rdcycle/commit.go`~~ — rdcycle_commit tool (git staging, commit, branch safety)
- ~~`rdcycle/report.go`~~ — rdcycle_report tool (RESEARCH-*.md generation)
- ~~`rdcycle/schedule.go`~~ — rdcycle_schedule tool (next cycle spec generation)
- ~~`ralph/workflow.go`~~ — WorkflowLoop bridging ralph lifecycle with workflow.Engine
- ~~`examples/rdcycle/main.go`~~ — Full R&D cycle example: research+roadmap+rdcycle→workflow→WorkflowLoop

### Phase 23 — Workflow Compensation + Dynamic Gateway (COMPLETE)
- ~~`workflow/compensate.go`~~ — Saga/compensation pattern: CompensateFunc, CompensationStack (LIFO), AddCompensableNode, engine integration with CompensateOnFailure
- ~~`gateway/dynamic.go`~~ — DynamicUpstreamRegistry: runtime add/remove upstreams, default policy, lifecycle hooks

### Phase 24 — Autonomous Loop Guardrails + Self-Improvement (COMPLETE)
- ~~`rdcycle/module.go`~~ — Register missing tools (commit, report, schedule) + new tools (notes, improve)
- ~~`rdcycle/profiles.go`~~ — BudgetProfile presets (Personal/WorkAPI), BuildFinOpsStack composing Tracker+CostPolicy+WindowedTracker
- ~~`rdcycle/models.go`~~ — ModelTierConfig with task-phase-aware model selection
- ~~`ralph/ralph.go`~~ — ModelSelector on Config for per-iteration model hints
- ~~`ralph/loop.go`~~ — Wire ModelSelector into sampling request options
- ~~`rdcycle/notes.go`~~ — ImprovementNote persistence, rdcycle_notes tool for per-cycle reflection
- ~~`rdcycle/improve.go`~~ — rdcycle_improve tool: pattern analysis, cost trends, budget suggestions
- ~~`rdcycle/schedule.go`~~ — Inject lessons learned from past notes, conditional self_improve task every 10 cycles
- ~~`rdcycle/specs/rd_cycle.json`~~ — Added reflect task between verify and report

### Phase 25 — Coverage Hardening + Godoc Completeness (COMPLETE)
- ~~`secrets/sanitize_test.go`~~ — 67% → 91.9% coverage: IsSensitiveKey, MaskValue, Sanitize, SanitizeString, SanitizeHeaders, RedactedString, SecureCompare
- ~~`handler/examples_test.go`~~ — 72% → 88.5% coverage: FormatExamples, formatKV, intToStr, floatToStr, formatAny
- ~~`ralph/prompt_test.go`~~ — 79% → 85.2% coverage: buildIterationPrompt (blocked/completed/activity/truncation/tools/deps)
- ~~`rdcycle/example_test.go`~~ — Godoc examples: ExampleNewModule, ExampleNewInMemoryArtifactStore, ExamplePersonalProfile
- ~~`roadmap/example_test.go`~~ — Godoc examples: ExampleLoadRoadmap, ExampleGapAnalysis, ExampleNextPhase

### Phase 26 — Final Coverage Hardening (COMPLETE)
- ~~`observability/middleware_test.go`~~ — 71.3% → 89.3% coverage: addGenAISpanAttrs, Init branches, error paths, TokenUsageHolder
- ~~`mcptest/replay_test.go`~~ — 75.2% → 87.4% coverage: resultsMatch, resultToMap, SaveSession/LoadSession, Replay mismatches
- ~~`mcptest/snapshot_test.go`~~ — normaliseResult, timestamp stripping, structured content branches
- ~~`roots/server_test.go`~~ — 78.0% → 94.0% coverage: ServerRootsClient all branches, CachedClient error propagation, nil Middleware

### Phase 27 — Ralph Production Hardening: DAG Enforcement + YAML Specs (COMPLETE)
- ~~`ralph/loop.go`~~ — Server-side DAG enforcement: reject decisions targeting blocked tasks, guard MarkDone
- ~~`ralph/yaml.go`~~ — YAML spec support: LoadSpecYAML, ParseSpecYAML
- ~~`ralph/ralph.go`~~ — YAML struct tags, LoadSpec extension dispatch for .yaml/.yml
- ~~`ralph/template.go`~~ — YAML-aware RenderSpec with format detection
- ~~`roots/server_official.go`~~ — TODO cleanup: proper doc comment explaining official SDK limitation
- ~~`examples/gateway/main.go`~~ — Gateway example: DynamicUpstreamRegistry, per-upstream resilience, stdio

### Phase 28 — Package Documentation + Coverage Hardening + Examples (COMPLETE)
- ~~19× `*/doc.go`~~ — Package documentation for all major packages (registry, handler, resilience, auth, security, gateway, workflow, observability, mcptest, resources, prompts, logging, sampling, client, health, sanitize, secrets, eval, discovery)
- ~~`rdcycle/*_test.go`~~ — 81.6% → 88.6% coverage: schedule lessons-learned/self_improve, commit branch/refuse-master, report action items, notes defaults
- ~~`logging/handler_test.go`~~ — 83.5% → 98.7% coverage: addAttr KindGroup/inline/zero-value, WithAttrs(nil), WithGroup("")
- ~~`registry/compat_test.go`~~ — 84.9% → 94.4% coverage: ExtractResourceText, ExtractArguments, GetToolTaskSupport, HasTaskParams, ExtractTaskTTL, MakeStructuredResult
- ~~`registry/registry_test.go`~~ — SetMiddleware, ListToolsByRuntimeGroup, GetRuntimeGroupStats, GetToolCatalog
- ~~`registry/dynamic_test.go`~~ — RegisterFilteredWithServer
- ~~`examples/http/main.go`~~ — StreamableHTTP server with health, lifecycle, logging middleware
- ~~`examples/elicitation/main.go`~~ — ElicitForm, ElicitURL, ElicitFormSchema via stdio

### Phase 29 — Package Documentation Completion (COMPLETE)
- ~~13× `*/doc.go`~~ — Package documentation for remaining packages (dispatcher, extensions, finops, handoff, lifecycle, memory, orchestrator, ralph, rdcycle, research, roadmap, roots, skills)
- All 33 packages now have `doc.go` files with Go-convention package comments

### Phase 30 — Coverage Hardening: All Packages to 90%+ (COMPLETE)
- ~~`auth/*_test.go`~~ — 85.9% → 90.6%: OAuth/DPoP/workload identity/JWT edge cases (37 tests)
- ~~`eval/*_test.go`~~ — 85.7% → 96.4%: scorer errors, runner nil paths, suite loading (11 tests)
- ~~`ralph/*_test.go`~~ — 85.7% → 95.4%: DAG enforcement, YAML parse, module lifecycle, workflow loop
- ~~`research/*_test.go`~~ — 86.1% → 97.2%: GitHub API errors, diff edges, assess/summary branches
- ~~`gateway/*_test.go`~~ — 87.6% → 96.0%: dynamic upstream lifecycle, health loop, resilience (13 tests)
- ~~`mcptest/*_test.go`~~ — 87.4% → 90.0%: recorder, snapshot, benchmark edges
- ~~`secrets/providers/*_test.go`~~ — 87.9% → 96.4%: env/file provider edge cases (13 tests)
- ~~`handler/*_test.go`~~ — 88.5% → 96.6%: content annotations, type checks, schema generation
- ~~`rdcycle/*_test.go`~~ — 88.6% → 90.8%: improve recommendations, git helpers, workflow graph
- ~~`observability/*_test.go`~~ — 89.3% → 91.0%: Prometheus health endpoint
- ~~`roadmap/*_test.go`~~ — 89.6% → 94.5%: handler path overrides, gap analysis tiers
- All 35 non-example packages now at 90%+ coverage (min 90.0%, max 100.0%)
