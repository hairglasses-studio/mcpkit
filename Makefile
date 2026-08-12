.PHONY: build test vet lint check build-official test-official check-dual bench bench-guard rdloop-build rdloop-dry rdloop rdloop-12h rdloop-status skill-surface skill-surface-check smoke-matrix

# NOTE: `gateway` is deliberately absent from both lists below. Every
# substantive file in the package is `!official_sdk`-only (gateway.go,
# upstream.go, dynamic.go, resilience.go, federation.go, session_affinity.go,
# observability.go, affinity.go + their _test.go pairs) with no official_sdk
# counterpart — under -tags official_sdk the package compiles to just doc.go
# + errors.go and `go test` reports `[no test files]`. Listing it here was a
# false-green: green build/test output for zero ported functionality and
# zero tests (found 2026-08-07, P52.6). Re-add only once gateway actually has
# an official_sdk implementation.
OFFICIAL_SDK_BUILD_PACKAGES := \
	./registry \
	./handler \
	./mcptest \
	./transport \
	./session \
	./health \
	./sampling \
	./resources \
	./prompts \
	./feedback \
	./testing/conformance \
	./middleware/correlation \
	./middleware/gate \
	./observability \
	./discovery \
	./sanitize \
	./resilience \
	./a2a \
	./bridge/a2a

# `prompts` was previously excluded from this list: prompts/notify_test.go
# was untagged, using DynamicRegistry (which has no official_sdk port)
# directly, and would have broken the moment anyone ran `go test -tags
# official_sdk ./prompts`. Fixed 2026-08-08 (P52.6 round 2) by tagging
# notify_test.go `!official_sdk` — the DynamicRegistry gap itself is
# unchanged (`go test -tags official_sdk ./prompts` reports "[no test
# files]", an honest gap, not a build failure) but the package is safe to
# list here again. `testing/conformance` was entirely !official_sdk-tagged
# before the same round ported its tools/resources/prompts-lifecycle subset
# (NewPortableEverythingServer, portable_*.go) — 13 real tests now run under
# official_sdk (portable_server_test.go); sampling/elicitation/logging/
# completions stay !official_sdk-only (NewEverythingServer,
# everything_server.go — see doc.go for why). `middleware/correlation` was
# gratuitously `!official_sdk`-tagged despite only touching registry's
# compat aliases + stdlib (same class as boundedwrite/truncate, d6bb0be) —
# untagging it was the first blocker secretstudios-mcp's own
# `-tags official_sdk` build attempt hit (it consumes correlation.FromContext
# + correlation.Middleware). `discovery`, `sanitize`, and `resilience` were
# each mostly-or-entirely gratuitously tagged (2026-08-08, P52.6 round 4):
# discovery/metadata.go and wellknown.go only touched registry-neutral types
# plus stdlib (one real fix needed: registry.TemplateURI instead of
# .URITemplate.Raw()); sanitize/output.go and resilience/error_compactor.go
# likewise only touched registry compat aliases + stdlib. Untagging surfaced
# a large pre-existing but dormant discovery test suite (marketplace/client/
# publisher tests) that was already portable and is now exercised under
# official_sdk too. `middleware/gate` (2026-08-08, fleet flip-order #1) was
# gratuitously tagged — gate.go/doc.go only touch registry's compat aliases +
# stdlib; gate_test.go's one raw `mcp.CallToolRequest{}` literal became
# `registry.CallToolRequest{}` since the zero value compiles identically
# under either SDK. `observability` (same round) was a real port, not a
# tag removal: observability.go/middleware.go reached into
# `*registry.ToolMeta` (result.Meta / request.Params.Meta) directly to
# bridge OTel trace context through MCP `_meta` — a shape that is genuinely
# SDK-specific (mcp-go's Meta is a `*mcp.Meta` struct with an
# AdditionalFields sub-field; the official SDK's is a plain
# `map[string]any`, and even CallToolResult.Meta's pointer-vs-value shape
# differs). Fixed by adding a bounded `RequestMetaCarrier`/
# `SetResultMetaCarrier` compat accessor pair to registry/compat.go +
# compat_official.go (same pattern as ExtractTextContent/ToolMetaField) and
# rewriting Provider.InjectMeta/ExtractMeta (renamed InjectResultMeta/
# ExtractRequestMeta — neither was called outside this package) to go
# through them instead of touching the SDK-specific shape directly. `a2a` and
# `bridge/a2a` (2026-08-08, P52.6/P52.7 round 8): `a2a` was already fully
# portable with zero changes (never imported mcp-go directly — the A2A
# protocol layer works entirely through registry-neutral types) and simply
# had never been added to these lists before; its real ~90-test suite
# (client/server/interceptors/push-config/TCK) now runs under official_sdk
# too. `bridge/a2a` was a genuine port (flip1's build attempt found ~10
# compile-error classes): raw mcp.Tool/mcp.TextContent/mcp.ImageContent/
# mcp.EmbeddedResource type assertions and ToolInputSchema.Type/Properties/
# Required field access, none of which the official SDK's `any`-typed
# schema or pointer-based content types support directly. Fixed by adding
# ExtractImageContent/MakeImageContent, ExtractEmbeddedResource/
# MakeEmbeddedResourceText, InputSchemaType/Properties/Required/
# AdditionalProperties, and MakeToolInputSchema to registry/compat.go +
# compat_official.go (same ExtractTextContent/OutputSchemaType pattern),
# then rewriting translator.go/executor.go/remote_agent.go to go through
# them instead of raw mcp-go construction — both source files dropped their
# mcp-go import entirely as a result. One test (invalid-base64 image data)
# stayed genuinely !official_sdk-only: the official SDK's ImageContent.Data
# is already raw []byte, so "invalid base64" isn't an expressible scenario
# there at all, not a missing compat feature.
OFFICIAL_SDK_TEST_PACKAGES := \
	./registry \
	./handler \
	./mcptest \
	./transport \
	./session \
	./health \
	./sampling \
	./resources \
	./prompts \
	./feedback \
	./testing/conformance \
	./middleware/correlation \
	./middleware/gate \
	./observability \
	./discovery \
	./sanitize \
	./resilience \
	./a2a \
	./bridge/a2a

