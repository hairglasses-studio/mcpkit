# mcpkit Roadmap

Last updated: 2026-03-14.

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

### Decision Points

- **After Phase 5**: Evaluate A2A spec stability — if v1.0 ships, fast-track `a2a/`; otherwise prototype only
- **After Phase 6**: Assess orchestrator patterns against real-world usage from hg-mcp/jobb migrations
- **After Phase 7**: Re-evaluate WebMCP bridge and Chrome integration based on adoption signals

---

## Updated Dependency Layers (post-Phase 7)

- **Layer 1** (no internal deps): `registry`, `health`, `sanitize`, `secrets`, `client`
- **Layer 2** (depend on Layer 1): `resources`, `prompts`, `handler`, `resilience`, `mcptest`, `auth`, `observability`, `logging`, `sampling`, `roots`, `research`, `discovery`, `dispatcher`, `extensions`, `memory`, `finops`
- **Layer 3** (depend on Layer 2): `security`, `gateway`, `ralph`, `skills`, `a2a`
- **Layer 4** (depend on Layer 3): `orchestrator`, `handoff`, `workflow`, `bootstrap`
