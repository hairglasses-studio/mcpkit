---
paths: ["**/*_test.go"]
---

# Testing Conventions

## Integration Tests (mcptest)

Use the mcptest package for end-to-end tool testing:

```go
func TestMyTool(t *testing.T) {
    reg := registry.New()
    // register tools on reg...

    srv := mcptest.NewServer(t, reg)
    client := mcptest.NewClient(t, srv)

    result := client.CallTool("my-tool", map[string]any{
        "param": "value",
    })

    mcptest.AssertNoError(t, result)
    mcptest.AssertContains(t, result, "expected text")
}
```

## Unit Tests

- Use stdlib `testing` package, no external assertion libraries
- Table-driven tests for parameterized cases
- Use `t.Parallel()` where tests have no shared mutable state
- Name subtests descriptively: `t.Run("returns error for empty query", ...)`

## Running Tests

- Always use `-count=1` to bypass cache: `go test ./pkg -count=1`
- Each package must pass in isolation: `go test ./handler/ -count=1`
- Use `-v -cover` for visibility: `go test ./handler/ -count=1 -v -cover`
- Full suite: `make check` (build + vet + test)

## Test File Placement

- Test files live in the same package as the code they test
- File naming: `foo_test.go` tests `foo.go`
- Test helpers that are shared across tests in a package go in `helpers_test.go`
