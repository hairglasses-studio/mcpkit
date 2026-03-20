#!/usr/bin/env bash
set -euo pipefail

# Perpetual R&D loop launcher for mcpkit (24hr marathon mode)
# Usage: ./scripts/rdloop.sh [options]
#
#   -b, --budget NUM      Global dollar budget (default: 200.0)
#   -d, --duration DUR    Max runtime (default: 24h)
#   -m, --model MODEL     Claude model (default: claude-sonnet-4-6)
#   -s, --spec PATH       Initial spec file (default: rdcycle/specs/rd_cycle.json)
#   -r, --roadmap PATH    Roadmap file (default: roadmap.json)
#   --dry-run             Print config and exit without running

BUDGET=200.0
DURATION=24h
MODEL=claude-sonnet-4-6
SPEC=rdcycle/specs/rd_cycle.json
ROADMAP=roadmap.json
STATE=.rdloop_state.json
DRY_RUN=false
MAX_RESTARTS=3

while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--budget)   BUDGET="$2"; shift 2 ;;
        -d|--duration) DURATION="$2"; shift 2 ;;
        -m|--model)    MODEL="$2"; shift 2 ;;
        -s|--spec)     SPEC="$2"; shift 2 ;;
        -r|--roadmap)  ROADMAP="$2"; shift 2 ;;
        --state)       STATE="$2"; shift 2 ;;
        --dry-run)     DRY_RUN=true; shift ;;
        -h|--help)
            sed -n '3,12p' "$0"
            exit 0
            ;;
        *) echo "error: unknown option $1" >&2; exit 1 ;;
    esac
done

# Resolve paths.
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# ---------- Environment isolation ----------
# Prevent Claude Code / telemetry env vars from leaking into the autonomous loop.
unset CLAUDECODE CLAUDE_CODE_ENABLE_TELEMETRY CLAUDE_CODE_ENTRYPOINT 2>/dev/null || true

# ---------- Secret resolution ----------
# Resolve API keys: env var > .env (op:// references or plain values) > op CLI
USE_OP_RUN=false
if [[ -f "$REPO_ROOT/.env" ]]; then
    if grep -q 'op://' "$REPO_ROOT/.env" 2>/dev/null; then
        # .env contains op:// secret references — resolve via `op run`.
        if command -v op &>/dev/null; then
            USE_OP_RUN=true
            echo "Resolving op:// secret references from .env..."
        else
            echo "error: .env contains op:// references but op CLI is not installed." >&2
            exit 1
        fi
    else
        # Plain key=value .env — source directly.
        set -a
        # shellcheck disable=SC1091
        source "$REPO_ROOT/.env"
        set +a
    fi
fi

# If not using op run, fall back to env vars or interactive op CLI.
if [[ "$USE_OP_RUN" != "true" ]]; then
    if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
        if command -v op &>/dev/null && op whoami &>/dev/null; then
            echo "Loading ANTHROPIC_API_KEY from 1Password..."
            export ANTHROPIC_API_KEY=$(op item get "Anthropic API Key (Work - 10K credits)" \
                --account my.1password.com --vault Personal --fields password --reveal)
        else
            echo "error: ANTHROPIC_API_KEY not set. Set it via env, .env file, or configure 1Password CLI." >&2
            exit 1
        fi
    fi

    if [[ -z "${GITHUB_TOKEN:-}" ]]; then
        if command -v op &>/dev/null && op whoami &>/dev/null; then
            echo "Loading GITHUB_TOKEN from 1Password..."
            export GITHUB_TOKEN=$(op item get "AFTRS MCP - mcpkit GitHub PAT" \
                --account my.1password.com --vault Personal --fields credential --reveal)
        else
            echo "warning: GITHUB_TOKEN not set, ecosystem scans will have lower rate limits" >&2
        fi
    fi
fi

# ---------- Pre-flight checks ----------
preflight_checks() {
    echo "=== Shell pre-flight checks ==="

    # Disk space: require 5GB free.
    local free_kb
    free_kb=$(df -k . | awk 'NR==2 {print $4}')
    local free_gb=$(( free_kb / 1048576 ))
    if [[ $free_gb -lt 5 ]]; then
        echo "  WARNING: only ${free_gb}GB free disk space (recommend 5+ GB)"
    else
        echo "  disk: ${free_gb}GB free"
    fi

    # State file: report if resuming.
    if [[ -f "$STATE" ]]; then
        local cycles iters cost
        cycles=$(python3 -c "import json; print(json.load(open('$STATE'))['cycle_number'])" 2>/dev/null || echo "?")
        cost=$(python3 -c "import json; print(f\"{json.load(open('$STATE'))['total_cost']:.2f}\")" 2>/dev/null || echo "?")
        echo "  state: resuming (cycle $cycles, \$$cost spent)"
    else
        echo "  state: fresh start"
    fi

    # Network: test API reachability.
    if curl -sf --max-time 5 -o /dev/null https://api.anthropic.com 2>/dev/null; then
        echo "  api: reachable"
    else
        echo "  WARNING: api.anthropic.com may be unreachable"
    fi

    echo ""
}

