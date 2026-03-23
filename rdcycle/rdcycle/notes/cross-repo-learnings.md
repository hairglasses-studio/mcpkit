# Cross-Repo Ralph Loop Learnings (2026-03-16)

Source: mesmer, claudekit, ralphglasses, hg-mcp parallel research

## Critical Fixes (Apply to rdloop.sh)

### 1. Nested Session Prevention
All shell-based repos hit this. Fix before any `claude` invocation:
```bash
unset CLAUDECODE CLAUDE_CODE_ENABLE_TELEMETRY CLAUDE_CODE_ENTRYPOINT
```

### 2. ANTHROPIC_API_KEY Conflict
Claude Code uses OAuth now. Setting ANTHROPIC_API_KEY overrides OAuth. Fix:
```bash
unset ANTHROPIC_API_KEY
```

### 3. SIGPIPE on Large Binaries
`strings | grep -q` fails with pipefail on large binaries. Use `grep -c || true` instead.

## Patterns from Other Repos Worth Considering

### From mesmer: 12-Hour Launcher with Pre-flight Checks
mesmer's `.ralph/start-12hr.sh` includes:
- Budget projection before launch (loops × cost/loop vs budget)
- Disk space check (5GB minimum)
- Binary verification (strings check for expected symbols)
- Plan state check (completed/remaining task counts)
- Dry-run mode for validation without launching

### From mesmer: Adaptive Quality Gates by Role
Phase roles (builder/strategist/reconciler/deployer) determine which gates run:
- Strategist: markdown existence check only (~60% faster)
- Builder: full build+vet+test
- Deployer: build+helm lint (skip unit tests)

### From mesmer: Task Batching (3x speedup)
Batch 2-3 same-type tasks per loop. Config: `BATCH_SIMILAR_TASKS=true`, `MAX_TASKS_PER_BATCH=3`, `MAX_LINES_PER_BATCH=600`

### From ralphglasses: Environment Isolation Strategy
Explicitly unset ALL problematic env vars before spawning claude:
- CLAUDECODE, CLAUDE_CODE_ENABLE_TELEMETRY, CLAUDE_CODE_ENTRYPOINT
- ANTHROPIC_API_KEY (conflicts with OAuth)

### From claudekit: Budget-Aware Auto-Stop Hook
```go
cPolicy.OnCostUpdate = func(accumulated, limit float64, iteration int) {
    if accumulated > limit { loop.Stop() }
}
```
Already similar to rdloop's cost governor but worth comparing implementations.

### From claudekit: Verify Task Stuck-Loop Detection
When all roadmap phases complete, verify tasks can loop 3-5 times unnecessarily. Solution: mark_done immediately when check passes with no code changes.

## mcpkit-Specific Recommendations from Research
1. Add `unset CLAUDECODE` to rdloop.sh before any claude invocation
2. Consider adding pre-flight checks (disk, budget projection) to rdloop.sh
3. The stuck detection in `stuckdetect.go` is unique and valuable — other repos would benefit from this pattern
4. Per-phase budget overrides are more sophisticated than other repos — document as best practice
