# Beads Swarm - PRD Executor with Beads + Scraps

Execute **any PRD** with N parallel AI agents using:
- **Beads** (`bd`) for task management, dependencies, and agent state
- **Scraps** for file coordination, claims, and atomic commits

## Why Beads + Scraps?

| Layer | Tool | Purpose |
|-------|------|---------|
| **Brain** | Beads | Task graph, dependencies, memory, audit trail |
| **Hands** | Scraps | File locks, atomic commits, live streaming |

Beads drives *what to do*. Scraps coordinates *where/how code changes happen*.

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
│  1. bd init - Initialize Beads database                     │
│  2. Analyze PRD with LLM                                    │
│  3. bd create - Create tasks as Beads issues                │
│  4. bd dep add - Set up dependency graph                    │
│  5. Upload to scraps.sh repo                                │
└─────────────────────────────────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│   WORKER 1    │ │   WORKER 2    │ │   WORKER N    │
│               │ │               │ │               │
│ • bd list     │ │ • bd list     │ │ • bd list     │
│   --ready     │ │   --ready     │ │   --ready     │
│ • bd update   │ │ • bd update   │ │ • bd update   │
│   --claim     │ │   --claim     │ │   --claim     │
│ • scraps claim│ │ • scraps claim│ │ • scraps claim│
│ • Implement   │ │ • Implement   │ │ • Implement   │
│ • scraps      │ │ • scraps      │ │ • scraps      │
│   commit      │ │   commit      │ │   commit      │
│ • bd close    │ │ • bd close    │ │ • bd close    │
└───────────────┘ └───────────────┘ └───────────────┘
        │                 │                 │
        └─────────────────┼─────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   COMPLETED PROJECT                          │
│  • All source files committed to scraps.sh repo             │
│  • Full audit trail in .beads/ (syncs with git)             │
│  • Dependency graph preserved for analysis                   │
└─────────────────────────────────────────────────────────────┘
```

## Benchmark Results

Tested with the sample PRD (Task Management CLI) on 2026-02-02:

| Agents | Orchestrator | Workers | **Total** | Tasks Done | Throughput |
|--------|-------------|---------|-----------|------------|------------|
| **5**  | 53s         | 244s    | **297s**  | 9          | 1.8/min    |
| **10** | 82s         | 206s    | **289s**  | 19         | 3.9/min    |
| **25** | 167s        | 296s    | **464s**  | 42         | 5.4/min    |

### Key Findings

**Throughput scales with agents:**
- 5 → 10 agents: **2.2x faster** (near-linear scaling)
- 10 → 25 agents: **1.4x faster** (diminishing returns)

**Why diminishing returns at 25 agents?**
1. **Orchestrator overhead** - Creating 30 tasks with complex dependencies takes longer (167s vs 53s)
2. **Coordination overhead** - More agents competing for file claims
3. **Dependency bottlenecks** - Some tasks must wait for others to complete

**Sweet spot:** ~10 agents for this PRD size. For larger PRDs with more independent tasks, 25+ agents would scale better.

### Running the Benchmark

```bash
# Run with default agent counts (5, 10, 25)
./benchmark.sh

# Custom agent counts
./benchmark.sh --agents "5 10 20 40"

# Custom PRD
./benchmark.sh --prd my-project.md --agents "10 25"
```

Results are saved to `benchmark-results/` as JSON files.

## How Scaling Works

Speed scales well because:
- Beads' dependency graph enables maximum parallelism
- Scraps' file claims prevent conflicts
- Workers grab unblocked tasks immediately
- More agents = orchestrator creates more granular tasks

## Key Integration Points

### 1. Task Discovery (via tasks.json)
Workers read tasks from `.beads/tasks.json` exported by the orchestrator:
```python
# Get tasks from scraps repo
tasks = get_tasks_from_scraps(scraps)

# Check which files still need work
for task in tasks:
    if not file_exists(task.owns):
        ready_tasks.append(task)
```

### 2. File Coordination (Scraps Claims)
Scraps provides atomic file claiming to prevent conflicts:
```python
# Try to claim exclusive access to a file
if scraps.claim([task.owns], reason="Implementing task"):
    # We own this file - safe to implement
    implement_task(task)
    scraps.commit(files)
    scraps.release([task.owns])
else:
    # Someone else is working on it
    try_next_task()
```

### 3. Completion Detection
Workers detect completion by checking if files exist:
```python
# File exists = task completed by someone
existing = scraps.read_file(task.owns)
if existing:
    completed_files.add(task.owns)
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENROUTER_API_KEY` | Yes | OpenRouter API key |
| `SCRAPS_API_KEY` | Yes | Scraps API key |
| `OPENROUTER_MODEL` | No | Model (default: `anthropic/claude-3.5-haiku`) |
| `BEADS_DIR` | No | Custom .beads location |

## Files

- `run.sh` - Main entry point for running the swarm
- `benchmark.sh` - Performance benchmark script
- `orchestrator.py` - Analyzes PRD, creates Beads tasks
- `worker.py` - Claims tasks via Beads, implements via Scraps
- `sample-prd.md` - Example PRD for testing
- `benchmark-results/` - Benchmark output (JSON + logs)

## Requirements

- Python 3.10+
- Beads CLI (`go install github.com/steveyegge/beads/cmd/bd@latest`)
- Scraps CLI (`curl -fsSL https://scraps.sh/install.sh | sh`)
- OpenRouter account
- Scraps account

## Advanced: Swarm Molecules

For complex epics, use Beads' swarm feature:

```bash
# Create a swarm molecule for an epic
bd swarm create bd-epic-123

# Monitor swarm status
bd swarm status
```

Swarms provide:
- Structured coordination for large epics
- Agent state tracking (`bd agent state`)
- Fanout/fanin patterns for parallel work
