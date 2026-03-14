---
name: new-tool
description: Scaffold a new MCP tool with input/output structs, handler, registration, and tests
argument-hint: [tool-name] [package]
allowed-tools: Read, Edit, Write, Glob, Grep, Bash
---

# Scaffold a New MCP Tool

Create a complete MCP tool implementation in the specified package.

## Steps

1. **Read the target package** to understand existing patterns:
   - `Glob` for `$ARGUMENTS[1]/*.go` to see existing files
   - `Read` the package's `CLAUDE.md` if it exists
   - `Read` existing tool definitions in the package for style reference

2. **Create the tool file** `$ARGUMENTS[1]/$ARGUMENTS[0].go`:
   - Define input struct with `json` and `jsonschema` tags (see `.claude/rules/tool-design.md`)
   - Define output struct if the tool returns structured data
   - Implement the handler function returning `(*mcp.CallToolResult, nil)` — never `(nil, err)`
   - Use `handler.CodedErrorResult()` with constants from `handler/result.go` for errors
   - Write a clear, detailed tool description (treat it as onboarding docs)

3. **Register the tool** in the package's `ToolModule.Tools()` or equivalent registration function

4. **Create test file** `$ARGUMENTS[1]/$ARGUMENTS[0]_test.go`:
   - Integration test using `mcptest.NewServer` / `mcptest.NewClient`
   - Test the happy path and at least one error case
   - Use `mcptest.Assert*` helpers

5. **Run tests** to verify:
   ```bash
   go test ./$ARGUMENTS[1] -count=1 -v
   ```

6. **Report** the tool name, description, and test results
