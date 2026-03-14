---
name: fix-issue
description: End-to-end workflow to fix a GitHub issue — read, branch, implement, test, commit, PR
argument-hint: [issue-number]
disable-model-invocation: true
---

# Fix a GitHub Issue

Complete workflow from issue to pull request.

## Steps

1. **Read the issue**:
   ```bash
   gh issue view $ARGUMENTS
   ```

2. **Create a feature branch**:
   ```bash
   git checkout -b fix/$ARGUMENTS
   ```

3. **Implement the fix** based on the issue description:
   - Follow the coding conventions in `CLAUDE.md`
   - Follow the rules in `.claude/rules/`
   - Write or update tests

4. **Verify**:
   ```bash
   make check
   ```

5. **Commit** the changes with a message referencing the issue:
   ```
   Fix #$ARGUMENTS: <short description>
   ```

6. **Create a pull request**:
   ```bash
   gh pr create --title "Fix #$ARGUMENTS: <short description>" --body "Closes #$ARGUMENTS\n\n## Changes\n<bullet list>\n\n## Test plan\n<how to verify>"
   ```

7. **Report** the PR URL
