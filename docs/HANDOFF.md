# Handoff: Perpetual R&D Loop — WSL/Ubuntu Session

## Branch
```bash
git clone git@github.com:hairglasses-studio/mcpkit.git
cd mcpkit
git checkout feature/rdcycle-auto-2
```

## What Was Built
Perpetual task synthesis + ralph loop hardening for autonomous R&D cycles.

### New Components
| File | Purpose |
|------|---------|
| `ralph/circuitbreaker.go` | 3-state loop gate (closed/open/half_open), 10m cooldown |
| `ralph/costgovernor.go` | 3-layer cost defense: hard cap, unproductive streak, velocity alarm |
| `ralph/ralph.go` (ExitGate) | Dual-condition exit: all tasks must complete before loop accepts "done" |
| `rdcycle/learning.go` | Extracts avoid patterns, cost trends, task mutations from history |
| `rdcycle/strategy.go` | Adaptive strategy selection (full/maintenance/recovery/ecosystem) |
| `rdcycle/synthesizer.go` | Multi-source task synthesis with DAG scaffolding |
| `rdcycle/source_roadmap.go` | TaskSource: fetches from next roadmap phase |
| `rdcycle/source_improve.go` | TaskSource: generates fix/self-improve tasks |
| `rdcycle/orchestrator.go` | Uses adaptive Synthesizer with legacy fallback |
| `cmd/rdloop/runner.go` | Composes all components, reads cost downgrade signals |
| `scripts/rdloop.sh` | Launch script with 1Password integration |

### All tests pass
```bash
go build ./... && go vet ./... && go test ./... -count=1 -short
```

## Secrets (1Password CLI)

| Env Var | Description |
|---------|-------------|
| ANTHROPIC_API_KEY | Anthropic API key |
| GITHUB_TOKEN / MCPKIT_TOKEN | GitHub PAT with repo scope |
| PERSONAL_CLAUDE_MAX_ANTHROPIC_API_KEY | Secondary Anthropic key (optional) |

### Retrieve via 1Password CLI
```bash
# Install 1Password CLI first: https://developer.1password.com/docs/cli/get-started
op signin --account <your-account>.1password.com

export ANTHROPIC_API_KEY=$(op item get "<api-key-item>" \
    --vault <vault> --fields credential --reveal)
export GITHUB_TOKEN=$(op item get "<github-pat-item>" \
    --vault <vault> --fields credential --reveal)
```

## Launch (12hr, $100 budget)

### Option A: Script (handles secrets automatically)
```bash
./scripts/rdloop.sh --budget 100 --duration 12h
```

### Option B: Manual
```bash
export ANTHROPIC_API_KEY="..."  # from 1Password
export GITHUB_TOKEN="..."       # from 1Password
export RDLOOP_BUDGET=100.0
export RDLOOP_DURATION=12h
go run ./cmd/rdloop 2>&1 | tee rdloop_$(date +%Y%m%d_%H%M).log
```

## Safety Layers (7 total)
1. **Ralph CircuitBreaker** — opens after 5 no-progress or 5 same-error iters, 10m cooldown
2. **Ralph CostGovernor** — hard token cap, unproductive streak halt (5), velocity alarm
3. **ExitGate** — won't accept "complete" unless all tasks done
4. **rdcycle CircuitBreaker** — cross-cycle breaker
5. **CostVelocityGovernor** — cross-cycle cost monitoring
6. **Runner fail limit** — 3 consecutive failures = stop
7. **Global budget** — $100 hard cap

## Monitoring
```bash
tail -f rdloop_*.log                                    # live output
cat .rdloop_state.json | python3 -m json.tool           # cycle history
cat rdcycle/notes/improvement_log.json | python3 -m json.tool  # learning
```

## After the Run
1. `git diff` to review autonomous changes
2. Check `.rdloop_state.json` for cost/iterations
3. If good: commit, push, PR to main
