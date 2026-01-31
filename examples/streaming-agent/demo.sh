#!/bin/bash
# Multi-Agent Collaboration Demo
#
# This demo shows multiple AI agents collaborating on a shared codebase:
# 1. Orchestrator - reads PRD and creates task files
# 2. Workers (N) - claim and implement tasks in parallel
# 3. Documenter - watches for completions and writes docs
#
# Usage:
#   ./demo.sh                    # Interactive setup
#   ./demo.sh --watch-only       # Just run the watcher
#   ./demo.sh --help             # Show help

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default values
STORE="${SCRAPS_DEMO_STORE:-}"
REPO="${SCRAPS_DEMO_REPO:-multi-agent-demo}"
WORKER_COUNT=2

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"

    # Kill any remaining python processes we started
    jobs -p | xargs -r kill 2>/dev/null || true

    echo -e "${GREEN}Done!${NC}"
}

trap cleanup EXIT

print_banner() {
    echo -e "${BLUE}"
    echo "+==========================================================+"
    echo "|       Multi-Agent Collaboration Demo                     |"
    echo "|                                                          |"
    echo "|   Orchestrator -> Workers (N) -> Documenter              |"
    echo "+==========================================================+"
    echo -e "${NC}"
}

check_cli() {
    if ! command -v scraps &> /dev/null; then
        echo -e "${RED}Error: scraps CLI not found${NC}"
        echo ""
        echo "Install it with:"
        echo "  curl -fsSL https://scraps.sh/install.sh | sh"
        echo ""
        exit 1
    fi
    echo -e "${GREEN}* scraps CLI found${NC}"
}

check_auth() {
    if ! scraps whoami &> /dev/null; then
        echo -e "${YELLOW}! Not logged in${NC}"
        echo ""
        echo "Run: scraps login"
        echo ""
        exit 1
    fi
    echo -e "${GREEN}* Authenticated${NC}"
}

check_openrouter() {
    if [ -z "$OPENROUTER_API_KEY" ]; then
        echo -e "${YELLOW}! OPENROUTER_API_KEY not set${NC}"
        echo "  Get one at: https://openrouter.ai/keys"
        echo ""
        echo "Set it with: export OPENROUTER_API_KEY=sk-or-..."
        exit 1
    fi
    echo -e "${GREEN}* OpenRouter API key configured${NC}"
}

check_prd() {
    if [ ! -f "$SCRIPT_DIR/prd.md" ]; then
        echo -e "${RED}Error: prd.md not found${NC}"
        echo ""
        echo "Create a Product Requirements Document at: $SCRIPT_DIR/prd.md"
        exit 1
    fi
    echo -e "${GREEN}* PRD file found${NC}"
}

setup_python_env() {
    VENV_DIR="$SCRIPT_DIR/.venv"

    if [ ! -d "$VENV_DIR" ]; then
        echo -e "${BLUE}Setting up Python environment...${NC}"
        python3 -m venv "$VENV_DIR"
    fi

    source "$VENV_DIR/bin/activate"

    if ! python3 -c "import openai" 2>/dev/null; then
        echo -e "${BLUE}Installing Python dependencies...${NC}"
        pip install -q -r "$SCRIPT_DIR/requirements.txt"
    fi
    echo -e "${GREEN}* Python environment ready${NC}"
}

get_store() {
    if [ -z "$STORE" ]; then
        STORE=$(scraps whoami -o json 2>/dev/null | python3 -c "import sys, json; print(json.load(sys.stdin).get('username', ''))" 2>/dev/null || echo "")

        if [ -z "$STORE" ]; then
            echo -e "${YELLOW}Could not determine your store name.${NC}"
            read -p "Enter your store/username: " STORE
        fi
    fi
    echo -e "${GREEN}* Using store: $STORE${NC}"
}

setup_repo() {
    echo -e "${BLUE}Setting up repository...${NC}"

    if scraps repo show "$STORE/$REPO" &> /dev/null; then
        echo "  Repo exists: $REPO"
        echo ""
        echo -e "${YELLOW}Options:${NC}"
        echo "  1) Reuse existing repo (continue where left off)"
        echo "  2) Create new repo with fresh name"
        read -p "Choice (1/2, default: 2): " choice
        if [ "$choice" = "1" ]; then
            echo "  Reusing existing repo"
        else
            # Use a new unique name
            REPO="multi-agent-$(date +%H%M%S)"
            echo "  Creating new repo: $REPO"
            scraps repo create "$STORE/$REPO"
        fi
    else
        echo "  Creating repo: $REPO"
        scraps repo create "$STORE/$REPO"
    fi

    echo -e "${GREEN}* Repository ready: $STORE/$REPO${NC}"
}

print_watch_command() {
    echo ""
    echo -e "${CYAN}============================================================${NC}"
    echo -e "${CYAN}To watch the agents work, run this in another terminal:${NC}"
    echo ""
    echo -e "  ${YELLOW}scraps watch $STORE/$REPO${NC}"
    echo ""
    echo -e "${CYAN}============================================================${NC}"
    echo ""
}

