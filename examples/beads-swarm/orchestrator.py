#!/usr/bin/env python3
"""
Beads Swarm Orchestrator - Creates Beads tasks from any PRD.

Reads any PRD file, analyzes it with an LLM, and creates Beads issues
with dependencies designed for maximum parallelism.

Uses:
- Beads (bd) for task management and dependency graph
- Scraps for file coordination and commits

Usage:
    python orchestrator.py <store> <repo> <prd_file>
    python orchestrator.py alice my-project requirements.md
"""

import os
import sys
import json
import subprocess
import tempfile

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


def run_bd(*args, capture=True, check=True):
    """Run a beads (bd) command."""
    cmd = ["bd"] + list(args)
    if capture:
        result = subprocess.run(cmd, capture_output=True, text=True, check=check)
        return result.stdout.strip()
    else:
        subprocess.run(cmd, check=check)
        return ""


def bd_json(*args):
    """Run bd command with --json flag and parse output."""
    output = run_bd(*args, "--json")
    if output:
        try:
            return json.loads(output)
        except json.JSONDecodeError:
            return None
    return None


if len(sys.argv) < 4:
    print(f"Usage: {sys.argv[0]} <store> <repo> <prd_file>")
    sys.exit(1)

STORE = sys.argv[1]
REPO = sys.argv[2]
PRD_FILE = sys.argv[3]
BRANCH = os.environ.get("BRANCH", "main")
AGENT_ID = os.environ.get("AGENT_ID", f"orchestrator-{os.getpid()}")

# How many tasks to target based on agent count (passed via env)
TARGET_TASKS = int(os.environ.get("TARGET_TASKS", "10"))

# Working directory for beads
WORK_DIR = os.environ.get("BEADS_WORK_DIR", tempfile.mkdtemp(prefix="beads-swarm-"))


TOOLS = [
    {
        "name": "create_task",
        "description": "Create a Beads task. Each task should own exactly ONE file for maximum parallelism.",
        "input_schema": {
            "type": "object",
            "properties": {
                "task_number": {
                    "type": "integer",
                    "description": "Task number (1, 2, 3, etc.)",
                },
                "title": {
                    "type": "string",
                    "description": "Short task title",
                },
                "description": {
                    "type": "string",
                    "description": "Detailed description of what to implement",
                },
                "acceptance_criteria": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of acceptance criteria",
                },
                "owns": {
                    "type": "string",
                    "description": "EXACTLY ONE file path this task owns (e.g., 'src/auth.ts')",
                },
                "priority": {
                    "type": "integer",
                    "description": "Priority 0-4 (0=highest, foundation files; 4=lowest, final integration)",
                    "default": 2,
                },
                "depends_on": {
                    "type": "array",
                    "items": {"type": "integer"},
                    "description": "Task numbers this depends on (e.g., [1, 2]). Empty = can run immediately.",
                    "default": [],
                },
            },
            "required": ["task_number", "title", "description", "acceptance_criteria", "owns"],
        },
    },
    {
        "name": "done",
        "description": "Finish creating tasks.",
        "input_schema": {
            "type": "object",
            "properties": {
                "summary": {
                    "type": "string",
                    "description": "Brief summary of tasks created",
                },
            },
            "required": ["summary"],
        },
    },
]


def init_beads(prd_content: str):
    """Initialize beads database in the work directory."""
    os.chdir(WORK_DIR)

    # Check if already initialized
    if os.path.exists(os.path.join(WORK_DIR, ".beads")):
        print(f"  Beads already initialized in {WORK_DIR}")
        return

    # Initialize git repo first (beads needs git)
    if not os.path.exists(os.path.join(WORK_DIR, ".git")):
        subprocess.run(["git", "init"], capture_output=True, check=True)
        subprocess.run(["git", "config", "user.email", "orchestrator@agent.local"], capture_output=True, check=True)
        subprocess.run(["git", "config", "user.name", "Orchestrator"], capture_output=True, check=True)

    # Initialize beads
    run_bd("init")
    print(f"  Beads initialized in {WORK_DIR}")

    # Create the epic for this PRD
    # Write PRD to a temp file for bd create --body-file
    prd_file = os.path.join(WORK_DIR, "prd.md")
    with open(prd_file, "w") as f:
        f.write(prd_content)


