# Changelog

All notable changes to mcpkit will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
