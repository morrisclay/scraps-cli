#!/usr/bin/env python3
"""
Orchestrator Agent - Breaks PRD into task files.

Reads a local prd.md file, uploads it to the repo, uses Claude to analyze
and generate a task breakdown, creates task files with YAML frontmatter,
commits everything, and exits.

Usage:
    python orchestrator.py <store> <repo>
    python orchestrator.py alice demo-project
"""

import os
import sys
import json

import openai
from agent_base import ScrapsClient, ClaudeAgent


class APICreditsError(Exception):
    """Raised when API credits are exhausted."""
    pass


def check_api_error(e: Exception):
    """Check if error is due to credit/billing issues and raise appropriate exception."""
    error_msg = str(e).lower()
    if "credit" in error_msg or "billing" in error_msg or "quota" in error_msg or "insufficient" in error_msg:
        raise APICreditsError("API credits exhausted. Please add credits to your OpenRouter account.")
    raise e

if len(sys.argv) < 3:
    print(f"Usage: {sys.argv[0]} <store> <repo>")
    sys.exit(1)

STORE = sys.argv[1]
REPO = sys.argv[2]
BRANCH = os.environ.get("BRANCH", "main")
AGENT_ID = os.environ.get("AGENT_ID", f"orchestrator-{os.getpid()}")
PRD_FILE = os.environ.get("PRD_FILE", "prd.md")

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

# ---------------------------------------------------------------------------
# Tools for Claude
# ---------------------------------------------------------------------------

TOOLS = [
    {
        "name": "create_task",
        "description": "Create a task file in the tasks/ directory. Call this for each task you want to create.",
        "input_schema": {
            "type": "object",
            "properties": {
                "task_number": {
                    "type": "integer",
                    "description": "Task number (001, 002, etc.)",
                },
                "slug": {
                    "type": "string",
                    "description": "Short slug for filename (e.g., 'user-auth', 'task-crud')",
                },
                "title": {
                    "type": "string",
                    "description": "Full task title",
                },
                "description": {
                    "type": "string",
                    "description": "Detailed description of what needs to be implemented",
                },
                "acceptance_criteria": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of acceptance criteria",
                },
                "priority": {
                    "type": "integer",
                    "description": "Priority 1-5 (1 is highest)",
                    "default": 3,
                },
                "depends_on": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of task slugs this depends on",
                    "default": [],
                },
            },
            "required": ["task_number", "slug", "title", "description", "acceptance_criteria"],
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

    criteria_lines = "\n".join(f"- [ ] {c}" for c in inputs["acceptance_criteria"])

    return f"""---
status: pending
claimed_by: null
priority: {priority}
depends_on: [{depends_str}]
---
# Task: {inputs['title']}

## Description
{inputs['description']}

## Acceptance Criteria
{criteria_lines}
"""


def main():
    print(f"Orchestrator {AGENT_ID} working on {STORE}/{REPO}")
    print("-" * 50)

    # Read local PRD file
    prd_path = os.path.join(SCRIPT_DIR, PRD_FILE)
    if not os.path.exists(prd_path):
        print(f"Error: PRD file not found: {prd_path}")
        sys.exit(1)

    with open(prd_path) as f:
        prd_content = f.read()

    print(f"Read PRD from {PRD_FILE} ({len(prd_content)} chars)")

    # Initialize clients
    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="orchestrator")

    # Files to commit
    pending_files: dict[str, str] = {}

    # Add PRD to pending files
    pending_files["prd.md"] = prd_content

    # Set up Claude agent
    system_prompt = """You are an orchestrator agent that breaks down a Product Requirements Document (PRD) into discrete, implementable tasks.

Your job is to:
1. Analyze the PRD carefully
2. Break it down into 3-6 well-defined tasks
3. Create a task file for each using the create_task tool
4. Consider dependencies between tasks
5. Call done when finished

Each task should be:
- Self-contained and implementable by a single agent
- Clearly defined with specific acceptance criteria
- Ordered by dependencies (later tasks can depend on earlier ones)

Task priorities:
- 1: Critical/blocking
- 2: High priority
- 3: Normal
- 4: Low priority
- 5: Nice to have"""

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
    """Inner loop for orchestrator."""
    prompt = f"""Please analyze this Product Requirements Document and break it into implementable tasks.

Create 3-6 tasks that cover all the requirements. Consider what order they should be implemented in
and set appropriate dependencies.

---
{prd_content}
---

Use create_task for each task, then call done when finished."""

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

        # Print any text content
        if message.content:
            print(message.content)

        # Process tool calls
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

                    print(f"  + Created {filename}: {args['title']}")

                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({"ok": True, "path": filename}),
                    })

                elif name == "done":
                    print(f"\n{args.get('summary', 'Tasks created')}")

                    # Commit all files
                    if pending_files:
                        print(f"\nCommitting {len(pending_files)} files...")
                        task_count = len([f for f in pending_files if f.startswith("tasks/")])
                        sha = scraps.commit(
                            f"Add PRD and {task_count} tasks for implementation",
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
            prompt = ""  # Clear prompt for subsequent iterations
        elif response.choices[0].finish_reason == "stop":
            break


if __name__ == "__main__":
    main()
