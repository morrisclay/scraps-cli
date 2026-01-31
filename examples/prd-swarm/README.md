# PRD Swarm - Universal Multi-Agent PRD Executor

Execute **any PRD** with N parallel AI agents using scraps.sh coordination.

## Features

- **Universal**: Works with any PRD file - just point it at your requirements
- **Parallel**: Scales speed linearly with more agents (10 agents = ~10x faster)
- **Coordinated**: Uses scraps.sh file claiming to prevent conflicts
- **Atomic**: Each task owns one file for true parallelism
- **Smart Dependencies**: Orchestrator creates optimal dependency graph

## Quick Start

```bash
# Set your keys
export OPENROUTER_API_KEY="sk-or-..."  # Get at: openrouter.ai/keys
export SCRAPS_API_KEY="scraps_..."     # Get at: scraps.sh/settings

# Run with 10 agents on your PRD
./run.sh --prd my-project.md --agents 10

# Or use the sample PRD
./run.sh --agents 5
```

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                     YOUR PRD FILE                           │
│  "Build a REST API with auth, users, and posts..."         │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                    ORCHESTRATOR                              │
│  • Analyzes PRD                                             │
│  • Creates N atomic tasks (one file per task)               │
│  • Sets up dependency graph for max parallelism             │
│  • Uploads to scraps.sh repo                                │
└─────────────────────────────────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│   WORKER 1    │ │   WORKER 2    │ │   WORKER N    │
│               │ │               │ │               │
│ • Poll tasks  │ │ • Poll tasks  │ │ • Poll tasks  │
│ • Claim file  │ │ • Claim file  │ │ • Claim file  │
│ • Implement   │ │ • Implement   │ │ • Implement   │
│ • Commit      │ │ • Commit      │ │ • Commit      │
└───────────────┘ └───────────────┘ └───────────────┘
        │                 │                 │
        └─────────────────┼─────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   COMPLETED PROJECT                          │
│  All source files committed to scraps.sh repo               │
└─────────────────────────────────────────────────────────────┘
```

## Performance

| Agents | Tasks | Typical Time | Speedup |
|--------|-------|--------------|---------|
| 1      | 10    | ~5 min       | 1x      |
| 5      | 10    | ~1 min       | 5x      |
| 10     | 10    | ~30 sec      | 10x     |
| 20     | 20    | ~30 sec      | 20x     |

## Usage Examples

```bash
# Simple: 5 agents with sample PRD
./run.sh --agents 5

# Custom PRD with 10 agents
./run.sh --prd ~/projects/my-app/requirements.md --agents 10

# Complex project with 20 agents
./run.sh --prd enterprise-api.md --agents 20

# Watch progress in another terminal
scraps watch <your-store>/<repo>
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENROUTER_API_KEY` | Yes | OpenRouter API key |
| `SCRAPS_API_KEY` | Yes | Scraps API key |
| `OPENROUTER_MODEL` | No | Model (default: `anthropic/claude-3.5-haiku`) |

## Files

- `run.sh` - Main entry point
- `orchestrator.py` - Breaks PRD into atomic tasks
- `worker.py` - Claims and implements tasks
- `agent_base.py` - Shared utilities (imported from parent)
- `sample-prd.md` - Example PRD for testing

## Requirements

- Python 3.10+
- scraps CLI (`curl -fsSL https://scraps.sh/install.sh | sh`)
- OpenRouter account
- Scraps account
