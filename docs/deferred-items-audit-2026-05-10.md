# Deferred-Items Audit Findings (2026-05-10)

After today's mcpkit work shipped (PRs #17–#22, v0.6.0 tagged), the plan's "deferred to Q3" items were re-audited against actual code. **Three of the four supposedly multi-week features turned out to be already implemented**; only one remains genuinely outstanding.

## Findings

### D1 — RFC 9728 OAuth Protected Resource Metadata

**Plan said**: "1 week of new work."
**Actual**: ✅ **Already implemented.** `auth/metadata.go` defines `ProtectedResourceMetadata` and `MetadataHandler` serving `/.well-known/oauth-protected-resource`. Test coverage in `auth/metadata_test.go` + `auth/auth_test.go`. See [docs/auth-rfc-compliance.md](auth-rfc-compliance.md) §RFC 9728.

**Remaining work**: optional fields (DPoP signing-alg list, encryption-alg list, etc.) — deployment-specific, not blocking. **Reclassified as background.**

### C3 — Progress notification middleware

**Plan said**: "1 week new package `middleware/progress.go`."
**Actual**: ✅ **Already implemented.** `registry/progress.go` defines the `ProgressReporter` interface; `registry/progress_server.go` provides `ServerProgressReporter` + `ServerProgressMiddleware` that auto-injects a reporter into the handler context when the request includes a progress token. Official-SDK variant at `registry/progress_server_official.go`. Test coverage in `registry/progress_server_test.go` (140 LOC) + `registry/progress_test.go` (134 LOC).

**Remaining work**: none. Production-ready for any handler that wants `notifications/progress` per MCP spec 2025-11-25. **Reclassified as done.**

### C4 — Correlation ID middleware

**Plan said**: "3 days new package `middleware/correlation.go`."
**Actual**: ✅ **Shipped today as PR #22.** Decoupled from `middleware/debug` (which had correlation tracking coupled to debug logging). New `middleware/correlation` package provides Middleware + FromContext + LoggerFromContext, with crypto/rand-backed default IDs and a `GenerateID` override for forwarding upstream headers. Tests cover uniqueness, overrides, slog enrichment, and graceful fallback.

### F2 — Observability guide

**Plan said**: "4h new docs."
**Actual**: ✅ **Shipped today as PR #21** as `docs/observability-guide.md`. Documents the layered model (observability/ for OTel GenAI attrs, finops/ for budget accounting sharing the call context).

### D2 + D3 — RFC 9449 (DPoP) + RFC 8707 (Resource Indicators)

**Plan said**: "audit 4h + 2h."
**Actual**: ✅ **Shipped today as PR #21** as `docs/auth-rfc-compliance.md`. Findings: RFC 9449 fully compliant (auth/dpop.go family). RFC 8707 partial — accepted gap (resource binding handled via `aud` claim, not the explicit `resource` parameter — low priority given mcpkit's server-side deployment model).

## Genuinely remaining work (revised post-audit)

After this audit, the *only* multi-day items still outstanding from the plan are:

### Workstream B — Test coverage hardening

| Pkg | Coverage | Target | Effort |
|---|---|---|---|
| `worktree/` | 0% (403 LOC, ZERO tests) | 85%+ | 8-12h |
| `device/` | 41.3% | 85%+ | 8-12h |
| `bootstrap/` | 44.2% | 85%+ | 6-8h |
| `a2a/` | 69.1% | 85%+ | 4-6h |
| `embedding/` | 71.0% | 85%+ | 3-4h |

Each is genuinely multi-hour and benefits from a dedicated focused session. Per-package PRs recommended.

### Workstream E — Testing infrastructure

| ID | Item | Effort |
|---|---|---|
| E1 | Fuzz testing corpus (Go 1.18+ `testdata/fuzz/`) | 1 week |
| E2 | Benchstat baselines + CI integration | 1 week |
| E3 | Performance regression alerts wired into CI (depends on E2) | 3 days |

E2 fulfills the explicit `[P2][L] Performance benchmarks` open item in ROADMAP.md (line 487).

### Workstream C1 + C2 — SEP monitoring

Both still in DRAFT upstream:
- SEP-2663 (Tasks Extension) — track via MCP spec repo
- SEP-2640 (Skills Extension `skill://` URIs) — track via experimental-ext-skills repo

No code action until FINAL.

## Lessons learned

The research-agent enumeration of "outstanding work" had a **75% false-positive rate** on the multi-week items: 3 of 4 supposedly missing features were already implemented (C3, C4 in distinct form, D1). The lesson mirrors today's jobb PR-E1 calibration:

> **Never trust an agent's "this is missing" without grep-verifying against the actual code first.**

The same load-bearing guardrail that applied to YAGNI candidate verification (jobb PR-E1) applies to "missing feature" claims. Future enumeration agents should be instructed to grep-verify every claim before reporting.

## Recommended next session focus

If continuing mcpkit work, the highest-leverage **genuinely-remaining** item is:

**B1 — worktree test coverage 0% → 85%** (~8-12h dedicated session).

The `worktree/` package is 403 LOC of production code (pool manager for git worktrees) with ZERO test coverage. Adding integration tests using `t.TempDir()` + real git binary unblocks production confidence. Other B-items (device, bootstrap, a2a, embedding) follow the same pattern at smaller scale.

Last audit: **2026-05-10** by Claude Opus 4.7 (1M context).
