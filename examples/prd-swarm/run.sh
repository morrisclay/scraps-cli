#!/bin/bash
# PRD Swarm - Execute any PRD with N parallel agents
#
# Usage:
#   ./run.sh --prd my-project.md --agents 10
#   ./run.sh --agents 5                         # Uses sample-prd.md
#
# Environment:
#   OPENROUTER_API_KEY - Required (get at openrouter.ai/keys)
#   SCRAPS_API_KEY     - Required (get at scraps.sh/settings)
#   OPENROUTER_MODEL   - Optional (default: anthropic/claude-3.5-haiku)

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
PRD_FILE="$SCRIPT_DIR/sample-prd.md"
STORE="${SCRAPS_DEMO_STORE:-}"

cleanup() {
    echo ""
    echo -e "${YELLOW}Stopping agents...${NC}"
    jobs -p | xargs -r kill 2>/dev/null || true
    wait 2>/dev/null || true
    echo -e "${GREEN}Done!${NC}"
}

trap cleanup EXIT

print_banner() {
    echo -e "${BOLD}${CYAN}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                                                            ║"
    echo "║   🚀 PRD SWARM - Universal Multi-Agent Executor 🚀        ║"
    echo "║                                                            ║"
    echo "║   Execute any PRD with N parallel AI agents                ║"
    echo "║                                                            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

show_help() {
    echo "PRD Swarm - Execute any PRD with parallel AI agents"
    echo ""
    echo "Usage: ./run.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --prd FILE      PRD file to execute (default: sample-prd.md)"
    echo "  --agents N      Number of parallel agents (default: 5)"
    echo "  --store NAME    Scraps store name (default: auto-detect)"
    echo "  --help          Show this help"
    echo ""
    echo "Environment Variables:"
    echo "  OPENROUTER_API_KEY   Your OpenRouter API key (required)"
    echo "  SCRAPS_API_KEY       Your Scraps API key (required)"
    echo "  OPENROUTER_MODEL     Model to use (default: anthropic/claude-3.5-haiku)"
    echo ""
    echo "Examples:"
    echo "  ./run.sh --agents 10                    # Sample PRD with 10 agents"
    echo "  ./run.sh --prd my-api.md --agents 20    # Custom PRD with 20 agents"
    echo ""
    echo "Models (set OPENROUTER_MODEL):"
    echo "  anthropic/claude-3.5-haiku   - Reliable, good balance"
    echo "  google/gemini-2.0-flash-001  - Fast & cheap"
    echo "  deepseek/deepseek-chat       - Cheapest"
    echo "  anthropic/claude-sonnet-4    - Best quality"
}

check_prereqs() {
    echo -e "${BLUE}Checking prerequisites...${NC}"

    # Check for scraps CLI or API key
    if [ -z "$SCRAPS_API_KEY" ]; then
        if command -v scraps &> /dev/null; then
            if scraps whoami &> /dev/null; then
                echo -e "${GREEN}✓ Authenticated via scraps CLI${NC}"
            else
                echo -e "${RED}Error: Run 'scraps login' or set SCRAPS_API_KEY${NC}"
                exit 1
            fi
        else
            echo -e "${RED}Error: SCRAPS_API_KEY not set${NC}"
            echo "Get one at: https://scraps.sh/settings"
            echo "Or install CLI: curl -fsSL https://scraps.sh/install.sh | sh"
            exit 1
        fi
    else
        echo -e "${GREEN}✓ SCRAPS_API_KEY configured${NC}"
    fi

    # Check OpenRouter
    if [ -z "$OPENROUTER_API_KEY" ]; then
        echo -e "${RED}Error: OPENROUTER_API_KEY not set${NC}"
        echo "Get one at: https://openrouter.ai/keys"
        exit 1
    fi
    echo -e "${GREEN}✓ OPENROUTER_API_KEY configured${NC}"

    # Check PRD file
    if [ ! -f "$PRD_FILE" ]; then
        echo -e "${RED}Error: PRD file not found: $PRD_FILE${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ PRD file: $PRD_FILE${NC}"

    # Setup Python
    PARENT_VENV="$SCRIPT_DIR/../streaming-agent/.venv"
    LOCAL_VENV="$SCRIPT_DIR/.venv"

    if [ -d "$PARENT_VENV" ]; then
        source "$PARENT_VENV/bin/activate"
        echo -e "${GREEN}✓ Using shared Python environment${NC}"
    elif [ -d "$LOCAL_VENV" ]; then
        source "$LOCAL_VENV/bin/activate"
        echo -e "${GREEN}✓ Python environment ready${NC}"
    else
        echo -e "${BLUE}Setting up Python environment...${NC}"
        python3 -m venv "$LOCAL_VENV"
        source "$LOCAL_VENV/bin/activate"
        pip install -q openai httpx
        echo -e "${GREEN}✓ Python environment created${NC}"
    fi

    # Verify dependencies
    if ! python3 -c "import openai, httpx" 2>/dev/null; then
        pip install -q openai httpx
    fi
}

get_store() {
    if [ -z "$STORE" ]; then
        if command -v scraps &> /dev/null; then
            STORE=$(scraps whoami -o json 2>/dev/null | python3 -c "import sys, json; print(json.load(sys.stdin).get('username', ''))" 2>/dev/null || echo "")
        fi

        if [ -z "$STORE" ]; then
            echo -e "${YELLOW}Could not auto-detect store name.${NC}"
            read -p "Enter your store/username: " STORE
        fi
    fi
    echo -e "${GREEN}✓ Store: $STORE${NC}"
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
                # Make absolute if relative
                if [[ ! "$PRD_FILE" = /* ]]; then
                    PRD_FILE="$(pwd)/$PRD_FILE"
                fi
                shift 2
                ;;
            --agents)
                AGENT_COUNT="$2"
                shift 2
                ;;
            --store)
                STORE="$2"
                shift 2
                ;;
            *)
                echo "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # Validate agent count
    if [ "$AGENT_COUNT" -lt 1 ] 2>/dev/null; then AGENT_COUNT=1; fi
    if [ "$AGENT_COUNT" -gt 50 ] 2>/dev/null; then AGENT_COUNT=50; fi

    print_banner
    check_prereqs
    get_store

    # Create unique repo
    REPO="prd-$(date +%H%M%S)"
    echo ""
    echo -e "${BLUE}Creating repository: ${BOLD}$STORE/$REPO${NC}"

    if command -v scraps &> /dev/null; then
        scraps repo create "$STORE/$REPO"
    else
        # Use API directly if CLI not available
        curl -s -X POST "https://api.scraps.sh/api/v1/stores/$STORE/repos" \
            -H "Authorization: Bearer $SCRAPS_API_KEY" \
            -H "Content-Type: application/json" \
            -d "{\"name\": \"$REPO\"}" > /dev/null
    fi

    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}Watch progress in another terminal:${NC}"
    echo ""
    echo -e "  ${BOLD}${YELLOW}scraps watch $STORE/$REPO${NC}"
    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Calculate target tasks based on agent count
    # More agents = more tasks for better parallelism
    TARGET_TASKS=$((AGENT_COUNT + AGENT_COUNT / 2))
    if [ "$TARGET_TASKS" -lt 6 ]; then TARGET_TASKS=6; fi
    if [ "$TARGET_TASKS" -gt 30 ]; then TARGET_TASKS=30; fi

    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}Phase 1: Orchestrator - Analyzing PRD${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "  PRD: $(basename "$PRD_FILE")"
    echo "  Target tasks: $TARGET_TASKS"
    echo ""

    TARGET_TASKS="$TARGET_TASKS" AGENT_ID="orchestrator" \
        python3 "$SCRIPT_DIR/orchestrator.py" "$STORE" "$REPO" "$PRD_FILE"

    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}Phase 2: Workers - Implementing in parallel ($AGENT_COUNT agents)${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Launch all workers simultaneously
    pids=()
    start_time=$(date +%s)

    for i in $(seq 1 "$AGENT_COUNT"); do
        AGENT_ID="agent-$i" python3 "$SCRIPT_DIR/worker.py" "$STORE" "$REPO" &
        pids+=($!)
        echo -e "${GREEN}  Started agent-$i${NC}"
    done

    echo ""
    echo -e "${BOLD}${YELLOW}$AGENT_COUNT agents now working in parallel!${NC}"
    echo ""

    # Wait for all workers
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    end_time=$(date +%s)
    duration=$((end_time - start_time))

    echo ""
    echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${GREEN}  COMPLETE! $AGENT_COUNT agents finished in ${duration}s${NC}"
    echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "View results:  ${CYAN}scraps repo show $STORE/$REPO${NC}"
    echo -e "Clone repo:    ${CYAN}scraps clone $STORE/$REPO${NC}"
    echo -e "View commits:  ${CYAN}scraps log $STORE/$REPO${NC}"
    echo ""
}

main "$@"
