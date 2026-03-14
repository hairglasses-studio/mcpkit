---
name: test-package
description: Run tests for a specific mcpkit package with verbose output and coverage
argument-hint: [package-path]
allowed-tools: Bash, Read
---

# Run Package Tests

Execute tests for the specified package and report results.

## Steps

1. Run the tests:
   ```bash
   go test ./$ARGUMENTS -count=1 -v -cover
   ```

2. If tests fail:
   - Read the failing test file to understand the test intent
   - Report which tests failed, the error messages, and the relevant line numbers
   - Suggest fixes if the cause is apparent

3. If tests pass:
   - Report the pass count and coverage percentage
