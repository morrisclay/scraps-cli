#!/usr/bin/env python3
"""
Universal PRD Orchestrator - Creates atomic tasks from any PRD.

Reads any PRD file, analyzes it with an LLM, and creates optimal atomic tasks
with dependencies designed for maximum parallelism.

Usage:
    python orchestrator.py <store> <repo> <prd_file>
    python orchestrator.py alice my-project requirements.md
"""

import os
import sys
import json

# Add streaming-agent directory to path for shared utilities
PARENT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STREAMING_AGENT_DIR = os.path.join(PARENT_DIR, "streaming-agent")
sys.path.insert(0, STREAMING_AGENT_DIR)
from agent_base import ScrapsClient, ClaudeAgent

# Check for openai import separately to give better error
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

# How many tasks to target based on agent count (passed via env)
TARGET_TASKS = int(os.environ.get("TARGET_TASKS", "10"))


TOOLS = [
    {
        "name": "create_task",
        "description": "Create an atomic task file. Each task should own exactly ONE file for maximum parallelism.",
        "input_schema": {
            "type": "object",
            "properties": {
                "task_number": {
                    "type": "integer",
                    "description": "Task number (1, 2, 3, etc.)",
                },
                "slug": {
                    "type": "string",
                    "description": "Short slug for filename (e.g., 'user-auth', 'task-model')",
                },
                "title": {
                    "type": "string",
                    "description": "Full task title",
                },
                "description": {
                    "type": "string",
                    "description": "Detailed description of what to implement, including any interfaces/types needed",
                },
                "acceptance_criteria": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of acceptance criteria for this task",
                },
                "owns": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "EXACTLY ONE file path this task owns (e.g., ['src/auth.ts']). One file = one task.",
                    "maxItems": 1,
                    "minItems": 1,
                },
                "priority": {
                    "type": "integer",
                    "description": "Priority 1-5 (1=highest, foundation files; 5=lowest, final integration)",
                    "default": 3,
                },
                "depends_on": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Task numbers this depends on (e.g., ['001', '002']). Empty = can run immediately.",
                    "default": [],
                },
            },
            "required": ["task_number", "slug", "title", "description", "acceptance_criteria", "owns"],
        },
    },
    {
        "name": "done",
        "description": "Finish creating tasks and commit all files to the repo.",
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


def create_task_content(inputs: dict) -> str:
    """Generate markdown content for a task file."""
    priority = inputs.get("priority", 3)
    depends = inputs.get("depends_on", [])
    depends_str = ", ".join(depends) if depends else ""
    owns = inputs.get("owns", [])
    owns_str = ", ".join(owns) if owns else ""

    criteria_lines = "\n".join(f"- [ ] {c}" for c in inputs["acceptance_criteria"])
    owns_lines = "\n".join(f"- `{f}`" for f in owns) if owns else "- (none specified)"

    return f"""---
status: pending
claimed_by: null
priority: {priority}
depends_on: [{depends_str}]
owns: [{owns_str}]
---
# Task: {inputs['title']}

## Description
{inputs['description']}

## Owned Files
{owns_lines}

## Acceptance Criteria
{criteria_lines}
"""


def main():
    print(f"Orchestrator {AGENT_ID} working on {STORE}/{REPO}")
    print(f"PRD: {PRD_FILE}")
    print(f"Target tasks: {TARGET_TASKS}")
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
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="orchestrator")

    pending_files: dict[str, str] = {}
    pending_files["prd.md"] = prd_content

    # System prompt optimized for atomic task creation
    system_prompt = f"""You are an orchestrator agent that breaks down PRDs into ATOMIC tasks for parallel execution by a swarm of AI agents.

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
- 001-package: owns [package.json]
- 002-tsconfig: owns [tsconfig.json]
- 003-types: owns [src/types.ts]

Phase 2 - Depend on types (run in parallel):
- 004-store: owns [src/store.ts], depends_on: [003]
- 005-auth: owns [src/auth.ts], depends_on: [003]
- 006-validation: owns [src/validation.ts], depends_on: [003]

Phase 3 - Depend on implementation (run in parallel):
- 007-handlers: owns [src/handlers.ts], depends_on: [003, 004, 005]
- 008-middleware: owns [src/middleware.ts], depends_on: [005]

Phase 4 - Final assembly:
- 009-routes: owns [src/routes.ts], depends_on: [007, 008]
- 010-index: owns [src/index.ts], depends_on: [009]

This keeps 3+ agents busy at all times!

TASK PRIORITIES:
- 1: Config/setup files (package.json, tsconfig.json)
- 2: Core types/interfaces
- 3: Core implementation (stores, auth, utils)
- 4: Features/handlers
- 5: Assembly/main entry point

IMPORTANT:
- Include necessary type definitions in each task's description
- Reference existing files workers should import from
- Make tasks self-contained with clear acceptance criteria

WORKFLOW (you MUST follow this):
1. Analyze the PRD
2. Call create_task for EACH task (call it multiple times)
3. Call done when ALL tasks are created - THIS IS REQUIRED

You MUST call done after creating tasks. Do not ask for confirmation."""

    agent = ClaudeAgent(system_prompt, TOOLS)

    print("\nAnalyzing PRD and creating tasks...")

    try:
        _run_orchestrator_loop(agent, scraps, pending_files, prd_content)
    except APICreditsError as e:
        print(f"\n{e}")
        scraps.stream_event("error", error="api_credits_exhausted")
        scraps.stream_event("agent_leave", role="orchestrator")
        sys.exit(1)


def _run_orchestrator_loop(agent, scraps, pending_files, prd_content):
    """Run the orchestrator loop."""
    prompt = f"""Analyze this PRD and create atomic tasks for parallel implementation.

REQUIREMENTS:
- Target approximately {TARGET_TASKS} tasks
- Each task owns EXACTLY ONE file
- Minimize dependencies for maximum parallelism
- For utility libraries: NO dependencies between utils

PRD:
---
{prd_content}
---

INSTRUCTIONS:
1. Call create_task for each task you want to create
2. After creating ALL tasks, call done with a summary
3. Do NOT ask questions - just create the tasks and call done"""

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
                    task_num = args["task_number"]
                    slug = args["slug"]
                    filename = f"tasks/{task_num:03d}-{slug}.md"

                    content = create_task_content(args)
                    pending_files[filename] = content

                    owns = args.get("owns", ["(none)"])
                    deps = args.get("depends_on", [])
                    deps_str = f" (deps: {deps})" if deps else " (no deps - parallel)"
                    print(f"  + {filename}: {args['title']}")
                    print(f"      owns: {owns[0] if owns else '?'}{deps_str}")

                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({"ok": True, "path": filename}),
                    })

                elif name == "done":
                    print(f"\n{args.get('summary', 'Tasks created')}")

                    if pending_files:
                        print(f"\nCommitting {len(pending_files)} files...")
                        task_count = len([f for f in pending_files if f.startswith("tasks/")])
                        sha = scraps.commit(
                            f"Add PRD and {task_count} atomic tasks",
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
            scraps.stream_event("agent_leave", role="orchestrator")
            print("\nOrchestrator done!")
            return

        if tool_results:
            agent.add_tool_results(tool_results)
            prompt = ""
        elif response.choices[0].finish_reason == "stop":
            break


if __name__ == "__main__":
    main()
