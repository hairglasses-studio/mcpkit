# Contributing to mcpkit

Thank you for your interest in contributing to mcpkit. This document covers
everything you need to get started.

## Reporting Bugs

Open a [GitHub Issue](https://github.com/hairglasses-studio/mcpkit/issues) with:

- Go version and OS
- Minimal reproduction steps
- Expected vs. actual behavior
- Relevant error output or logs

## Suggesting Features

Open a [GitHub Issue](https://github.com/hairglasses-studio/mcpkit/issues) with
the `enhancement` label. Describe the use case, not just the solution. If the
feature involves API changes, include a short code sketch showing how it would
be called.

## Submitting Pull Requests

1. Fork the repository and clone your fork.
2. Create a branch from `main`: `git checkout -b feat/my-change`
3. Make your changes, following the code style below.
4. Run the full check suite: `make check`
5. Commit with a clear, descriptive message.
6. Push your branch and open a PR against `main`.

Keep PRs focused. One logical change per PR is easier to review than a combined
refactor-plus-feature.

## Development Setup

**Requirements:** Go 1.26.1+

```bash
git clone https://github.com/hairglasses-studio/mcpkit
cd mcpkit
make build    # go build ./...
make test     # go test ./... -count=1
make vet      # go vet ./...
make check    # build + vet + test
```

## Code Style

- Format with `gofmt` (or `goimports`).
- Pass `go vet ./...` with no warnings.
- Follow existing patterns in the codebase.
- Handler functions must return `(*mcp.CallToolResult, nil)` -- never `(nil, error)`.
- Use `handler.CodedErrorResult` for error responses.
- Protect shared state with `sync.RWMutex` (`RLock` for reads, `Lock` for writes).
- Middleware follows the signature:
  `func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc`

## Testing Requirements

- All existing tests must pass before submitting a PR.
- Add tests for new features and bug fixes.
- Run tests with race detection: `go test ./... -count=1 -race`
- Integration tests use `mcptest.NewServer()`; unit tests use stdlib `testing`.

## Commit Messages

Use conventional-style prefixes:

```
feat: add streaming transport support
fix: handle nil context in middleware chain
docs: update handler examples in README
test: add integration tests for registry
```

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