run_orchestrator() {
    echo ""
    echo -e "${BLUE}============================================================${NC}"
    echo -e "${BLUE}Phase 1: Orchestrator - Breaking PRD into tasks${NC}"
    echo -e "${BLUE}============================================================${NC}"
    echo ""

    AGENT_ID="orchestrator" python3 "$SCRIPT_DIR/orchestrator.py" "$STORE" "$REPO"
}

run_workers() {
    local count="$1"
    echo ""
    echo -e "${BLUE}============================================================${NC}"
    echo -e "${BLUE}Phase 2: Workers - Implementing tasks (${count} workers)${NC}"
    echo -e "${BLUE}============================================================${NC}"
    echo ""

    # Start workers in background
    local pids=()
    for i in $(seq 1 "$count"); do
        echo -e "${GREEN}Starting worker-$i...${NC}"
        AGENT_ID="worker-$i" python3 "$SCRIPT_DIR/worker.py" "$STORE" "$REPO" &
        pids+=($!)
        sleep 0.5  # Stagger startup slightly
    done

    echo ""
    echo -e "${YELLOW}Workers running. Waiting for completion...${NC}"
    echo ""

    # Wait for all workers
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    echo ""
    echo -e "${GREEN}All workers finished!${NC}"
}

run_reviewer() {
    echo ""
    echo -e "${BLUE}============================================================${NC}"
    echo -e "${BLUE}Phase 3: Reviewer - Checking completed work${NC}"
    echo -e "${BLUE}============================================================${NC}"
    echo ""

    AGENT_ID="reviewer" python3 "$SCRIPT_DIR/reviewer.py" "$STORE" "$REPO"
}

run_documenter() {
    echo ""
    echo -e "${BLUE}============================================================${NC}"
    echo -e "${BLUE}Phase 4: Documenter - Creating documentation${NC}"
    echo -e "${BLUE}============================================================${NC}"
    echo ""

    AGENT_ID="documenter" python3 "$SCRIPT_DIR/documenter.py" "$STORE" "$REPO"
}

show_help() {
    echo "Multi-Agent Collaboration Demo"
    echo ""
    echo "Usage: ./demo.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --watch-only       Just run the watcher to observe an existing session"
    echo "  --workers N        Number of worker agents (default: 2, max: 5)"
    echo "  --help             Show this help"
    echo ""
    echo "Environment Variables:"
    echo "  OPENROUTER_API_KEY   Your OpenRouter API key (required)"
    echo "  SCRAPS_DEMO_STORE    Your store name (auto-detected if not set)"
    echo "  SCRAPS_DEMO_REPO     Repository name (default: multi-agent-demo)"
    echo ""
    echo "The demo will:"
    echo "  1. Read prd.md (Product Requirements Document)"
    echo "  2. Run the Orchestrator to break it into tasks"
    echo "  3. Spawn N workers to implement tasks in parallel"
    echo "  4. Run the Documenter to create documentation"
}

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_help
                exit 0
                ;;
            --watch-only)
                print_banner
                check_cli
                check_auth
                get_store
                echo ""
                echo -e "${CYAN}Watching $STORE/$REPO...${NC}"
                echo -e "Press ${YELLOW}Ctrl+C${NC} to stop"
                echo ""
                scraps watch "$STORE/$REPO"
                exit 0
                ;;
            --workers)
                WORKER_COUNT="$2"
                shift 2
                ;;
            *)
                echo "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # Validate worker count
    if [ "$WORKER_COUNT" -lt 1 ] 2>/dev/null; then WORKER_COUNT=1; fi
    if [ "$WORKER_COUNT" -gt 5 ] 2>/dev/null; then WORKER_COUNT=5; fi

    print_banner

    # Check prerequisites
    echo -e "${BLUE}Checking prerequisites...${NC}"
    check_cli
    check_auth
    check_openrouter
    check_prd
    setup_python_env
    get_store

    echo ""

    # Setup
    setup_repo

    # Ask for worker count if not specified via --workers
    if [ "$WORKER_COUNT" -eq 2 ]; then
        echo ""
        echo -e "${YELLOW}How many worker agents? (1-5, default: 2)${NC}"
        read -p "> " input_count
        if [ -n "$input_count" ]; then
            WORKER_COUNT="$input_count"
            if [ "$WORKER_COUNT" -lt 1 ] 2>/dev/null; then WORKER_COUNT=1; fi
            if [ "$WORKER_COUNT" -gt 5 ] 2>/dev/null; then WORKER_COUNT=5; fi
        fi
    fi

    echo ""
    echo -e "${GREEN}Configuration:${NC}"
    echo "  Store: $STORE"
    echo "  Repo: $REPO"
    echo "  Workers: $WORKER_COUNT"

    # Print watch command for user to run in another terminal
    print_watch_command

    read -p "Press Enter to start the demo... "

    # Run the phases
    run_orchestrator
    run_workers "$WORKER_COUNT"
    run_reviewer
    run_documenter

    echo ""
    echo -e "${GREEN}============================================================${NC}"
    echo -e "${GREEN}Demo complete!${NC}"
    echo -e "${GREEN}============================================================${NC}"
    echo ""
    echo "To see the results:"
    echo "  scraps repo show $STORE/$REPO"
    echo ""
    echo "To clone the repo:"
    echo "  scraps clone $STORE/$REPO"
    echo ""
}

main "$@"
