#!/usr/bin/env python3
"""
Beads Orchestrator - Phase-based task creation for complex PRDs.

Instead of one file per task (atomic), creates phases ("beads") where:
- Each phase contains multiple related files
- Phases execute sequentially (sync points)
- Tasks within a phase can run in parallel
- Each phase has full context from previous phases

Usage:
    python orchestrator_beads.py <store> <repo> <prd_file>
"""

import os
import sys
import json

# Add streaming-agent directory to path for shared utilities
PARENT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STREAMING_AGENT_DIR = os.path.join(PARENT_DIR, "streaming-agent")
sys.path.insert(0, STREAMING_AGENT_DIR)
from agent_base import ScrapsClient, ClaudeAgent

try:
    import openai
except ImportError:
    print("Error: openai package not installed")
    print("Run: pip install openai httpx")
    sys.exit(1)


class APICreditsError(Exception):
    """Raised when API credits are exhausted."""
    pass


def check_api_error(e: Exception):
    """Check if error is due to credit/billing issues."""
    error_msg = str(e).lower()
    if any(x in error_msg for x in ["credit", "billing", "quota", "insufficient"]):
        raise APICreditsError("API credits exhausted. Please add credits to your OpenRouter account.")
    raise e


if len(sys.argv) < 4:
    print(f"Usage: {sys.argv[0]} <store> <repo> <prd_file>")
    sys.exit(1)

STORE = sys.argv[1]
REPO = sys.argv[2]
PRD_FILE = sys.argv[3]
BRANCH = os.environ.get("BRANCH", "main")
AGENT_ID = os.environ.get("AGENT_ID", f"orchestrator-{os.getpid()}")

# Number of phases to target
TARGET_PHASES = int(os.environ.get("TARGET_PHASES", "5"))
# Tasks per phase (for parallelism within phase)
TASKS_PER_PHASE = int(os.environ.get("TASKS_PER_PHASE", "4"))


TOOLS = [
    {
        "name": "create_phase",
        "description": """Create a phase (bead) containing multiple related tasks that can run in parallel.

A phase represents a coherent unit of work where:
- All tasks in the phase share context from previous phases
- Tasks within the phase can execute in parallel
- The phase must complete before the next phase starts

Example phases for a database:
- Phase 1: Foundation (types, traits, errors)
- Phase 2: Storage (codec, chunks, manifest)
- Phase 3: Streaming (WAL, buffer, flusher)
- Phase 4: Query (parser, planner, executor)
- Phase 5: Integration (server, CLI)""",
        "input_schema": {
            "type": "object",
            "properties": {
                "phase_number": {
                    "type": "integer",
                    "description": "Phase number (1, 2, 3, etc.)",
                },
                "phase_name": {
                    "type": "string",
                    "description": "Short name for the phase (e.g., 'foundation', 'storage', 'query')",
                },
                "phase_description": {
                    "type": "string",
                    "description": "What this phase accomplishes and why it's needed before later phases",
                },
                "tasks": {
                    "type": "array",
                    "description": "List of tasks within this phase (can run in parallel)",
                    "items": {
                        "type": "object",
                        "properties": {
                            "task_id": {
                                "type": "string",
                                "description": "Unique task ID within phase (e.g., 'types', 'traits')",
                            },
                            "title": {
                                "type": "string",
                                "description": "Task title",
                            },
                            "files": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": "Files this task creates/owns (can be multiple related files)",
                            },
                            "description": {
                                "type": "string",
                                "description": "Detailed implementation description",
                            },
                            "acceptance_criteria": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": "Acceptance criteria for this task",
                            },
                        },
                        "required": ["task_id", "title", "files", "description", "acceptance_criteria"],
                    },
                },
            },
            "required": ["phase_number", "phase_name", "phase_description", "tasks"],
        },
    },
    {
        "name": "done",
        "description": "Finish creating phases and commit all files to the repo.",
        "input_schema": {
            "type": "object",
            "properties": {
                "summary": {
                    "type": "string",
                    "description": "Brief summary of phases created",
                },
                "total_phases": {
                    "type": "integer",
                    "description": "Total number of phases",
                },
                "total_tasks": {
                    "type": "integer",
                    "description": "Total number of tasks across all phases",
                },
            },
            "required": ["summary", "total_phases", "total_tasks"],
        },
    },
]


