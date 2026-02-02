#!/bin/bash
# Benchmark: Atomic vs Beads Task Breakdown
#
# Compares two approaches for multi-agent PRD execution:
# - ATOMIC: One file per task, maximum parallelism potential
# - BEADS: Phase-based, progressive integration
#
# Usage:
#   ./benchmark.sh --prd benchmark-prd.md --agents 5
#   ./benchmark.sh --prd benchmark-prd.md --agents 10 --runs 3

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Defaults
AGENT_COUNT=5
PRD_FILE="$SCRIPT_DIR/benchmark-prd.md"
RUNS=1
STORE="${SCRAPS_DEMO_STORE:-}"

# Results storage
RESULTS_DIR="$SCRIPT_DIR/benchmark-results"
mkdir -p "$RESULTS_DIR"

cleanup() {
    echo ""
    echo -e "${YELLOW}Stopping agents...${NC}"
    jobs -p | xargs -r kill 2>/dev/null || true
    wait 2>/dev/null || true
}

trap cleanup EXIT

print_banner() {
    echo -e "${BOLD}${CYAN}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                                                            ║"
    echo "║   📊 BENCHMARK: Atomic vs Beads Task Breakdown 📊         ║"
    echo "║                                                            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

show_help() {
    echo "Benchmark: Compare Atomic vs Beads task breakdown approaches"
    echo ""
    echo "Usage: ./benchmark.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --prd FILE      PRD file to benchmark (default: benchmark-prd.md)"
    echo "  --agents N      Number of parallel agents (default: 5)"
    echo "  --runs N        Number of runs per approach (default: 1)"
    echo "  --atomic-only   Only run atomic approach"
    echo "  --beads-only    Only run beads approach"
    echo "  --help          Show this help"
}

check_prereqs() {
    echo -e "${BLUE}Checking prerequisites...${NC}"

    if [ -z "$SCRAPS_API_KEY" ]; then
        if command -v scraps &> /dev/null && scraps whoami &> /dev/null; then
            echo -e "${GREEN}✓ Authenticated via scraps CLI${NC}"
        else
            echo -e "${RED}Error: SCRAPS_API_KEY not set${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}✓ SCRAPS_API_KEY configured${NC}"
    fi

    if [ -z "$OPENROUTER_API_KEY" ]; then
        echo -e "${RED}Error: OPENROUTER_API_KEY not set${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ OPENROUTER_API_KEY configured${NC}"

    if [ ! -f "$PRD_FILE" ]; then
        echo -e "${RED}Error: PRD file not found: $PRD_FILE${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ PRD file: $(basename "$PRD_FILE")${NC}"

    # Setup Python
    PARENT_VENV="$SCRIPT_DIR/../streaming-agent/.venv"
    LOCAL_VENV="$SCRIPT_DIR/.venv"

    if [ -d "$PARENT_VENV" ]; then
        source "$PARENT_VENV/bin/activate"
    elif [ -d "$LOCAL_VENV" ]; then
        source "$LOCAL_VENV/bin/activate"
    else
        python3 -m venv "$LOCAL_VENV"
        source "$LOCAL_VENV/bin/activate"
        pip install -q openai httpx
    fi
    echo -e "${GREEN}✓ Python environment ready${NC}"
}

get_store() {
    if [ -z "$STORE" ]; then
        if command -v scraps &> /dev/null; then
            STORE=$(scraps whoami -o json 2>/dev/null | python3 -c "import sys, json; print(json.load(sys.stdin).get('username', ''))" 2>/dev/null || echo "")
        fi
    fi
    echo -e "${GREEN}✓ Store: $STORE${NC}"
}

run_atomic() {
    local run_num=$1
    local result_file="$RESULTS_DIR/atomic-run-$run_num.json"

    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}ATOMIC Approach - Run $run_num${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"

    # Create unique repo
    REPO="bench-atomic-$(date +%H%M%S)"
    echo -e "${CYAN}Repository: $STORE/$REPO${NC}"

    if command -v scraps &> /dev/null; then
        scraps repo create "$STORE/$REPO" 2>/dev/null || true
    else
        curl -s -X POST "https://api.scraps.sh/api/v1/stores/$STORE/repos" \
            -H "Authorization: Bearer $SCRAPS_API_KEY" \
            -H "Content-Type: application/json" \
            -d "{\"name\": \"$REPO\"}" > /dev/null
    fi

    # Run orchestrator
    echo ""
    echo -e "${YELLOW}Phase 1: Atomic Orchestrator${NC}"
    local orch_start=$(date +%s)

    TARGET_TASKS=$((AGENT_COUNT * 2)) AGENT_ID="orchestrator" \
        python3 "$SCRIPT_DIR/orchestrator.py" "$STORE" "$REPO" "$PRD_FILE" 2>&1 | \
        tee "$RESULTS_DIR/atomic-orch-$run_num.log"

    local orch_end=$(date +%s)
    local orch_time=$((orch_end - orch_start))

    # Run workers
    echo ""
    echo -e "${YELLOW}Phase 2: Workers ($AGENT_COUNT agents)${NC}"
    local work_start=$(date +%s)

    pids=()
    for i in $(seq 1 "$AGENT_COUNT"); do
        AGENT_ID="agent-$i" python3 "$SCRIPT_DIR/worker.py" "$STORE" "$REPO" \
            >> "$RESULTS_DIR/atomic-worker-$run_num.log" 2>&1 &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local work_end=$(date +%s)
    local work_time=$((work_end - work_start))
    local total_time=$((work_end - orch_start))

    # Count results (use || true to handle no-match case, then default to 0)
    local tasks_completed
    tasks_completed=$(grep -c "Task completed" "$RESULTS_DIR/atomic-worker-$run_num.log" 2>/dev/null || true)
    tasks_completed=${tasks_completed:-0}

    local errors
    errors=$(grep -c "Concurrent modification" "$RESULTS_DIR/atomic-worker-$run_num.log" 2>/dev/null || true)
    errors=${errors:-0}

    # Save results
    cat > "$result_file" << EOF
{
    "approach": "atomic",
    "run": $run_num,
    "repo": "$STORE/$REPO",
    "agents": $AGENT_COUNT,
    "orchestrator_time": $orch_time,
    "worker_time": $work_time,
    "total_time": $total_time,
    "tasks_completed": $tasks_completed,
    "concurrent_errors": $errors
}
EOF

    echo ""
    echo -e "${GREEN}ATOMIC Run $run_num Complete:${NC}"
    echo "  Total time: ${total_time}s"
    echo "  Tasks completed: $tasks_completed"
    echo "  Concurrency errors: $errors"
    echo "  Repo: $STORE/$REPO"
}

run_beads() {
    local run_num=$1
    local result_file="$RESULTS_DIR/beads-run-$run_num.json"

    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}BEADS Approach - Run $run_num${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"

    # Create unique repo
    REPO="bench-beads-$(date +%H%M%S)"
    echo -e "${CYAN}Repository: $STORE/$REPO${NC}"

    if command -v scraps &> /dev/null; then
        scraps repo create "$STORE/$REPO" 2>/dev/null || true
    else
        curl -s -X POST "https://api.scraps.sh/api/v1/stores/$STORE/repos" \
            -H "Authorization: Bearer $SCRAPS_API_KEY" \
            -H "Content-Type: application/json" \
            -d "{\"name\": \"$REPO\"}" > /dev/null
    fi

    # Run beads orchestrator
    echo ""
    echo -e "${YELLOW}Phase 1: Beads Orchestrator${NC}"
    local orch_start=$(date +%s)

    TARGET_PHASES=5 TASKS_PER_PHASE=$AGENT_COUNT AGENT_ID="orchestrator" \
        python3 "$SCRIPT_DIR/orchestrator_beads.py" "$STORE" "$REPO" "$PRD_FILE" 2>&1 | \
        tee "$RESULTS_DIR/beads-orch-$run_num.log"

    local orch_end=$(date +%s)
    local orch_time=$((orch_end - orch_start))

    # Run beads workers
    echo ""
    echo -e "${YELLOW}Phase 2: Beads Workers ($AGENT_COUNT agents)${NC}"
    local work_start=$(date +%s)

    pids=()
    for i in $(seq 1 "$AGENT_COUNT"); do
        AGENT_ID="agent-$i" python3 "$SCRIPT_DIR/worker_beads.py" "$STORE" "$REPO" \
            >> "$RESULTS_DIR/beads-worker-$run_num.log" 2>&1 &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local work_end=$(date +%s)
    local work_time=$((work_end - work_start))
    local total_time=$((work_end - orch_start))

    # Count results (use || true to handle no-match case, then default to 0)
    local tasks_completed
    tasks_completed=$(grep -c "Task completed" "$RESULTS_DIR/beads-worker-$run_num.log" 2>/dev/null || true)
    tasks_completed=${tasks_completed:-0}

    local errors
    errors=$(grep -c "Concurrent modification" "$RESULTS_DIR/beads-worker-$run_num.log" 2>/dev/null || true)
    errors=${errors:-0}

    # Save results
    cat > "$result_file" << EOF
{
    "approach": "beads",
    "run": $run_num,
    "repo": "$STORE/$REPO",
    "agents": $AGENT_COUNT,
    "orchestrator_time": $orch_time,
    "worker_time": $work_time,
    "total_time": $total_time,
    "tasks_completed": $tasks_completed,
    "concurrent_errors": $errors
}
EOF

    echo ""
    echo -e "${GREEN}BEADS Run $run_num Complete:${NC}"
    echo "  Total time: ${total_time}s"
    echo "  Tasks completed: $tasks_completed"
    echo "  Concurrency errors: $errors"
    echo "  Repo: $STORE/$REPO"
}

print_summary() {
    echo ""
    echo -e "${BOLD}${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${CYAN}                    BENCHMARK SUMMARY                        ${NC}"
    echo -e "${BOLD}${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Parse and display results
    python3 << 'PYTHON'
import json
import glob
import os

results_dir = os.environ.get('RESULTS_DIR', 'benchmark-results')

atomic_results = []
beads_results = []

for f in glob.glob(f"{results_dir}/atomic-run-*.json"):
    with open(f) as fp:
        atomic_results.append(json.load(fp))

for f in glob.glob(f"{results_dir}/beads-run-*.json"):
    with open(f) as fp:
        beads_results.append(json.load(fp))

def avg(lst, key):
    if not lst:
        return 0
    return sum(r[key] for r in lst) / len(lst)

print("┌─────────────────┬────────────┬────────────┐")
print("│ Metric          │   ATOMIC   │   BEADS    │")
print("├─────────────────┼────────────┼────────────┤")
print(f"│ Avg Total Time  │ {avg(atomic_results, 'total_time'):>8.1f}s  │ {avg(beads_results, 'total_time'):>8.1f}s  │")
print(f"│ Avg Tasks Done  │ {avg(atomic_results, 'tasks_completed'):>10.1f} │ {avg(beads_results, 'tasks_completed'):>10.1f} │")
print(f"│ Avg Errors      │ {avg(atomic_results, 'concurrent_errors'):>10.1f} │ {avg(beads_results, 'concurrent_errors'):>10.1f} │")
print(f"│ Runs            │ {len(atomic_results):>10} │ {len(beads_results):>10} │")
print("└─────────────────┴────────────┴────────────┘")

# Determine winner
if atomic_results and beads_results:
    atomic_score = avg(atomic_results, 'tasks_completed') / max(avg(atomic_results, 'total_time'), 1)
    beads_score = avg(beads_results, 'tasks_completed') / max(avg(beads_results, 'total_time'), 1)

    print()
    if beads_score > atomic_score * 1.1:
        print("🏆 BEADS wins! (better task completion rate)")
    elif atomic_score > beads_score * 1.1:
        print("🏆 ATOMIC wins! (faster overall)")
    else:
        print("🤝 Roughly equivalent performance")

    print()
    print("Tasks/second:")
    print(f"  ATOMIC: {atomic_score:.3f}")
    print(f"  BEADS:  {beads_score:.3f}")
PYTHON

    echo ""
    echo "Detailed results in: $RESULTS_DIR/"
}

main() {
    local run_atomic=true
    local run_beads=true

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_help
                exit 0
                ;;
            --prd)
                PRD_FILE="$2"
                if [[ ! "$PRD_FILE" = /* ]]; then
                    PRD_FILE="$(pwd)/$PRD_FILE"
                fi
                shift 2
                ;;
            --agents)
                AGENT_COUNT="$2"
                shift 2
                ;;
            --runs)
                RUNS="$2"
                shift 2
                ;;
            --atomic-only)
                run_beads=false
                shift
                ;;
            --beads-only)
                run_atomic=false
                shift
                ;;
            *)
                echo "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    print_banner
    check_prereqs
    get_store

    echo ""
    echo -e "${CYAN}Configuration:${NC}"
    echo "  PRD: $(basename "$PRD_FILE")"
    echo "  Agents: $AGENT_COUNT"
    echo "  Runs per approach: $RUNS"
    echo ""

    # Clear previous results
    rm -f "$RESULTS_DIR"/*.json "$RESULTS_DIR"/*.log 2>/dev/null || true

    # Run benchmarks
    for run in $(seq 1 "$RUNS"); do
        if $run_atomic; then
            run_atomic "$run"
            sleep 5  # Brief pause between runs
        fi

        if $run_beads; then
            run_beads "$run"
            sleep 5
        fi
    done

    # Print summary
    RESULTS_DIR="$RESULTS_DIR" print_summary
}

main "$@"
