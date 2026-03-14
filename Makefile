.PHONY: build test vet lint check

build:
	go build ./...

test:
	go test ./... -count=1

vet:
	go vet ./...

lint:
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

check: build vet test
