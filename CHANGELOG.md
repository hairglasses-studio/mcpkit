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

- **surfaceinventory — struct-based parameter resolution for mcpkit TypedHandler** (from the type-resolution research): resolves `handler.TypedHandler[In, Out](…)` input parameters from the `In` struct's fields + json/jsonschema tags, via a same-DIRECTORY ast struct index built during the existing walk — **no go/packages, no type checker, no subprocess** (ScanFiles now groups .go files by package dir and does a cheap index pass before extraction). Uses the invopop/jsonschema tag grammar TypedHandler actually uses (`description=`, repeatable `enum=`, bare `required`, `-` skip; Required = no `,omitempty` OR bare `required`) — deliberately NOT the official-SDK AddTool grammar (whole-tag-is-description), which is a separate follow-up. Scope (documented skips, "skip don't guess"): unqualified same-package `In` types only; cross-package/qualified/alias/embedded stay nil. Extends the `param_quality` dimension from 7 to **11 repos** (adds process-mcp/github-runner-mcp/secretstudios/dotfiles-mcp; mcpkit's own TypedHandler params now measured). Scan cost unchanged (~5.5s).

- **fleetinventory — Claude Code `_meta` extension adoption** (informational; the research's HIGH-priority static check, and one nothing else in the ecosystem covers): detects consumer *usage* of the three Claude-Code-specific per-tool `_meta` keys that are NOT in the generic MCP spec — `anthropic/alwaysLoad` (opt a tool out of deferred/tool-search loading), `anthropic/maxResultSizeChars` (raise the persist-to-disk output ceiling), `anthropic/requiresUserInteraction` (force a permission prompt) — via mcpkit `ToolDefinition` field-setters (`AlwaysLoad: true`/`.MaxResultChars =`/`SkipRequiresUserInteraction`) and the literal keys. USE-pattern only, so the mcpkit struct *definition* and read-sites don't false-match; the detector skips its own source. Per-repo `RepoReport.CCMeta` + a fleet "Claude Code _meta extensions" section, with an `alwaysLoad`-OVERUSE warning (>8 tools opting out of deferred loading undermines tool-search, which Claude Code enables by default). Live: secretstudios (alwaysLoad 5, maxResultChars 25), mcpkit, jobb, ralphglasses, dotfiles-mcp, process-mcp adopt some. Perf: a cheap `strings.Contains` gate keeps the full-fleet scan at ~5s despite reading .go bodies (bounded 12KB/file).
- **surfaceinventory — per-parameter extraction** for mcp-go tools: `Surface.Params []ToolParam` (name, type, has-description, required, has-enum) parsed from the inline `mcp.WithString/WithNumber/WithBoolean/WithArray/WithObject("p", mcp.Description(…)/Required()/Enum(…))` options in a `NewTool` call. Struct-based inputs (mcpkit TypedHandler, official-SDK generics) need cross-file type resolution and stay nil (distinct from a genuinely param-less tool) — a documented coverage bias, deferred to a future type-resolving extractor.
- **fleetinventory — `param_quality` scoring dimension** (Anthropic "writing tools for agents": every param should carry its own description; use enums for closed value sets): scores the fraction of statically-visible params carrying a description, with a light penalty (≤15) for enum-candidate params (names matching `type|status|mode|format|kind|level|state|…`) that declare no enum. Measured only over tools with extractable params (nil elsewhere — no false scoring of TypedHandler/Python repos). Weight 0.06, others rebalanced to sum 1.0; `param` scoreboard column + a note when below 70. Live: measured on 7 mcp-go repos (jobb/ralphglasses/runmylife/jellyfin-mcp-deluxe/mesmer/hg-mcp 85-88, mcpkit 100).

- **fleetinventory — Tasks-shape correctness warning** (from the Go-SDK research, its High-priority "warning not maturity-signal" flag): statically detects use of the removed-from-core 2025-11-25 Tasks shape and warns, with severity by spec era — NON-CONFORMANT on a modern-capable/dual server (core Tasks removed in the 2026-07-28 rewrite; no shipping Go SDK implements the replacement extension), migration-hazard on a legacy-only one. Deliberately HIGH-CONFIDENCE only: flags an `mcpkit/registry/tasks` import or a package-qualified mcp-go Tasks type (`mcp.Task{`/`mcp.TaskStatus`/`mcp.TaskSupport`/`mcp.NewTask`) — NOT bare `TaskStatus`/`tasks/list`, which flood false positives (runmylife's `tasks/list` is the Google Tasks API; several repos have private `TaskStatus` types). Under-detects rather than cries wolf. Surfaced as `MCPRuntime.{TasksLegacyShape,TasksEvidence}` + a "Tasks-shape warnings" block in the report; a warning, not a scored penalty (era-dependent validity). Bounded 8KB-per-file header read; live sweep flags 3 (jobb, mcpkit, mesmer — all genuine mcp-go Task usage), ~5s total. Live `tasks/list` probe is the authoritative check, deferred to the live variant.

- **fleetinventory — `discoverability` scoring dimension** (tool-count-aware; the Claude-Code research's #1 highest-leverage check): for servers past the tool-search-relevance threshold (25+ tools; Anthropic guidance: selection accuracy degrades past ~30-50 tools without tool-search), scores how well tools cluster into coherent name families that a single BM25/regex tool-search query can surface — `0.7·(share of tools in a ≥2 family) + 0.3·(dominant-namespace share, capped at 50%)`. A large well-prefixed server (one dominant prefix, or a few coherent families) scores high; a long tail of singleton/ungrouped names scores low. Below the threshold the dimension is not applicable (nil — tool-search is unnecessary at that size). Weight 0.10, others rebalanced to sum 1.0; `disc` scoreboard column + a note (top prefix %, ungrouped count) when below 70. Live: mesmer/hg-mcp/secretstudios 100 (single dominant prefix), hg-pi 62 and mcpkit 64 flagged (fragmented namespaces). The static half; whether a server sets deferred loading (`defer_loading`/`anthropic/alwaysLoad`) is a config/live signal deferred to the live variant.

- **fleetinventory — MCP runtime / spec-era profiling** (from the Go-SDK capability research, its P0 signal): a static go.mod parse per repo detects which MCP SDK(s) it depends on and classifies the wire-protocol era — `modern-capable` (official `modelcontextprotocol/go-sdk` ≥v1.7.0, which implements the breaking 2026-07-28 stateless/MRTR rewrite), `legacy-only` (mark3labs/mcp-go or official <v1.7.0 → 2025-11-25), `dual`, or `via-mcpkit`. New `RepoReport.MCPRuntime {sdks, spec_era}` + an "MCP runtime / spec era" table and fleet distribution line in the report. INFORMATIONAL, not scored — running the ecosystem-standard 2025-11-25 shape is not a defect. Caveat documented: a modern-capable dependency means capable, not that the code actually speaks Modern (SDKs back-compat); authoritative era detection needs the live probe (deferred). First sweep: 6 dual / 6 legacy-only; dotfiles-mcp furthest behind (go-sdk v1.5.0 + mcp-go v0.47.0).

- **fleetinventory — `security` scoring dimension** (from the Claude-Code/MCP best-practices research pass, static subset only): encodes the statically-checkable OWASP MCP Top 10 categories plus MCP tool-name spec validity. **MCP01** (secret exposure) flags provider-shaped credential literals (`sk-…`, `AKIA…`, `gh[pousr]_…`, Slack/Google keys, PEM private-key blocks, explicit `password=`/`api_key=` assignments) appearing in a tool description or schema string. **MCP03** (tool poisoning) flags model-directed imperative language in descriptions ("ignore previous instructions", "without telling the user", `<system>` markers, "you must call/exfiltrate…") — text aimed at steering the model rather than describing behavior. Plus `spec_invalid_tool_name` for MCP-tool names violating the spec charset/length `[A-Za-z0-9_.-]{1,128}`. (MCP09 Shadow Servers is already what a fleet-wide inventory *is*.) New `RepoScore.SecurityFindings` (kind/surface/file/line/detail) and a `security` column in the scoreboard; weight 0.12, others rebalanced to keep the composite sum at 1.0. First live sweep: **0 findings fleet-wide** (clean), dimension measured on 15 MCP-bearing repos. `naming_discipline` doc-clarified as a *house* style (snake_case), explicitly not an MCP requirement — the spec's actual name MUST is the new security check.

### Fixed

- **fleetinventory scoring — two trustworthiness fixes so the scoreboard stops mis-ranking well-run repos as cleanup targets.** (1) *Description coverage* now credits a description slot that is syntactically present even when its value is a non-literal expression (a `desc := …` local, a `+`-concatenation, a call) — previously the static scanner's inability to read the string counted a real description as absent, so runmylife (actually ~100%) scored 50 and ralphglasses 62. surfaceinventory gains a `Surface.HasDescription` signal set whenever the description slot exists; live effect: runmylife 50→100, ralphglasses 62→99, jobb 30→69. (2) *Cross-repo duplication* now collapses registered publish-mirror pairs into one group via each repo's `mirror-parity.json` (standalone_repo ↔ canonical in-tree host), so a tool shared between a canonical server and its declared mirror is not counted as duplication debt — this is what made the two retracted roadmap dedup items look like levers. Live effect: dotfiles-mcp duplication 58→99, ralphglasses 80→96; the `dotfiles` namespace drops from cross-repo-duplicate span 2 to span 1 while genuinely-shared namespaces like `session` (mcpkit) stay flagged.

### Added

- **surfaceinventory** — `surface_inventory_live` tool: connects to a fleet MCP server binary as a client over stdio (official go-sdk's `mcp.CommandTransport{Command: exec.Command(...)}` + `Client.Connect`), enumerates its actually-registered tools/resources/prompts via `ClientSession`'s auto-paginating `Tools`/`Resources`/`Prompts(ctx, params)` iterators (`iter.Seq2`), and diffs the live names against a static AST scan of the same repo — `{live_only, static_only, both, counts}`. `official_sdk`-only (mcp-go has no equivalent client-side list iterator); the default build ships a `live_default.go` stub that fails closed with a clear "requires official_sdk build" error instead of an empty/misleading diff. Diff logic (`diffNames`) is pure and portable, tested under both tags; an `official_sdk`-gated integration test spawns mcpkit's own `examples/minimal` server (2 tools) over a real stdio round trip and confirms the live/static name sets match.

- **discovery** — `.well-known/mcp.json` server cards (`ServerMetadata.ToolCount`) now carry a machine-readable `tool_count` derived from the live `registry.ToolRegistry` at generation time (`cfg.Tools.ToolCount()`), not `len(Tools)` or a hand-maintained constant. Closes the gap where `fleetinventory`'s `declaredGap` scoring dimension (which already trusted only a flat top-level `tool_count` key, deliberately never prose "N tools" claims) had nothing real to read from mcpkit's own manifest — every repo's declared-count dimension was silently unmeasured. Covered via `ServerCardHandler`/`HandleContractWrite`/`MetadataFromConfig` tests asserting the count tracks live registry mutations across requests.

- **fleetinventory** — Quality-scoring layer (`score.go`, from the lane-scoring design): per-repo 0-100 composite over six dimensions (description coverage, naming discipline, intra+cross duplication, severity-weighted violation burden, median/MAD size outliers, declared-count gap via machine-readable `.well-known/mcp.json` `tool_count` only — prose claims deliberately not trusted), with data-confidence renormalization (unmeasured dimensions stay nil), truncated-walk confidence capping, lifecycle×scope impact weighting from the workspace manifest, and a lifecycle-weighted roadmap priority that ranks the cleanup queue. Plus a fleet-wide namespace view (prefix × repo-span) exposing cross-repo tool-name duplication. New `fleet_inventory_score` tool; scoreboard appended to the markdown report. Full scored sweep: ~5.6s.

- **surfaceinventory** — Python/FastMCP extraction: `@mcp.tool()`/`@mcp.resource("uri")`/`@mcp.prompt()` decorators (kwargs or def-name + docstring-derived descriptions). Closes the blind spot where hg-pi (100 tools) and hg-android (252 tools) scanned as zero. venv/site-packages/__pycache__ pruned; test_*.py skipped.

### Fixed

- **ralph, rdcycle, research, roadmap, vuln, hitools, mcptest** — 35 of mcpkit's own tool registrations statically scanned as having an empty description (`surface_inventory_scan` false-negative: the AST scanner only reads a bare string-literal 2nd argument to `handler.TypedHandler`/`&mcp.Tool{}`, so a `desc := "..." + "..."` local variable or a `+`-concatenated multi-line literal — both common in this repo — read as `""` even though the runtime description was real and non-empty). Rewrote each to pass a single, accurate, literal one-sentence description directly at the call site; the two `research/*.go` and `vuln/module.go` tools that append `handler.FormatExamples(...)` for runtime documentation now do so via a post-construction `td.Tool.Description +=` so both the static scanner and the MCP client see the full picture. Fleet-wide `description coverage` scoring on mcpkit itself goes from 35/104 empty to 0/104.

- **fleetinventory** — `.ralph`/`.ouro` state dirs pruned from the walk (`HasRalph` now via direct stat): jobb's leaked `.ralph/go-mod-cache-*` Go module caches had inflated its apparent MCP surface 24x (700 scanned vs 58 real) and poisoned dedup metrics with SDK example fixtures.

- **fleetinventory** — Fleet inventory platform: one bounded, parallel, pruned walk per repo (per-repo timeout, 500k-file cap, errors recorded never fatal) feeds provider-parity metrics (instruction files, retired mirrors, skills canonical/generated, MCP config sources incl. `[profiles.*]` detection), code-surface counts via `surfaceinventory.ScanFiles` (no second walk), baseline violations, and disk/manifest/catalog drift. Exposed as `fleet_inventory_scan` + `fleet_inventory_report`; build-tag-free under both SDKs. Live sweep: 32 repos in ~16s. The robust successor to the find-bound bash agent-parity audit.
- **surfaceinventory** — `ScanFiles(dir, name, files, kinds)`: extract surfaces from a caller-supplied file listing, for callers that already walked the repo.
- **surfaceinventory** — New fleet-wide static surface-inventory module: `surface_inventory_scan` + `surface_inventory_report` tools enumerate MCP tool/resource/prompt registrations across every fleet SDK idiom (mcpkit `handler.TypedHandler`, mcp-go `mcp.NewTool`, official-SDK `mcp.AddTool`/`&mcp.Tool{}` and slice-elided `ToolDef{}` literals), plus CLI subcommands (cobra/`flag.NewFlagSet`) and HTTP routes, via parse-only `go/ast` inspection (stdlib only, no type-checking). Workspace-manifest-driven repo discovery with `.git`-subdir fallback; per-file parse errors recorded, never fatal; build-tag-free (both SDKs). Closes the gap where every per-server `tool_catalog` could only introspect its own registry — first live sweep: 32 repos, 16.5k surfaces in ~6.5s.

## [v0.8.1] - 2026-08-08

1 commit since v0.8.0, closing out the small-gaps batch that was meant to
land before the v0.8.0 cut but arrived just after it (a message-ordering
gap, not a rescope). Patch bump: purely additive, no API changes to
anything already shipped.

### Added

- **registry** — `NewServerProgressReporterFromRequest` gains its mcp-go
  counterpart (previously official_sdk-only, added in v0.8.0). Takes
  `(server, req, total)` rather than official's `(req, total)` — mcp-go's
  `CallToolRequest` carries no session/server reference the way the
  official SDK's does, so `server` stays an explicit argument — but the
  mcp-go build was never subject to official's session-count ambiguity in
  the first place (`SendNotificationToClient` resolves the session via
  `ctx` at call time, not anything bound at construction), so this is a
  thin, genuinely trivial delegation to the existing
  `NewServerProgressReporter`, added purely for call-site/name-parity
  discoverability with the official-only constructor.

## [v0.8.0] - 2026-08-08

41 commits since v0.7.0 (2026-07-08), starting at commit `f43a5d9` ("chore(rdcycle): update cycle-1/cycle-2 specs"). All work is additive (no breaking API changes); minor bump appropriate. Fleet consumers can `go get -u github.com/hairglasses-studio/mcpkit@v0.8.0` cleanly.

The headline of this cycle is the `official_sdk` build tag (`modelcontextprotocol/go-sdk`) becoming a real, dual-SDK-tested alternative to the default `mark3labs/mcp-go` build across most of the tree — not just compiling, but with real behavior parity verified by both-tags tests. `go-sdk` bumped v1.6.0 → v1.7.0.

### Added

- **registry** — Real `official_sdk` progress-notification support: `ServerProgressReporter`/`ServerProgressMiddleware` now send genuine `notifications/progress` messages via `ServerSession.NotifyProgress` (previously a no-op stub — go-sdk has supported this since at least v1.6.1, an unwired gap not an SDK limitation), plus a new `NewServerProgressReporterFromRequest` constructor that binds directly to a request's session for always-correct delivery under concurrent/stateless deployments; `ProgressTokenFromRequest` now does real token extraction via `CallToolParamsRaw.GetProgressToken()` instead of hardcoding `nil`.
- **registry** — `NewTextContent` (official_sdk counterpart to mcp-go's aliased constructor) and an `Annotations`/`MakeResourceAnnotations` compat pair for resource audience/priority hints (the two SDKs' `Priority` field differs, `*float64` vs `float64`).
- **registry** — `ServeAuto` (official_sdk), `NewResource`/`NewResourceTemplate`, `MakePrompt`, `TemplateURI`, `PromptArguments` + `ToolMetaField` read accessors, `Annotation{ReadOnly,Destructive,Idempotent,OpenWorld}Hint` read accessors, content/schema compat accessors (for the bridge/a2a port), `NewMCPServerWithOptions` + `NewStreamableHTTPHandler`, `ResourceContents` added to compat.go's mcp-go alias surface, consent meta for write tools + annotation overrides.
- **resources / prompts** — SDK-neutral resource/prompt handler adapters (`TextResourceHandler`, `JSONResourceHandler`, `TextPromptHandler`) so consumer handler code compiles once for both build tags instead of branching on the SDK's differing request/result shapes; `resources.CallHandlerText` neutral resource-handler test invoker; `uri_middleware.go` (SSRF/path-traversal defense) ported to official_sdk with full test coverage.
- **mcptest** — `Client.ListToolNames`/`ListResourceURIs`/`ListPromptNames`.
- **bridge/a2a** — Full real official_sdk port (translator/executor/remote_agent + complete test suite), not a stub — the blocker that had silently broken systemd-mcp/tmux-mcp/process-mcp/github-runner-mcp's official-tag builds (missed by the initial fleet exposure audit).
- **observability** — Real official_sdk support via a meta-carrier compat pair (previously untagged/broken under official_sdk).
- **testing/conformance** — Tools/resources/prompts lifecycle ported to official_sdk (the portable subset).
- **middleware/boundedwrite, middleware/truncate** — Made build-tag-free (work under both SDKs without a tag split at all).

### Changed

- **Gratuitous `official_sdk` build tags removed** from packages that didn't actually need SDK-specific code: `middleware/correlation`, `middleware/gate`, Audit/SafetyTier middleware (registry), `ErrorCompactorMiddleware` (resilience), `OutputMiddleware` (sanitize), and `discovery` (`.well-known/mcp.json` generation) — these now compile identically under both tags without duplication.
- **`ApplyMCPAnnotations`** brought to parity between builds, including a fix to the official-side `OutputSchema` wire format.
- **Makefile `official_sdk` build/test package lists** — `gateway` removed (a false-green: it does not actually build under official_sdk); `a2a`, `bridge/a2a`, `resources`, `prompts`, `testing/conformance`, `discovery`, `sanitize`, `resilience`, `middleware/gate` added once genuinely dual-SDK.
- **handler** — Official_sdk `TypedHandler` now populates `Tool.OutputSchema` (was previously left empty on that build).
- **resilience** — Release the half-open circuit-breaker permit on partial success so breakers can actually close again.

### Fixed

- **prompts** — `notify_test.go` tagged `!official_sdk` (an untagged-test gap, parity with the equivalent resources-package fix).

### Internal

- `docs(sdk-migration)` — reopened the stale "wait for go-sdk v2" recheck trigger (the rewrite shipped as v1.7.0, not v2, so the original hold condition no longer applies) and posted `a2a`/`bridge/a2a` status banners (`a2a/` frozen pending reconciliation, `bridge/a2a` designated primary).
- Three small compat gaps found by the dotfiles-mcp codemod folded in ahead of this cut (see `a61c028`): `NewTextContent`/`Annotations` additions above, plus confirmation that `registry.ToolInputSchema{...}` composite literals are fundamentally non-portable (official's `ToolInputSchema` is `any`, not a struct) and `MakeToolInputSchema(...)` is the only portable construction path — no code change needed there beyond documenting it.

## [v0.7.0] - 2026-07-08

42 commits since v0.6.0 (2026-05-10), starting at commit `4bac7bb` ("docs(migration): add consumer pin status section"). All work is additive (no breaking API changes); minor bump appropriate. Fleet consumers can `go get -u github.com/hairglasses-studio/mcpkit@v0.7.0` cleanly.

### Added

- **middleware/correlation** — Focused correlation-ID + `slog` middleware (C4).
- **fossil/** — Full typed integration surface built out from the v0.6.0 initial package: scan wrapper, typed result contracts (`contracts.go`, `clones.go`), scaffolding wrapper, end-to-end integration test, `example_test.go`/`doc.go`, and the informational SARIF workflow; multi-wave rollout (Wave B, Wave C) with docs kept in sync.
- **registry** — `AlwaysLoad` tool flag injects `_meta["anthropic/alwaysLoad"]=true` so discovery front-door tools survive deferred tool loading in Claude Code; `ApplyMCPAnnotations` now also populates the top-level `Tool.Title` field (preferred by MCP spec 2025-06-18) alongside `Annotations.Title`.
- **worktree** — Pool pre-warming for fast session startup (`feat/scale`).
- **benchmark** — Middleware overhead comparison vs raw `mcp-go`, plus a p99 benchmark CI guard.
- **gateway/multi** — `example_test.go` demonstrating protocol detection.
- **docs** — Observability guide, auth RFC compliance audit (D2+D3), SDK migration guide consumer-pin-status section (G1+G2), and a deferred-items audit findings doc (3 of 4 "deferred" features already shipped).
- **worktree / bootstrap / device / a2a / embedding** — Expanded test coverage: worktree 0.0%→87.2%, bootstrap 44.2%→95.2%, device 41.3%→88.4%, a2a 69.1%→79.1% (B4 partial), embedding 71.0%→93.3% (B5).

### Changed

- **mcp-go** upgraded v0.52.0 → v0.54.0.
- **bootstrap** — `WithStrictInputSchemaDefault` added to bootstrap defaults (mcp-surface).
- **finops** — `DefaultPricing` refreshed to current-gen models.
- **Provider model policy** — rdcycle Claude fallback routed to Sonnet.
- **CI** — fast PR lane added; workflows routed to local runners; gitleaks fixture fingerprints allowlisted; CI/cycle specs repaired.
- **Provider surface cleanup** — stale Codex MCP block removed, cline projection removed, local Gemini fossil-mcp server config cleared, redundant `fossil-mcp` entry removed from `.mcp.json` (now workspace-scoped); mcpkit compatibility surfaces reconciled.
- **ROADMAP.md** — crosspollinate fossil suggestion converted into explicit execution-status and tranche items with acceptance/abort criteria instead of doc-only guidance; coverage claims updated to track the test waves above.

### Fixed

- **README.md / AGENTS.md** — corrected the positioning banner, which had been factually backwards (described mcpkit as a compatibility shim; it is the canonical, actively-maintained MCP-server framework consumed as a dependency across the fleet).
- Removed 2 accidentally-committed Go binaries (`examples/pagination`, `tools/smoke-matrix`) and gitignored the pattern so it can't recur.

### Internal

- State/unification roadmap checkpoints recorded alongside the above waves.

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
