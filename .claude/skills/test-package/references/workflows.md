# mcpkit Workflow Front Doors

Start with the workflow that best matches the requested outcome, then move to the narrower package references only after the entrypoint is clear.

## High priority

- Framework mapping
  - Use when the request is about package boundaries, dependency layering, migration impact, or framework-wide behavior changes.
  - Canonical skill: `mcpkit`
  - Compatibility alias: `fix-issue`
- Tool scaffolding
  - Use when adding a new tool, handler, registry module, or package skeleton.
  - Canonical skill: `mcpkit`
  - Compatibility aliases: `mcp-tool-scaffold`, `new-tool`

## Medium priority

- Package testing
  - Use when the first task is choosing the smallest valid test scope before running broader checks.
  - Canonical skill: `mcpkit`
  - Compatibility alias: `test-package`
- Issue work
  - Use when triaging or fixing a concrete framework bug with compatibility and regression review.
  - Canonical skill: `mcpkit`
  - Compatibility alias: `fix-issue`
- Go API reference
  - Use when the question is about typed handlers, registries, middleware, result builders, or package contracts.
  - Canonical skill: `mcpkit-go`

## Lower priority

- Go conventions review
  - Use when the work is mostly repo-wide review, style, or shared Go implementation hygiene.
  - Canonical skill: `go-conventions`