def create_phase_content(phase: dict, task: dict, all_phases: list) -> str:
    """Generate markdown content for a task file within a phase."""
    phase_num = phase["phase_number"]
    phase_name = phase["phase_name"]

    # Build list of dependencies (all previous phases)
    deps = [f"{p['phase_number']:03d}" for p in all_phases if p["phase_number"] < phase_num]
    deps_str = ", ".join(deps) if deps else ""

    # Files owned by this task
    files = task.get("files", [])
    files_str = ", ".join(files) if files else ""
    files_lines = "\n".join(f"- `{f}`" for f in files) if files else "- (none)"

    criteria_lines = "\n".join(f"- [ ] {c}" for c in task.get("acceptance_criteria", []))

    # Context from previous phases
    prev_phases_context = ""
    if deps:
        prev_phases_context = "\n\n## Context from Previous Phases\n"
        for p in all_phases:
            if p["phase_number"] < phase_num:
                prev_phases_context += f"\n### Phase {p['phase_number']}: {p['phase_name']}\n"
                prev_phases_context += f"{p['phase_description']}\n"
                prev_phases_context += "Files created:\n"
                for t in p.get("tasks", []):
                    for f in t.get("files", []):
                        prev_phases_context += f"- `{f}` - {t['title']}\n"

    return f"""---
status: pending
claimed_by: null
phase: {phase_num}
phase_name: {phase_name}
depends_on: [{deps_str}]
owns: [{files_str}]
---
# Phase {phase_num} ({phase_name}): {task['title']}

## Phase Context
{phase['phase_description']}

## Task Description
{task['description']}

## Files to Create
{files_lines}

## Acceptance Criteria
{criteria_lines}
{prev_phases_context}
"""


def main():
    print(f"Beads Orchestrator {AGENT_ID} working on {STORE}/{REPO}")
    print(f"PRD: {PRD_FILE}")
    print(f"Target phases: {TARGET_PHASES}, Tasks per phase: {TASKS_PER_PHASE}")
    print("-" * 50)

    # Read PRD file
    if not os.path.exists(PRD_FILE):
        print(f"Error: PRD file not found: {PRD_FILE}")
        sys.exit(1)

    with open(PRD_FILE) as f:
        prd_content = f.read()

    print(f"Read PRD ({len(prd_content)} chars)")

    # Initialize clients
    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="orchestrator-beads")

    pending_files: dict[str, str] = {}
    pending_files["prd.md"] = prd_content

    # Track all phases for context building
    all_phases: list[dict] = []

    # System prompt for beads-based task creation
    system_prompt = f"""You are an orchestrator that breaks down complex PRDs into PHASES (beads) for multi-agent execution.

BEADS PATTERN:
- A "bead" is a phase containing multiple related tasks
- Phases execute SEQUENTIALLY (phase 2 waits for phase 1)
- Tasks WITHIN a phase execute in PARALLEL
- Each phase provides context for subsequent phases

TARGET: {TARGET_PHASES} phases, {TASKS_PER_PHASE} tasks per phase

PHASE STRUCTURE FOR A DATABASE PROJECT:

Phase 1 - Foundation (parallel tasks):
  - types: Core type definitions (Point, Timestamp, FieldValue)
  - errors: Error types and Result aliases
  - traits: Core traits (Storage, Stream, Query)
  - config: Configuration structures

Phase 2 - Storage Layer (depends on Phase 1):
  - arrow_codec: Point <-> RecordBatch conversion
  - chunk: Parquet read/write
  - manifest: Chunk metadata index
  - backend: Filesystem abstraction

Phase 3 - Streaming Layer (depends on Phase 2):
  - wal: Write-ahead log implementation
  - buffer: In-memory buffer manager
  - flusher: Buffer to chunk conversion
  - compactor: Chunk merging

Phase 4 - Query Layer (depends on Phase 2):
  - parser: SQL parsing
  - planner: Query planning
  - optimizer: Plan optimization
  - executor: Query execution

Phase 5 - Integration (depends on Phase 3, 4):
  - server: HTTP API
  - cli: Command-line interface
  - client: Client library
  - main: Entry point

BENEFITS OF BEADS:
1. Workers have FULL context from previous phases
2. Related code is designed together
3. Integration happens at phase boundaries
4. Fewer but larger tasks = less coordination overhead

RULES:
1. Each task can own MULTIPLE related files
2. Tasks in same phase have NO dependencies on each other
3. All tasks in phase N depend on ALL of phase N-1
4. Include clear descriptions for what each file should contain

WORKFLOW:
1. Call create_phase for each phase
2. Call done when all phases are created"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    print("\nAnalyzing PRD and creating phases...")

    try:
        _run_orchestrator_loop(agent, scraps, pending_files, prd_content, all_phases)
    except APICreditsError as e:
        print(f"\n{e}")
        scraps.stream_event("error", error="api_credits_exhausted")
        scraps.stream_event("agent_leave", role="orchestrator-beads")
        sys.exit(1)


def _run_orchestrator_loop(agent, scraps, pending_files, prd_content, all_phases):
    """Run the orchestrator loop."""
    prompt = f"""Analyze this PRD and create phases (beads) for parallel implementation.

