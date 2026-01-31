#!/bin/bash
# Parallel Agent Demo - 20 agents, zero dependencies, maximum speed
#
# This demo shows TRUE parallelization:
# - 20 pre-generated tasks with NO dependencies
# - 20 workers start simultaneously
# - All agents write code at the same time
# - Watch with: scraps watch <store>/<repo>
#
# Usage:
#   ./demo-parallel.sh              # Run with 20 workers
#   ./demo-parallel.sh --workers 10 # Run with custom worker count

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

# Default: 20 workers for 20 tasks
WORKER_COUNT=20
STORE="${SCRAPS_DEMO_STORE:-}"

cleanup() {
    echo ""
    echo -e "${YELLOW}Stopping workers...${NC}"
    jobs -p | xargs -r kill 2>/dev/null || true
    wait 2>/dev/null || true
    echo -e "${GREEN}Done!${NC}"
}

trap cleanup EXIT

print_banner() {
    echo -e "${BOLD}${CYAN}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                                                            ║"
    echo "║   ⚡ PARALLEL AGENT SWARM DEMO ⚡                          ║"
    echo "║                                                            ║"
    echo "║   20 agents writing code simultaneously                    ║"
    echo "║                                                            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

check_prereqs() {
    echo -e "${BLUE}Checking prerequisites...${NC}"

    if ! command -v scraps &> /dev/null; then
        echo -e "${RED}Error: scraps CLI not found${NC}"
        echo "Install: curl -fsSL https://scraps.sh/install.sh | sh"
        exit 1
    fi
    echo -e "${GREEN}✓ scraps CLI${NC}"

    if ! scraps whoami &> /dev/null; then
        echo -e "${RED}Error: Not logged in. Run: scraps login${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Authenticated${NC}"

    if [ -z "$OPENROUTER_API_KEY" ]; then
        echo -e "${RED}Error: OPENROUTER_API_KEY not set${NC}"
        echo "Get one at: https://openrouter.ai/keys"
        exit 1
    fi
    echo -e "${GREEN}✓ OpenRouter API key${NC}"

    # Setup Python
    VENV_DIR="$SCRIPT_DIR/.venv"
    if [ ! -d "$VENV_DIR" ]; then
        python3 -m venv "$VENV_DIR"
    fi
    source "$VENV_DIR/bin/activate"
    if ! python3 -c "import openai" 2>/dev/null; then
        pip install -q -r "$SCRIPT_DIR/requirements.txt"
    fi
    echo -e "${GREEN}✓ Python environment${NC}"
}

get_store() {
    if [ -z "$STORE" ]; then
        STORE=$(scraps whoami -o json 2>/dev/null | python3 -c "import sys, json; print(json.load(sys.stdin).get('username', ''))" 2>/dev/null || echo "")
    fi
    if [ -z "$STORE" ]; then
        echo -e "${RED}Could not determine store name${NC}"
        exit 1
    fi
}

main() {
    # Parse args
    while [[ $# -gt 0 ]]; do
        case $1 in
            --workers) WORKER_COUNT="$2"; shift 2 ;;
            --help|-h)
                echo "Usage: ./demo-parallel.sh [--workers N]"
                echo "  --workers N   Number of parallel workers (default: 20)"
                exit 0
                ;;
            *) echo "Unknown option: $1"; exit 1 ;;
        esac
    done

    print_banner
    check_prereqs
    get_store

    # Create unique repo
    REPO="swarm-$(date +%H%M%S)"
    echo ""
    echo -e "${BLUE}Creating repository: ${BOLD}$STORE/$REPO${NC}"
    scraps repo create "$STORE/$REPO"

    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}Watch the swarm in another terminal:${NC}"
    echo ""
    echo -e "  ${BOLD}${YELLOW}scraps watch $STORE/$REPO${NC}"
    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Generate tasks (instant, no LLM)
    echo -e "${BLUE}Generating 20 parallel tasks...${NC}"
    python3 "$SCRIPT_DIR/generate_tasks.py" "$STORE" "$REPO"

    echo ""
    echo -e "${BOLD}${GREEN}Launching $WORKER_COUNT workers...${NC}"
    echo ""

    # Launch all workers simultaneously
    pids=()
    for i in $(seq 1 "$WORKER_COUNT"); do
        AGENT_ID="agent-$i" python3 "$SCRIPT_DIR/worker.py" "$STORE" "$REPO" &
        pids+=($!)
        echo -e "${GREEN}  Started agent-$i${NC}"
    done

    echo ""
    echo -e "${BOLD}${YELLOW}$WORKER_COUNT agents now working in parallel!${NC}"
    echo -e "${CYAN}Watch progress: scraps watch $STORE/$REPO${NC}"
    echo ""

    # Wait for completion
    start_time=$(date +%s)

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    end_time=$(date +%s)
    duration=$((end_time - start_time))

    echo ""
    echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${GREEN}  COMPLETE! $WORKER_COUNT agents finished in ${duration}s${NC}"
    echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "View results: ${CYAN}scraps repo show $STORE/$REPO${NC}"
    echo -e "Clone repo:   ${CYAN}scraps clone $STORE/$REPO${NC}"
    echo ""
}

main "$@"