def create_beads_task(inputs: dict, task_ids: dict) -> str:
    """Create a Beads task and return its ID."""
    task_num = inputs["task_number"]
    title = inputs["title"]
    desc = inputs["description"]
    owns = inputs.get("owns", "")
    priority = inputs.get("priority", 2)
    acceptance = inputs.get("acceptance_criteria", [])
    depends_on = inputs.get("depends_on", [])

    # Build description with metadata
    full_desc = f"{desc}\n\n"
    if owns:
        full_desc += f"## Owned Files\n- `{owns}`\n\n"
    if acceptance:
        full_desc += "## Acceptance Criteria\n"
        for criterion in acceptance:
            full_desc += f"- [ ] {criterion}\n"

    # Create the task using bd create
    # Use --silent to get just the ID
    cmd = [
        "bd", "create", title,
        "--type", "task",
        "--priority", str(priority),
        "--description", full_desc,
        "--silent",
    ]

    # Add notes field for owned file (workers will read this)
    if owns:
        cmd.extend(["--notes", f"owns:{owns}"])

    result = subprocess.run(cmd, capture_output=True, text=True)
    task_id = result.stdout.strip()

    if not task_id or result.returncode != 0:
        print(f"  Error creating task: {result.stderr}")
        return ""

    task_ids[task_num] = task_id

    # Add dependencies
    for dep_num in depends_on:
        if dep_num in task_ids:
            dep_id = task_ids[dep_num]
            # task_id depends on dep_id (dep_id blocks task_id)
            run_bd("dep", dep_id, "--blocks", task_id, check=False)

    deps_str = f" (deps: {depends_on})" if depends_on else " (no deps - parallel)"
    print(f"  + {task_id}: {title}")
    print(f"      owns: {owns or '?'}{deps_str}")

    return task_id


def sync_beads_to_scraps(scraps: ScrapsClient):
    """Sync beads database and export to scraps repo."""
    # Flush beads to JSONL
    run_bd("sync", check=False)

    # Collect files to commit
    pending_files = {}

    # Read PRD
    prd_path = os.path.join(WORK_DIR, "prd.md")
    if os.path.exists(prd_path):
        with open(prd_path) as f:
            pending_files["prd.md"] = f.read()

    # Read only JSONL text files from .beads (not the SQLite database)
    beads_dir = os.path.join(WORK_DIR, ".beads")
    if os.path.exists(beads_dir):
        for filename in os.listdir(beads_dir):
            # Only include JSONL files, skip .db files and other binaries
            if not filename.endswith(".jsonl"):
                continue
            filepath = os.path.join(beads_dir, filename)
            if os.path.isfile(filepath):
                try:
                    with open(filepath) as f:
                        pending_files[f".beads/{filename}"] = f.read()
                except UnicodeDecodeError:
                    print(f"  Skipping binary file: {filename}")

    # Export tasks as JSON for workers
    tasks_json = bd_json("list", "--all", "--limit", "0")
    if tasks_json:
        pending_files[".beads/tasks.json"] = json.dumps(tasks_json, indent=2)

    # Commit to scraps
    if pending_files:
        print(f"\nCommitting {len(pending_files)} files to scraps...")
        sha = scraps.commit(f"Add PRD and Beads tasks", pending_files)
        print(f"Committed: {sha[:8]}")
        return sha

    return None


