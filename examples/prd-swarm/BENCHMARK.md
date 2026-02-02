# PRD Swarm Benchmark: Atomic vs Beads

This benchmark compares two approaches for orchestrating multiple AI agents to implement a complex PRD (Product Requirements Document).

## Approaches

### Atomic (One File Per Task)
- Each task owns exactly one file
- Tasks have explicit dependencies (e.g., task 3 depends on task 2)
- Maximum theoretical parallelism, but dependencies create bottlenecks
- Workers are "context-blind" - they only see their single task

### Beads (Phase-Based)
- Tasks grouped into sequential phases (e.g., foundation → storage → streaming → query → integration)
- Multiple files per task within a phase
- All tasks within a phase run in parallel
- Phase N+1 waits for all of phase N to complete
- Workers have context from previous phases

## Test PRD: StreamDB

A complex Rust streaming timeseries database with:
- Arrow columnar storage
- Parquet chunk management
- Write-ahead log (WAL) for durability
- SQL query parsing and execution
- HTTP API and CLI

See `benchmark-prd.md` for full specification.

## Running the Benchmark

```bash
# Prerequisites
export OPENROUTER_API_KEY="your-key"

# Run both approaches
./benchmark.sh --prd benchmark-prd.md --agents 5

# Run only one approach
./benchmark.sh --beads-only --agents 5
./benchmark.sh --atomic-only --agents 5

# Multiple runs for statistical significance
./benchmark.sh --agents 5 --runs 3
```

## Results

### Configuration
- PRD: StreamDB (Rust timeseries database)
- Agents: 5 parallel workers
- Timeout: ~5 minutes per approach

### Outcome

```
┌─────────────────┬────────────┬────────────┐
│ Metric          │   ATOMIC   │   BEADS    │
├─────────────────┼────────────┼────────────┤
│ Total Time      │      418s  │      642s  │
│ Tasks Completed │        2/10│      10/25 │
│ Completion Rate │        20% │        40% │
│ Tasks/Second    │     0.0048 │     0.0156 │
└─────────────────┴────────────┴────────────┘

Winner: BEADS (3.25x higher throughput)
```

### Analysis

**Why Atomic Struggled:**
1. The LLM orchestrator created a sequential dependency chain:
   - Task 1 (setup) → Task 2 (types) → Tasks 3-8 (parallel) → Task 9 → Task 10
2. All 5 workers waited 200+ seconds for Task 1 to complete
3. Only 2 tasks finished before timeout

**Why Beads Won:**
1. Phase structure guarantees 5 parallel tasks per phase
2. No inter-task dependencies within a phase
3. Completed 2 full phases (10 tasks) vs only 2 atomic tasks
4. Workers had richer context from previous phases

### Extended Run (Beads Only)

With longer timeout (no time limit):
- **Total time:** 1079s (~18 minutes)
- **Tasks completed:** 27/25 (all tasks + extras)
- **Concurrency errors:** 7

All 5 phases completed successfully:
1. Foundation (types, errors, config)
2. Storage Layer (Arrow, Parquet, manifest)
3. Streaming Layer (WAL, buffer, compactor)
4. Query Layer (parser, planner, executor)
5. Integration (HTTP, CLI, tests)

## Key Insights

### When to Use Atomic
- Simple PRDs with few dependencies
- Tasks that are truly independent
- When maximum flexibility is needed

### When to Use Beads
- Complex PRDs with natural phases
- When tasks build on previous work
- When you want predictable progress
- When workers benefit from shared context

### The Dependency Problem

Atomic's weakness is that LLM orchestrators tend to create dependency chains even when not strictly necessary. For a complex PRD like StreamDB:

```
Atomic: 1 → 2 → [3,4,5,6,7,8] → 9 → 10
        ↑ bottleneck

Beads:  Phase 1: [A,B,C,D,E] (parallel)
        Phase 2: [F,G,H,I,J] (parallel)
        ...
```

The beads structure enforces parallelism by design.

## Files

| File | Description |
|------|-------------|
| `benchmark.sh` | Main benchmark harness |
| `benchmark-prd.md` | Complex test PRD (StreamDB) |
| `orchestrator.py` | Atomic task orchestrator |
| `orchestrator_beads.py` | Beads phase orchestrator |
| `worker.py` | Atomic task worker |
| `worker_beads.py` | Beads phase worker |
| `benchmark-results/` | JSON results and logs |

## Technical Notes

### URL Encoding Fix
The beads worker required URL encoding for nested task paths:
```python
from urllib.parse import quote
encoded_path = quote(path, safe='')  # tasks/001-foundation → tasks%2F001-foundation
```

### Grep Count Fix
Fixed bash grep counting that caused JSON corruption:
```bash
# Before (broken - captures "0\n0" when no matches)
errors=$(grep -c "pattern" file || echo 0)

# After (correct)
errors=$(grep -c "pattern" file || true)
errors=${errors:-0}
```

## Conclusion

For complex, multi-phase projects, the **beads approach delivers significantly better throughput** by structuring work into parallel phases rather than relying on LLM-generated dependency graphs.

The atomic approach may still be preferable for simpler projects or when tasks are genuinely independent, but for real-world PRDs with natural build phases, beads provides more predictable and efficient execution.
