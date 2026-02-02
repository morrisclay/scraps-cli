#!/bin/bash
# Benchmark: Beads Swarm Scaling
#
# Tests how completion time scales with different agent counts.
#
# Usage:
#   ./benchmark.sh                           # Default: 5, 10, 25 agents
#   ./benchmark.sh --agents "5 10 25 50"     # Custom agent counts
#   ./benchmark.sh --prd custom.md           # Custom PRD

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
AGENT_COUNTS="5 10 25"
PRD_FILE="$SCRIPT_DIR/sample-prd.md"
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
    echo "║   📊 BEADS SWARM SCALING BENCHMARK 📊                     ║"
    echo "║                                                            ║"
    echo "║   Measuring completion time vs agent count                 ║"
    echo "║                                                            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

show_help() {
    echo "Benchmark: Beads Swarm Scaling"
    echo ""
    echo "Usage: ./benchmark.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --prd FILE         PRD file to benchmark (default: sample-prd.md)"
    echo "  --agents \"N1 N2\"   Space-separated agent counts (default: \"5 10 25\")"
    echo "  --help             Show this help"
    echo ""
    echo "Examples:"
    echo "  ./benchmark.sh                           # Test with 5, 10, 25 agents"
    echo "  ./benchmark.sh --agents \"5 10 25 50\"     # Custom counts"
}