def main():
    print(f"Orchestrator {AGENT_ID} working on {STORE}/{REPO}")
    print(f"PRD: {PRD_FILE}")
    print(f"Target tasks: {TARGET_TASKS}")
    print(f"Work directory: {WORK_DIR}")
    print("-" * 50)

    # Read PRD file
    if not os.path.exists(PRD_FILE):
        print(f"Error: PRD file not found: {PRD_FILE}")
        sys.exit(1)

    with open(PRD_FILE) as f:
        prd_content = f.read()

    print(f"Read PRD ({len(prd_content)} chars)")

    # Initialize beads
    print("\nInitializing Beads...")
    init_beads(prd_content)

    # Initialize scraps client
    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="orchestrator")

    # Track task IDs for dependency resolution
    task_ids = {}

    # System prompt for task creation
    system_prompt = f"""You are an orchestrator that breaks down PRDs into ATOMIC tasks for parallel execution.

CRITICAL: CREATE ATOMIC TASKS
- Each task owns EXACTLY ONE file
- More tasks = more parallelism = faster execution
- Target approximately {TARGET_TASKS} tasks
- Break large features into multiple single-file tasks

DEPENDENCY STRATEGY FOR SPEED:
- Tasks with NO dependencies can run in PARALLEL immediately
- Only add dependencies when a task MUST read another task's file
- Create wide dependency graphs, not long chains
- Aim for maximum tasks runnable in parallel at any time

OPTIMAL STRUCTURE EXAMPLE (10 tasks for an API):

Phase 1 - No deps (run in parallel):
- Task 1: package.json (priority 0)
- Task 2: tsconfig.json (priority 0)
- Task 3: src/types.ts (priority 1)

Phase 2 - Depend on types (run in parallel):
- Task 4: src/store.ts, depends_on: [3]
- Task 5: src/auth.ts, depends_on: [3]
- Task 6: src/validation.ts, depends_on: [3]

Phase 3 - Depend on implementation (run in parallel):
- Task 7: src/handlers.ts, depends_on: [3, 4, 5]
- Task 8: src/middleware.ts, depends_on: [5]

Phase 4 - Final assembly:
- Task 9: src/routes.ts, depends_on: [7, 8]
- Task 10: src/index.ts, depends_on: [9]

This keeps 3+ agents busy at all times!

TASK PRIORITIES (for Beads):
- 0: Config/setup files (package.json, tsconfig.json)
- 1: Core types/interfaces
- 2: Core implementation (stores, auth, utils)
- 3: Features/handlers
- 4: Assembly/main entry point

WORKFLOW:
1. Analyze the PRD
2. Call create_task for EACH task (call it multiple times)
3. Call done when ALL tasks are created"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    print("\nAnalyzing PRD and creating Beads tasks...")

    try:
        _run_orchestrator_loop(agent, scraps, task_ids, prd_content)
    except APICreditsError as e:
        print(f"\n{e}")
        scraps.stream_event("error", error="api_credits_exhausted")
        scraps.stream_event("agent_leave", role="orchestrator")
        sys.exit(1)


def _run_orchestrator_loop(agent, scraps, task_ids, prd_content):
    """Run the orchestrator loop."""
    prompt = f"""Analyze this PRD and create atomic tasks for parallel implementation.

REQUIREMENTS:
- Target approximately {TARGET_TASKS} tasks
- Each task owns EXACTLY ONE file
- Minimize dependencies for maximum parallelism

PRD:
---
{prd_content}
---

INSTRUCTIONS:
1. Call create_task for each task you want to create
2. After creating ALL tasks, call done with a summary"""

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

                if name == "create_task":
                    task_id = create_beads_task(args, task_ids)
                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({"ok": True, "task_id": task_id}),
                    })

                elif name == "done":
                    print(f"\n{args.get('summary', 'Tasks created')}")

                    # Sync beads to scraps
                    sync_beads_to_scraps(scraps)

                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({"ok": True, "finished": True}),
                    })
                    finished = True

        agent.add_assistant_response(response)

        if finished:
            scraps.stream_event("agent_leave", role="orchestrator")
            print("\nOrchestrator done!")
            print(f"\nBeads database: {WORK_DIR}/.beads")
            print(f"View tasks: cd {WORK_DIR} && bd list --pretty")
            return

        if tool_results:
            agent.add_tool_results(tool_results)
            prompt = ""
        elif response.choices[0].finish_reason == "stop":
            break


if __name__ == "__main__":
    main()