REQUIREMENTS:
- Create {TARGET_PHASES} phases with {TASKS_PER_PHASE} tasks each
- Each phase should be a coherent unit of work
- Tasks within a phase can run in parallel
- Later phases build on earlier phases

PRD:
---
{prd_content}
---

INSTRUCTIONS:
1. Call create_phase for each phase (with its tasks)
2. Call done when all phases are created
3. Do NOT ask questions - create the phases and call done"""

    total_tasks = 0

    while True:
        try:
            response = agent.send(prompt)
        except openai.BadRequestError as e:
            check_api_error(e)
        except openai.APIError as e:
            check_api_error(e)

        message = response.choices[0].message
        tool_results = []
        finished = False

        if message.content:
            print(message.content)

        if message.tool_calls:
            for tool_call in message.tool_calls:
                name = tool_call.function.name
                try:
                    args = json.loads(tool_call.function.arguments)
                except json.JSONDecodeError:
                    args = {}

                if name == "create_phase":
                    phase_num = args["phase_number"]
                    phase_name = args["phase_name"]
                    tasks = args.get("tasks", [])

                    # Store phase for context building
                    all_phases.append(args)

                    print(f"\n  Phase {phase_num}: {phase_name}")
                    print(f"    {args['phase_description'][:80]}...")

                    # Create task files for each task in the phase
                    for task in tasks:
                        task_id = task["task_id"]
                        filename = f"tasks/{phase_num:03d}-{phase_name}/{task_id}.md"

                        content = create_phase_content(args, task, all_phases)
                        pending_files[filename] = content

                        files = task.get("files", [])
                        print(f"      + {task_id}: {task['title']}")
                        print(f"        files: {files}")
                        total_tasks += 1

                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({
                            "ok": True,
                            "phase": phase_num,
                            "tasks_created": len(tasks)
                        }),
                    })

                elif name == "done":
                    summary = args.get("summary", "Phases created")
                    print(f"\n{summary}")
                    print(f"Total: {args.get('total_phases', len(all_phases))} phases, {args.get('total_tasks', total_tasks)} tasks")

                    if pending_files:
                        print(f"\nCommitting {len(pending_files)} files...")
                        sha = scraps.commit(
                            f"Add PRD and {len(all_phases)} phases with {total_tasks} tasks (beads pattern)",
                            pending_files
                        )
                        print(f"Committed: {sha[:8]}")

                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({"ok": True, "finished": True}),
                    })
                    finished = True

        agent.add_assistant_response(response)

        if finished:
            scraps.stream_event("agent_leave", role="orchestrator-beads")
            print("\nBeads Orchestrator done!")
            return

        if tool_results:
            agent.add_tool_results(tool_results)
            prompt = ""
        elif response.choices[0].finish_reason == "stop":
            break


if __name__ == "__main__":
    main()
