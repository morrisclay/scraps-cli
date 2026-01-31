#!/usr/bin/env python3
"""
Documenter Agent - Watches for completed tasks and generates documentation.

Watches the stream for task completions, reads the task file and implementation,
uses Claude to generate documentation in the docs/ directory, and commits it.

Usage:
    python documenter.py <store> <repo>
    python documenter.py alice demo-project

Environment:
    AGENT_ID     - Unique agent identifier (default: documenter-<pid>)
    BRANCH       - Git branch (default: main)
"""

import os
import sys
import json
import time
import re

import openai
from agent_base import ScrapsClient, ClaudeAgent, parse_task_file


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
AGENT_ID = os.environ.get("AGENT_ID", f"documenter-{os.getpid()}")

POLL_INTERVAL = 5.0  # seconds between checking for completed tasks

# ---------------------------------------------------------------------------
# Tools for Claude
# ---------------------------------------------------------------------------

TOOLS = [
    {
        "name": "write_doc",
        "description": "Write a documentation file in the docs/ directory.",
        "input_schema": {
            "type": "object",
            "properties": {
                "filename": {
                    "type": "string",
                    "description": "Filename (e.g., 'auth.md', 'api.md')",
                },
                "content": {
                    "type": "string",
                    "description": "Markdown documentation content",
                },
            },
            "required": ["filename", "content"],
        },
    },
    {
        "name": "done",
        "description": "Finish documenting this task and commit the docs.",
        "input_schema": {
            "type": "object",
            "properties": {
                "summary": {
                    "type": "string",
                    "description": "Brief summary of documentation created",
                },
            },
            "required": ["summary"],
        },
    },
]


def get_completed_tasks(scraps: ScrapsClient) -> list[tuple[str, str]]:
    """Get all completed task files. Returns list of (path, content)."""
    completed = []
    files = scraps.list_files("tasks")

    for filepath in sorted(files):
        if not filepath.endswith(".md"):
            continue

        content = scraps.read_file(filepath)
        if not content:
            continue

        task = parse_task_file(filepath, content)
        if task.status == "completed":
            completed.append((filepath, content))

    return completed


def get_documented_tasks(scraps: ScrapsClient) -> set[str]:
    """Get set of task paths that have already been documented."""
    documented = set()

    # Check for a tracking file or scan existing docs
    tracking_content = scraps.read_file("docs/.documented")
    if tracking_content:
        for line in tracking_content.strip().split("\n"):
            if line.strip():
                documented.add(line.strip())

    return documented


def get_source_files(scraps: ScrapsClient) -> dict[str, str]:
    """Get all source files from src/ directory."""
    sources = {}
    files = scraps.list_files("src")

    for filepath in files:
        content = scraps.read_file(filepath)
        if content:
            sources[filepath] = content

    return sources


def document_task(scraps: ScrapsClient, task_path: str, task_content: str,
                  source_files: dict[str, str], documented: set[str]) -> bool:
    """Generate documentation for a completed task. Returns True if successful."""
    task = parse_task_file(task_path, task_content)
    pending_files: dict[str, str] = {}

    print(f"\nDocumenting: {task.title}")
    print("-" * 40)

    # Build source context
    source_context = ""
    for path, content in source_files.items():
        source_context += f"\n### {path}\n```python\n{content}\n```\n"

    # Set up Claude agent
    system_prompt = """You are a documentation agent. Your job is to create clear, useful documentation for completed tasks.

Guidelines:
- Write in clear, concise Markdown
- Include code examples from the implementation
- Document the API/interface exposed
- Note any important design decisions
- Keep docs focused and practical"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    prompt = f"""Please create documentation for this completed task.

## Task
{task.body}

## Implementation (source files)
{source_context if source_context else "(No source files found yet)"}

Create appropriate documentation files in docs/ and call done when finished."""

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

                if name == "write_doc":
                    filename = args.get("filename", "doc.md")
                    content = args.get("content", "")
                    path = f"docs/{filename}"
                    pending_files[path] = content
                    print(f"  + {path} ({len(content)} chars)")

                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({"ok": True, "path": path}),
                    })

                elif name == "done":
                    summary = args.get("summary", "Documentation complete")
                    print(f"\n  {summary}")

                    if pending_files:
                        # Update documented tracking
                        new_documented = documented | {task_path}
                        pending_files["docs/.documented"] = "\n".join(sorted(new_documented))

                        # Commit docs
                        print(f"  Committing {len(pending_files)} files...")
                        sha = scraps.commit(
                            f"Add documentation for: {task.title}",
                            pending_files
                        )
                        print(f"  Committed: {sha[:8]}")

                    tool_results.append({
                        "tool_use_id": tool_call.id,
                        "content": json.dumps({"ok": True, "finished": True}),
                    })
                    finished = True

        agent.add_assistant_response(response)

        if finished:
            return True

        if tool_results:
            agent.add_tool_results(tool_results)
            prompt = ""
        elif response.choices[0].finish_reason == "stop":
            print("  Warning: Agent ended without calling done")
            return False


def main():
    print(f"Documenter {AGENT_ID} starting on {STORE}/{REPO}")
    print("-" * 50)

    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="documenter")

    docs_created = 0
    consecutive_empty = 0
    max_empty = 20  # Exit after 20 consecutive polls with no new completed tasks

    try:
        while True:
            # Get current state
            completed_tasks = get_completed_tasks(scraps)
            documented = get_documented_tasks(scraps)
            source_files = get_source_files(scraps)

            # Find tasks that need documentation
            needs_docs = [(p, c) for p, c in completed_tasks if p not in documented]

            if not needs_docs:
                consecutive_empty += 1
                if consecutive_empty >= max_empty:
                    print(f"\nNo new tasks to document for {max_empty} polls, exiting")
                    break
                pending = len([1 for p, c in get_all_tasks(scraps) if parse_task_file(p, c).status == "pending"])
                in_progress = len([1 for p, c in get_all_tasks(scraps) if parse_task_file(p, c).status == "in_progress"])

                if pending == 0 and in_progress == 0 and len(completed_tasks) > 0:
                    print(f"\nAll tasks completed and documented!")
                    break

                print(f"Waiting for completed tasks... (pending: {pending}, in_progress: {in_progress})")
                time.sleep(POLL_INTERVAL)
                continue

            consecutive_empty = 0

            # Document the first undocumented task
            task_path, task_content = needs_docs[0]
            print(f"\nFound undocumented task: {task_path}")

            if document_task(scraps, task_path, task_content, source_files, documented):
                docs_created += 1
                documented.add(task_path)
                print(f"\nDocumented! ({docs_created} total)")
            else:
                print(f"\nDocumentation failed for {task_path}")

    except KeyboardInterrupt:
        print("\nInterrupted")
    except APICreditsError as e:
        print(f"\n{e}")
        scraps.stream_event("error", error="api_credits_exhausted")
    finally:
        scraps.stream_event("agent_leave", role="documenter", docs_created=docs_created)
        print(f"\nDocumenter {AGENT_ID} finished. Created {docs_created} documentation files.")


def get_all_tasks(scraps: ScrapsClient) -> list[tuple[str, str]]:
    """Get all task files. Returns list of (path, content)."""
    tasks = []
    files = scraps.list_files("tasks")

    for filepath in sorted(files):
        if not filepath.endswith(".md"):
            continue

        content = scraps.read_file(filepath)
        if content:
            tasks.append((filepath, content))

    return tasks


if __name__ == "__main__":
    main()
