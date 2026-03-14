.PHONY: build test vet lint check build-official test-official check-dual

build:
	go build ./...

test:
	go test ./... -count=1

vet:
	go vet ./...

lint:
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

check: build vet test

# Dual-SDK targets — verify the official_sdk build tag compiles.
# Tests under official_sdk are limited to packages with complete implementations.
build-official:
	go build -tags official_sdk ./...

test-official:
	go test -tags official_sdk ./registry/ ./mcptest/ ./resilience/ ./security/ ./handler/ ./health/ ./sanitize/ ./auth/ ./client/ -count=1

check-dual: check build-official