# ---------- Build ----------
BINARY="$REPO_ROOT/rdloop"
echo "Building rdloop binary..."
if ! go build -o "$BINARY" ./cmd/rdloop; then
    echo "error: build failed — run 'make check' first" >&2
    exit 1
fi
echo "Build OK."
echo ""

preflight_checks

# ---------- Log rotation ----------
mkdir -p "$REPO_ROOT/logs"
# Keep last 10 log files, remove older ones.
# shellcheck disable=SC2012
ls -t "$REPO_ROOT"/logs/rdloop_*.log 2>/dev/null | tail -n +11 | xargs -r rm --

LOGFILE="$REPO_ROOT/logs/rdloop_$(date +%Y%m%d_%H%M).log"

# ---------- Export config ----------
export RDLOOP_BUDGET="$BUDGET"
export RDLOOP_DURATION="$DURATION"
export RDLOOP_MODEL="$MODEL"
export RDLOOP_SPEC="$SPEC"
export RDLOOP_ROADMAP="$ROADMAP"
export RDLOOP_STATE="$STATE"

echo "=== mcpkit perpetual R&D loop ==="
echo "  budget:   \$$BUDGET"
echo "  duration: $DURATION"
echo "  model:    $MODEL"
echo "  spec:     $SPEC"
echo "  roadmap:  $ROADMAP"
echo "  state:    $STATE"
echo "  log:      $LOGFILE"
echo "  restarts: max $MAX_RESTARTS"
echo "  secrets:  $(if [[ "$USE_OP_RUN" == "true" ]]; then echo "op run (op:// refs)"; else echo "env vars"; fi)"
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    echo "(dry run — exiting without starting)"
    exit 0
fi

echo "Starting in 3s... (Ctrl+C to abort)"
sleep 3

# ---------- Supervisor loop ----------
# Restart on crash up to MAX_RESTARTS times, with exponential backoff.
# Check state file for budget exhaustion before each restart.
restart_count=0
while true; do
    echo "=== rdloop process starting (attempt $((restart_count + 1))/$((MAX_RESTARTS + 1))) ===" | tee -a "$LOGFILE"

    exit_code=0
    if [[ "$USE_OP_RUN" == "true" ]]; then
        op run --env-file="$REPO_ROOT/.env" -- "$BINARY" 2>&1 | tee -a "$LOGFILE" || exit_code=$?
    else
        "$BINARY" 2>&1 | tee -a "$LOGFILE" || exit_code=$?
    fi

    # Clean exit (0) = budget exhausted or duration reached — don't restart.
    if [[ $exit_code -eq 0 ]]; then
        echo "rdloop exited cleanly." | tee -a "$LOGFILE"
        break
    fi

    # Check if budget is exhausted before restarting.
    if [[ -f "$STATE" ]]; then
        budget_exhausted=$(python3 -c "
import json, sys
s=json.load(open('$STATE'))
sys.exit(0 if s['total_cost'] >= $BUDGET else 1)
" 2>/dev/null && echo "true" || echo "false")
        if [[ "$budget_exhausted" == "true" ]]; then
            echo "Budget exhausted — not restarting." | tee -a "$LOGFILE"
            break
        fi
    fi

    restart_count=$((restart_count + 1))
    if [[ $restart_count -gt $MAX_RESTARTS ]]; then
        echo "Max restarts ($MAX_RESTARTS) exceeded — giving up." | tee -a "$LOGFILE"
        exit 1
    fi

    # Exponential backoff: 30s, 60s, 120s.
    backoff_secs=$((30 * (1 << (restart_count - 1))))
    if [[ $backoff_secs -gt 120 ]]; then
        backoff_secs=120
    fi
    echo "rdloop crashed (exit $exit_code). Restarting in ${backoff_secs}s... ($restart_count/$MAX_RESTARTS)" | tee -a "$LOGFILE"
    sleep "$backoff_secs"
done

echo "=== rdloop session complete ===" | tee -a "$LOGFILE"
