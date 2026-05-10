.PHONY: build test vet lint check build-official test-official check-dual bench bench-guard rdloop-build rdloop-dry rdloop rdloop-12h rdloop-status skill-surface skill-surface-check smoke-matrix

OFFICIAL_SDK_BUILD_PACKAGES := \
	./registry \
	./handler \
	./mcptest \
	./transport \
	./session \
	./gateway \
	./health \
	./sampling \
	./resources \
	./prompts

OFFICIAL_SDK_TEST_PACKAGES := \
	./registry \
	./handler \
	./mcptest \
	./transport \
	./session \
	./gateway \
	./health \
	./sampling

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

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || \
	(command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "no linter installed, skipping")

check: build vet test skill-surface-check

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
