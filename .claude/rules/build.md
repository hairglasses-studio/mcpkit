Build & test:
- `go build ./...`       — build all packages
- `go vet ./...`         — static analysis
- `go test ./... -count=1` — run all tests (no cache)
- `make check`           — all three above
- `make build-official`  — verify official SDK build
- `make check-dual`      — full check + official SDK build
