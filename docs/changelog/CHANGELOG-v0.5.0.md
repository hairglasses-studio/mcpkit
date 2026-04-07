# Changelog — v0.5.0 (12-Factor Agents + Enterprise Infrastructure)

Release date: 2026-04-06

29 commits since v0.4.1.

## Added

### 12-Factor Agents Implementation (0a61139)

15 items across 8 packages implementing the 12-Factor Agents pattern:

- **`a2h/`** — Agent-to-Human handoff protocol
- **`agent/`** — Agent thread management with context threading
- **`eval/ab/`** — A/B testing framework for tool evaluation
- **`middleware/gate/`** — Gating middleware for conditional tool access
- **`notify/`** — Notification system for agent events
- **`trigger/`** — Event-driven tool trigger framework
- **`mcptest/fuzz.go`** — Fuzz testing helpers for tool handlers
- **`mcptest/safe_handler.go`** — Panic-safe handler wrapper
- **`workflow/sleep.go`** — Sleep/delay node for workflow graphs
- **`handler/params.go`** — Extended parameter extraction helpers
- **`ralph/factor_integration.go`** — Ralph 12-factor integration layer
- Circuit breaker observable state (`resilience/circuit.go`)
- Registry runtime group stats and tool catalog (`registry/registry.go`)
- Prompt listing enhancements (`prompts/prompts.go`)

### Enterprise Infrastructure (53f0a14)

- **`discovery/marketplace.go`** — Tool marketplace with search, publishing, and ratings
- **`finops/enforcement.go`** — Budget enforcement policies with hard/soft limits and alerts
- **`gateway/affinity.go`** — Session-affinity routing for stateful upstream servers
- **`gateway/federation.go`** — Multi-gateway federation with cross-cluster tool routing
- **`session/redis.go`** — Redis-backed session store for stateless HTTP deployments
- **`transport/session_extract.go`** — Session token extraction middleware (header, cookie, query)
- **`workflow/templates.go`** — Reusable workflow templates with parameterized instantiation

### L3 Autonomy Gates (6ba300b)

- **`health/gates.go`** — Gate framework with 7 default autonomy gates and registry for progressive agent independence

### WebSocket Security (0552612)

- **`transport/websocket_security.go`** — Origin validation, rate limiting, message size limits, connection lifecycle management

### Stateless HTTP Example (3030721)

- **`examples/stateless-http/`** — Docker-compose example with Redis sessions, Nginx load balancer, and tutorial

## Tests

- Bridge/A2A integration and streaming tests (021d10d, 724a124)
- Ralph 12-factor integration tests — 920-line test suite (51f96cc)
- Full test suites for all new packages (marketplace, enforcement, affinity, federation, redis, templates, gates, websocket security)

## Documentation

- `doc.go` added to 17 packages missing documentation (72f32ea)
- ASCII dependency layer diagram in README (3b006fc)

## Security

- Genericize 1Password references in rdloop script (cc66994)
- Remove private repo references from public docs (cfd6ad4)
- Remove employer reference from research sources (915126b)

## Infrastructure / CI

- GoReleaser workflow for automated releases (0552612)
- OpenSSF Scorecard workflow (d12da34)
- CodeQL scanning workflow (5d88333)
- Governance files: CODEOWNERS, FUNDING.yml, PR template (5d88333)
- Add govulncheck to CI pipeline (e392575)
- Remove failing Codex agent workflows (d987fc3, e392575)
- Remove compiled binaries from repo (bd3f686)
- Remove internal development artifacts (73da33e)
- Untrack AI tool configs (cec2f29)
- Gitignore `.agents/` directory (d505246)
- Remove duplicate .md issue templates (4b33576)

## New Packages Summary

| Package | Purpose |
|---------|---------|
| `a2h` | Agent-to-Human handoff |
| `agent` | Thread management |
| `eval/ab` | A/B testing |
| `middleware/gate` | Conditional gating |
| `notify` | Agent notifications |
| `trigger` | Event-driven triggers |
| `workflow/sleep` | Delay nodes |
| `discovery/marketplace` | Tool marketplace |
| `finops/enforcement` | Budget enforcement |
| `gateway/affinity` | Session-affinity routing |
| `gateway/federation` | Cross-cluster federation |
| `session/redis` | Redis session store |
| `transport/websocket_security` | WebSocket hardening |
| `transport/session_extract` | Session token extraction |
| `health/gates` | Autonomy gate framework |
| `workflow/templates` | Reusable workflow templates |