check_prereqs() {
    echo -e "${BLUE}Checking prerequisites...${NC}"

    # Check for bd CLI
    if ! command -v bd &> /dev/null; then
        echo -e "${RED}Error: bd (Beads) CLI not found${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Beads CLI installed${NC}"

    # Check scraps
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

    # Check OpenRouter
    if [ -z "$OPENROUTER_API_KEY" ]; then
        echo -e "${RED}Error: OPENROUTER_API_KEY not set${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ OPENROUTER_API_KEY configured${NC}"

    # Check PRD file
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

run_benchmark() {
    local agent_count=$1
    local run_id=$(date +%H%M%S)
    local result_file="$RESULTS_DIR/agents-${agent_count}-${run_id}.json"
    local log_file="$RESULTS_DIR/agents-${agent_count}-${run_id}.log"

    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}Running with ${BOLD}$agent_count agents${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"

    # Create unique repo
    REPO="bench-${agent_count}a-${run_id}"
    echo -e "${CYAN}Repository: $STORE/$REPO${NC}"

    if command -v scraps &> /dev/null; then
        scraps repo create "$STORE/$REPO" 2>/dev/null || true
    else
        curl -s -X POST "https://api.scraps.sh/api/v1/stores/$STORE/repos" \
            -H "Authorization: Bearer $SCRAPS_API_KEY" \
            -H "Content-Type: application/json" \
            -d "{\"name\": \"$REPO\"}" > /dev/null
    fi

    # Create beads work directory
    local beads_dir=$(mktemp -d -t beads-bench-XXXXXX)
    export BEADS_WORK_DIR="$beads_dir"

    # Calculate target tasks (same formula as run.sh)
    local target_tasks=$((agent_count + agent_count / 2))
    if [ "$target_tasks" -lt 6 ]; then target_tasks=6; fi
    if [ "$target_tasks" -gt 30 ]; then target_tasks=30; fi

    # Start timing
    local start_time=$(date +%s)

    # Run orchestrator
    echo ""
    echo -e "${YELLOW}Phase 1: Orchestrator (target: $target_tasks tasks)${NC}"
    local orch_start=$(date +%s)

    TARGET_TASKS="$target_tasks" AGENT_ID="orchestrator" \
        python3 "$SCRIPT_DIR/orchestrator.py" "$STORE" "$REPO" "$PRD_FILE" 2>&1 | \
        tee "$log_file"

    local orch_end=$(date +%s)
    local orch_time=$((orch_end - orch_start))

    # Count tasks created
    local tasks_created=$(cd "$beads_dir" && bd list 2>/dev/null | grep -E "^\s*#[0-9]+" | wc -l | tr -d ' ' || echo 0)
    echo -e "${GREEN}  Created $tasks_created tasks in ${orch_time}s${NC}"

    # Run workers
    echo ""
    echo -e "${YELLOW}Phase 2: Workers ($agent_count agents)${NC}"
    local work_start=$(date +%s)

    pids=()
    for i in $(seq 1 "$agent_count"); do
        AGENT_ID="agent-$i" python3 "$SCRIPT_DIR/worker.py" "$STORE" "$REPO" \
            >> "$log_file" 2>&1 &
        pids+=($!)
    done

    # Wait for all workers
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local work_end=$(date +%s)
    local work_time=$((work_end - work_start))
    local total_time=$((work_end - start_time))

    # Count results
    local tasks_completed=$(grep -c "Task completed" "$log_file" 2>/dev/null || echo 0)
    local files_written=$(grep -c "Committed:" "$log_file" 2>/dev/null || echo 0)
    local errors=$(grep -c "Error\|Failed" "$log_file" 2>/dev/null || echo 0)

    # Save results
    cat > "$result_file" << EOF
{
    "agents": $agent_count,
    "prd": "$(basename "$PRD_FILE")",
    "repo": "$STORE/$REPO",
    "orchestrator_time": $orch_time,
    "worker_time": $work_time,
    "total_time": $total_time,
    "tasks_created": $tasks_created,
    "tasks_completed": $tasks_completed,
    "files_written": $files_written,
    "errors": $errors,
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

    echo ""
    echo -e "${GREEN}Results for $agent_count agents:${NC}"
    echo "  Orchestrator: ${orch_time}s"
    echo "  Workers: ${work_time}s"
    echo "  Total: ${total_time}s"
    echo "  Tasks: $tasks_completed/$tasks_created completed"
    echo "  Repo: $STORE/$REPO"

    # Cleanup beads dir
    rm -rf "$beads_dir"
}

print_summary() {
    echo ""
    echo -e "${BOLD}${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${CYAN}                   BENCHMARK RESULTS                         ${NC}"
    echo -e "${BOLD}${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Parse and display results
    python3 << 'PYTHON'
import json
import glob
import os

results_dir = os.environ.get('RESULTS_DIR', 'benchmark-results')

# Collect all results for this benchmark run
results = []
for f in sorted(glob.glob(f"{results_dir}/agents-*.json")):
    with open(f) as fp:
        results.append(json.load(fp))

if not results:
    print("No results found.")
    exit(0)

# Sort by agent count
results.sort(key=lambda x: x['agents'])

# Print table
print("┌─────────┬──────────┬───────────┬───────────┬────────────┬──────────┐")
print("│ Agents  │ Orch (s) │ Work (s)  │ Total (s) │ Tasks Done │ Speedup  │")
print("├─────────┼──────────┼───────────┼───────────┼────────────┼──────────┤")

baseline_time = results[0]['total_time'] if results else 1

for r in results:
    agents = r['agents']
    orch = r['orchestrator_time']
    work = r['worker_time']
    total = r['total_time']
    tasks = f"{r['tasks_completed']}/{r['tasks_created']}"

    # Calculate speedup vs first (baseline)
    if baseline_time > 0:
        speedup = baseline_time / total
    else:
        speedup = 1.0

    print(f"│ {agents:>7} │ {orch:>8} │ {work:>9} │ {total:>9} │ {tasks:>10} │ {speedup:>7.2f}x │")

print("└─────────┴──────────┴───────────┴───────────┴────────────┴──────────┘")

# Analysis
print()
print("Analysis:")
print("-" * 60)

if len(results) >= 2:
    first = results[0]
    last = results[-1]

    agent_increase = last['agents'] / first['agents']
    time_decrease = first['total_time'] / max(last['total_time'], 1)

    print(f"  • {first['agents']} → {last['agents']} agents ({agent_increase:.1f}x)")
    print(f"  • {first['total_time']}s → {last['total_time']}s ({time_decrease:.2f}x faster)")

    # Check scaling efficiency
    efficiency = time_decrease / agent_increase * 100
    print(f"  • Scaling efficiency: {efficiency:.0f}%")

    if efficiency >= 70:
        print("  • ✓ Excellent parallel scaling!")
    elif efficiency >= 50:
        print("  • ✓ Good parallel scaling")
    else:
        print("  • ⚠ Sublinear scaling (expected due to dependencies)")

# List repos for inspection
print()
print("Repositories created:")
for r in results:
    print(f"  • {r['repo']}")
PYTHON

    echo ""
    echo "Detailed logs in: $RESULTS_DIR/"
}

main() {
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
                AGENT_COUNTS="$2"
                shift 2
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
    echo "  Agent counts: $AGENT_COUNTS"
    echo ""

    # Clear previous results
    rm -f "$RESULTS_DIR"/agents-*.json "$RESULTS_DIR"/agents-*.log 2>/dev/null || true

    # Run benchmarks for each agent count
    for count in $AGENT_COUNTS; do
        run_benchmark "$count"

        # Brief pause between runs
        echo ""
        echo -e "${YELLOW}Pausing 10s before next run...${NC}"
        sleep 10
    done

    # Export for Python script
    export RESULTS_DIR

    # Print summary
    print_summary
}

main "$@"
