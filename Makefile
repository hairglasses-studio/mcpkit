.PHONY: build test vet lint check build-official test-official check-dual rdloop-build rdloop-dry rdloop rdloop-12h rdloop-status skill-surface skill-surface-check smoke-matrix

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

# Dual-SDK targets — verify the official_sdk build tag compiles.
# Tests under official_sdk are limited to packages with complete implementations.
build-official:
	go build -tags official_sdk ./...

test-official:
	go test -tags official_sdk ./... -count=1

check-dual: check build-official

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
