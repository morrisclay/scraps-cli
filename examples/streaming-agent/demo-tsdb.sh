#!/bin/bash
# StreamDB Demo - 20 agents building a timeseries database
#
# Demonstrates parallel development of a real system:
# - Durable streams for ingestion
# - Arrow columnar storage
# - SQL query engine
# - Object storage (R2-compatible)
#
# Usage:
#   ./demo-tsdb.sh              # Run with 20 workers
#   ./demo-tsdb.sh --workers 10 # Custom worker count

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
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                                                                ║"
    echo "║   ⚡ STREAMDB: Timeseries Database Build Demo ⚡               ║"
    echo "║                                                                ║"
    echo "║   20 agents building a real database in Rust:                  ║"
    echo "║   • Durable streams ingestion                                  ║"
    echo "║   • Arrow columnar storage                                     ║"
    echo "║   • SQL query engine                                           ║"
    echo "║   • Object storage backend                                     ║"
    echo "║                                                                ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
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
                echo "Usage: ./demo-tsdb.sh [--workers N]"
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
    REPO="streamdb-$(date +%H%M%S)"
    echo ""
    echo -e "${BLUE}Creating repository: ${BOLD}$STORE/$REPO${NC}"
    scraps repo create "$STORE/$REPO"

    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}Watch the swarm build a database:${NC}"
    echo ""
    echo -e "  ${BOLD}${YELLOW}scraps watch $STORE/$REPO${NC}"
    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Generate foundation + tasks
    echo -e "${BLUE}Initializing StreamDB project with 20 tasks...${NC}"
    python3 "$SCRIPT_DIR/generate_tsdb_tasks.py" "$STORE" "$REPO"

    echo ""
    echo -e "${BOLD}${GREEN}Launching $WORKER_COUNT agents to build StreamDB...${NC}"
    echo ""

    # Show what's being built
    echo -e "${CYAN}Components being built in parallel:${NC}"
    echo -e "  ${BLUE}Stream Layer:${NC}    server, client, protocol"
    echo -e "  ${BLUE}Storage Layer:${NC}   arrow codec, chunks, object store, manifest"
    echo -e "  ${BLUE}Ingestion:${NC}       write buffer, flusher, compactor"
    echo -e "  ${BLUE}Query Engine:${NC}    SQL parser, planner, scan, aggregates, streaming"
    echo -e "  ${BLUE}Integration:${NC}     CLI, database, HTTP API, demo"
    echo ""

    # Launch all workers
    pids=()
    for i in $(seq 1 "$WORKER_COUNT"); do
        AGENT_ID="builder-$i" python3 "$SCRIPT_DIR/worker.py" "$STORE" "$REPO" &
        pids+=($!)
        echo -e "${GREEN}  Started builder-$i${NC}"
    done

    echo ""
    echo -e "${BOLD}${YELLOW}$WORKER_COUNT agents now building StreamDB in parallel!${NC}"
    echo ""

    # Wait for completion
    start_time=$(date +%s)

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    end_time=$(date +%s)
    duration=$((end_time - start_time))

    echo ""
    echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${GREEN}  BUILD COMPLETE! StreamDB built by $WORKER_COUNT agents in ${duration}s${NC}"
    echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "Clone and build:"
    echo -e "  ${CYAN}scraps clone $STORE/$REPO${NC}"
    echo -e "  ${CYAN}cd $REPO && cargo build${NC}"
    echo ""
    echo -e "Run the demo:"
    echo -e "  ${CYAN}cargo run --example demo${NC}"
    echo ""
}

main "$@"