BENCH_PACKAGES ?= ./mcptest ./testing/benchmark
BENCH_FLAGS ?= -bench=. -benchmem -run '^$$'
BENCH_BASELINE ?= bench-baseline.txt
BENCH_CURRENT ?= bench-current.txt
BENCH_LATENCY_THRESHOLD ?= 15
BENCH_MEMORY_THRESHOLD ?= 20
BENCH_ALLOC_THRESHOLD ?= 10
BENCH_MIN_OVERLAP ?= 1

build:
	go build ./...

test:
	go test ./... -count=1

test-fast:
	go test -short ./registry ./handler ./transport ./protocol ./skills ./fleetinventory ./surfaceinventory ./cmd/...

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || \
	(command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "no linter installed, skipping")

check: build vet test-fast skill-surface-check

# Dual-SDK targets — verify the official_sdk build tag on packages with
# complete official-SDK implementations. Test scope is intentionally narrower
# than build scope until all package test fixtures are SDK-neutral.
build-official:
	go build -tags official_sdk $(OFFICIAL_SDK_BUILD_PACKAGES)

test-official:
	go test -tags official_sdk $(OFFICIAL_SDK_TEST_PACKAGES) -count=1

check-dual: check build-official test-official

bench:
	go test $(BENCH_FLAGS) $(BENCH_PACKAGES) > "$(BENCH_CURRENT)"
	cat "$(BENCH_CURRENT)"

bench-guard:
	go run ./tools/benchguard \
		-baseline "$(BENCH_BASELINE)" \
		-current "$(BENCH_CURRENT)" \
		-latency-threshold "$(BENCH_LATENCY_THRESHOLD)" \
		-memory-threshold "$(BENCH_MEMORY_THRESHOLD)" \
		-alloc-threshold "$(BENCH_ALLOC_THRESHOLD)" \
		-min-overlap "$(BENCH_MIN_OVERLAP)"

# rdloop targets — autonomous R&D cycle launcher.
rdloop-build:
	go build -o ./rdloop ./cmd/rdloop

rdloop-dry: rdloop-build
	./scripts/rdloop.sh --dry-run

rdloop:
	./scripts/rdloop.sh --budget 200 --duration 24h

rdloop-12h:
	./scripts/rdloop.sh --budget 100 --duration 12h

rdloop-status:
	@python3 -c "\
import json, sys; \
s=json.load(open('.rdloop_state.json')); \
print(f'Cycles: {s[\"cycle_number\"]}'); \
print(f'Iterations: {s[\"total_iterations\"]}'); \
print(f'Cost: \$${s[\"total_cost\"]:.2f}'); \
print(f'Avg/cycle: \$${s.get(\"avg_cost_per_cycle\",0):.4f}'); \
print(f'Peak/cycle: \$${s.get(\"peak_cost_per_cycle\",0):.4f}'); \
print(f'Downgrades: {s.get(\"total_downgrades\",0)}'); \
print(f'Last cycle: {s[\"last_cycle_at\"]}'); \
" 2>/dev/null || echo "No state file found. Run 'make rdloop' first."

skill-surface:
	go run ./tools/genskillsurface

skill-surface-check:
	go run ./tools/genskillsurface --check

# smoke-matrix — verify each public example works on stdio and HTTP transports.
# Spawns each example binary, issues initialize + tools/list, validates the
# response. Examples that do not support a given transport are skipped with
# a logged reason rather than a failure.
# CI note: included in the optional extended check; not part of the default
# 'check' target because it requires building all example binaries (~30s).
smoke-matrix:
	go run ./tools/smoke-matrix

HG_PIPELINE_MK ?= $(or $(wildcard $(abspath $(CURDIR)/../dotfiles/make/pipeline.mk)),$(wildcard $(HOME)/hairglasses-studio/dotfiles/make/pipeline.mk))
-include $(HG_PIPELINE_MK)
