#!/usr/bin/env bash
set -euo pipefail

# Perpetual R&D loop launcher for mcpkit
# Usage: ./scripts/rdloop.sh [options]
#
#   -b, --budget NUM      Global dollar budget (default: 100.0)
#   -d, --duration DUR    Max runtime (default: 12h)
#   -m, --model MODEL     Claude model (default: claude-sonnet-4-6)
#   -s, --spec PATH       Initial spec file (default: rdcycle/specs/rd_cycle.json)
#   -r, --roadmap PATH    Roadmap file (default: roadmap.json)
#   --dry-run             Print config and exit without running

BUDGET=100.0
DURATION=12h
MODEL=claude-sonnet-4-6
SPEC=rdcycle/specs/rd_cycle.json
ROADMAP=roadmap.json
STATE=.rdloop_state.json
DRY_RUN=false

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
            sed -n '3,10p' "$0"
            exit 0
            ;;
        *) echo "error: unknown option $1" >&2; exit 1 ;;
    esac
done

# Resolve API keys: env var > .env file > 1Password CLI
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Source .env file if present (key=value lines, no export needed).
if [[ -f "$REPO_ROOT/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "$REPO_ROOT/.env"
    set +a
fi

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

# Ensure we're in the repo root (REPO_ROOT already set above for .env loading).
cd "$REPO_ROOT"

# Verify build
if ! go build ./cmd/rdloop/... 2>/dev/null; then
    echo "error: build failed — run 'make check' first" >&2
    exit 1
fi

LOGFILE="rdloop_$(date +%Y%m%d_%H%M).log"

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
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    echo "(dry run — exiting without starting)"
    exit 0
fi

echo "Starting in 3s... (Ctrl+C to abort)"
sleep 3

exec go run ./cmd/rdloop 2>&1 | tee "$LOGFILE"
