# Changelog

All notable changes to mcpkit will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Backfill note (2026-05-10):** Tags `v0.4.1`, `v0.5.0`, `v0.5.1`, and `v0.5.2` were
> released between 2026-04-05 (v0.4.0) and this commit without dedicated CHANGELOG
> sections. The v0.6.0 section below covers all work since `v0.5.2`; for
> earlier-tag detail, `git log v0.<prior>..v0.<next>` until per-tag backfill
> ships as a follow-up.

## [Unreleased]

### Added

- **worktree** — Real-git integration tests for pool acquire/release, warmup, stale cleanup, linked-worktree wiring, and error paths; package coverage is now 87.2%.

## [v0.6.0] - 2026-05-10

36 commits since v0.5.2 (commit `6fe6dd5`, "Add shared embedding primitives"). All work is additive (no breaking API changes); minor bump appropriate. Fleet consumers (jobb, ralphglasses, hg-mcp) can `go get -u github.com/hairglasses-studio/mcpkit@v0.6.0` cleanly.

### Added

- **research/** — A2A + SDK competitive research, cloud platform monitors (Cloudflare/Vercel/Azure). Wired into rdcycle scans so each loop emits ecosystem signals alongside repo audit findings.
- **cmd/mcpkit-publish** — Registry publishing CLI with OAuth-backed publisher auth, lifecycle tests, and validation workflow. Enables direct publishing to the MCP server registry.
- **a2a** — Push-notification endpoints (WebSocket delivery for task state transitions). Closes the last P39 A2A bridge work.
- **orchestrator/** — Multi-agent orchestration patterns: swarm mesh, hierarchical delegation, dynamic pattern selector, plus performance benchmarks. Workflow templates expanded.
- **feedback/** — Telemetry submission tool, opt-in collector, dashboard export. Wired into rdcycle scans so feedback signals influence next-cycle priorities.
- **registry/tasks** — Task store with native MCP task lifecycle (foundation for future SEP-2663 Tasks Extension wrapping).
- **L3 autonomy gate framework** (health/) — 7 default gates with registry; ralph + rdcycle now respect autonomy thresholds when self-executing.
- **Stateless HTTP example** — `examples/stateless-http` with docker-compose, Redis sessions, full tutorial. Demonstrates SEP-2567 (Sessionless MCP) integration pattern.
- **WebSocket security hardening** — TLS verification, frame size limits, origin checks; GoReleaser workflow for cross-platform binary releases.
- **finops/** — Pricing model + bootstrap server package for cost-aware deployments.
- **hitools/** — `RequestHumanInput` tool implementing 12-Factor Gap 7 (human-in-the-loop).
- **12-Factor Agents implementation** — 15 items across 8 packages (state ownership, error compaction, control-flow externalization, agent tools, stateless reducer, etc.).
- **Crosspollinate adoption** — `yagni-audit + fossil-mcp` pattern docs from fleet-wide propagation effort (2026-05-10 session).

### Changed

- **Go-SDK v2 compatibility** — assessment complete; dual-SDK gates (`make build-official`, `make test-official`) verified against current upstream. No v2 module tag yet upstream; mcpkit ready when published.
- **ROADMAP.md** (2026-05-10) — corrected stale `transport` coverage claim (was "16% pending", actual 93.2% as of Phase 33 completion). Added explicit list of sub-85% packages.
- **rdcycle integration** — competitive research + feedback signals now feed back into rdcycle scan priorities, closing the loop on R&D direction.
- **CI** — golangci-lint v2 transition with new-only lint mode; OpenSSF Scorecard + CodeQL scanning; action upgrades to v6; 80% coverage threshold enforced; benchmark CI added.

### Removed

- **scripts/rdloop.sh** — explicitly retired (returned exit 1 with retirement message); deleted in PR #18.
- **11 unused identifiers** (PR #17) — `openaiToolResultMessage`, `prefetchCache`/`prefetchCacheEntry`, `protectedBranches` + 7 test helpers. First YAGNI audit pass per the new crosspollinate pattern.
- Compiled binaries + internal development artifacts removed from public repo (governance cleanup).
- Duplicate PR template (case-insensitive filename collision).

### Fixed

- **Static analysis** — zero `staticcheck -checks=U1000` hits after PR #17 (was 11).
- **gitignore** — fixed to allow skill surface files; re-synced Claude skills.
- **Failing Codex agent workflows** — removed; replaced by govulncheck + standard CI.

### Internal

- 12-Factor agent integration tests across ralph; bridge/a2a executor + translator test hardening; community-ready examples for a2a with README + coverage boost.
- 17 packages got `doc.go` files (catch-up batch); doc count now 49/80+ packages.
- Governance files added (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY); GitHub issue forms.

### Migration Notes

No breaking changes. Consumers can `go get -u github.com/hairglasses-studio/mcpkit@v0.6.0` directly. New surfaces (registry/tasks, research, feedback, hitools, L3 gates) are opt-in.

For multi-server deployments interested in the new registry publishing flow, see `cmd/mcpkit-publish/` + `docs/migration-guide.md`.

## [v0.4.0] - 2026-04-05

37 commits since v0.3.0. All 53 packages pass (3,358 tests). Zero `go vet` warnings.

### Added

- **bridge/a2a** — Bidirectional MCP-A2A bridge (translator, executor, agent card, remote agent, auth, streaming)
- **bridge/openapi** — OpenAPI v3 to MCP auto-bridge (operation→tool mapping, auth forwarding)
- **gateway/multi** — Multi-protocol HTTP gateway with automatic MCP/A2A/OpenAI detection
- **gateway/adapter** — Pluggable protocol adapter interface and registry
- **testing/tck** — Technology Compatibility Kit (12 compliance checks across tools + lifecycle)
- **testing/conformance** — MCP everything-server reference implementation (18 tools, resources, prompts)
- **testing/benchmark** — Cross-protocol performance regression suite (14 benchmarks)
- **middleware/truncate** — Response truncation with configurable byte budget
- **middleware/debug** — Structured logging with correlation IDs and sensitive field redaction

### Changed

- **device** — Cross-platform device abstraction (macOS CoreMIDI, Windows WinMM, auto-reconnect, hot-plug)
- **device** — Fixed `IsGridDeviceName` to require CDC serial (was incorrectly matching MIDI interfaces)
- **transport** — Coverage improvement: 44.9% to 88.7%
- Integrated official A2A Go SDK v2.1.0 with compatibility layer
- Closed remaining MCP spec compliance gaps
- Inlined reusable CI workflow, added benchmark CI

## [v0.1.0] - 2026-04-03

### Added

- Initial public release with 35+ packages across 4 dependency layers
- 100% MCP 2025-11-25 spec coverage (tools, resources, prompts, sampling, logging, elicitation, structured output, async tasks)
- `registry` — Tool registration with composable middleware chains
- `handler` — TypedHandler generics with automatic JSON schema generation
- `resilience` — Circuit breakers, rate limiters, caching middleware
- `auth` — JWT/JWKS validation, OAuth 2.1, DPoP, workload identity (GCP/AWS)
- `security` — RBAC, audit logging, tenant context propagation
- `gateway` — Multi-server aggregation with per-upstream resilience
- `workflow` — Cyclical graph engine with saga/compensation patterns
- `finops` — Token accounting, budget policies, dollar-cost estimation
- `ralph` — Autonomous loop runner for iterative task execution
- `rdcycle` — R&D cycle orchestration tools
- `observability` — OpenTelemetry tracing + Prometheus metrics middleware
- `mcptest` — Integration test framework with session replay and snapshot testing
- `sanitize` — Input/output sanitization, secret/PII redaction, URI validation
- `memory` — Agent memory registry with pluggable storage backends
- `skills` — Context-aware lazy tool loading with skill bundles
- `orchestrator` — Fan-out, pipeline, and select execution patterns
- `handoff` — Manager/agent-as-tool delegation protocol
- Dual-SDK support (mcp-go + official Go SDK via build tags)
- 90%+ test coverage across all 35 packages

[v0.4.0]: https://github.com/hairglasses-studio/mcpkit/releases/tag/v0.4.0
[v0.1.0]: https://github.com/hairglasses-studio/mcpkit/releases/tag/v0.1.0
