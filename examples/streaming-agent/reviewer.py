#!/usr/bin/env python3
"""
Reviewer Agent - Reviews completed tasks and requests fixes if needed.

Watches for completed tasks, reviews the implementation against acceptance
criteria, and either approves or creates a fix task for the original worker.

Usage:
    python reviewer.py <store> <repo>
    python reviewer.py alice demo-project

Environment:
    AGENT_ID     - Unique agent identifier (default: reviewer-<pid>)
    BRANCH       - Git branch (default: main)
"""

import os
import sys
import json
import time

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
AGENT_ID = os.environ.get("AGENT_ID", f"reviewer-{os.getpid()}")

POLL_INTERVAL = 5.0

# ---------------------------------------------------------------------------
# Tools for Claude
# ---------------------------------------------------------------------------

TOOLS = [
    {
        "name": "approve",
        "description": "Approve the task implementation - it meets all acceptance criteria.",
        "input_schema": {
            "type": "object",
            "properties": {
                "summary": {
                    "type": "string",
                    "description": "Brief summary of what was implemented correctly",
                },
            },
            "required": ["summary"],
        },
    },
    {
        "name": "request_fix",
        "description": "Request fixes for issues found in the implementation.",
        "input_schema": {
            "type": "object",
            "properties": {
                "issues": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of specific issues that need to be fixed",
                },
                "files_affected": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Which files need to be modified",
                },
            },
            "required": ["issues", "files_affected"],
        },
    },
]


def get_completed_tasks(scraps: ScrapsClient) -> list[tuple[str, str]]:
    """Get all completed task files that haven't been reviewed yet."""
    completed = []
    files = scraps.list_files("tasks")

    for filepath in sorted(files):
        if not filepath.endswith(".md"):
            continue
        # Skip fix tasks
        if "-fix-" in filepath:
            continue

        content = scraps.read_file(filepath)
        if not content:
            continue

        task = parse_task_file(filepath, content)
        if task.status == "completed":
            completed.append((filepath, content))

    return completed


def get_reviewed_tasks(scraps: ScrapsClient) -> set[str]:
    """Get set of task paths that have already been reviewed."""
    reviewed = set()

    tracking_content = scraps.read_file("reviews/.reviewed")
    if tracking_content:
        for line in tracking_content.strip().split("\n"):
            if line.strip():
                reviewed.add(line.strip())

    return reviewed


def get_source_files(scraps: ScrapsClient) -> dict[str, str]:
    """Get all source files from src/ directory."""
    sources = {}
    files = scraps.list_files("src")

    for filepath in files:
        content = scraps.read_file(filepath)
        if content:
            sources[filepath] = content

    return sources


def review_task(scraps: ScrapsClient, task_path: str, task_content: str,
                source_files: dict[str, str], reviewed: set[str]) -> str:
    """Review a completed task. Returns 'approved' or 'fix_requested'."""
    task = parse_task_file(task_path, task_content)

    print(f"\nReviewing: {task.title}")
    print("-" * 40)

    # Build source context - only files owned by this task
    owned_sources = ""
    for path in task.owns:
        if path in source_files:
            owned_sources += f"\n### {path}\n```\n{source_files[path]}\n```\n"

    # Also include related files for context
    other_sources = ""
    for path, content in source_files.items():
        if path not in task.owns:
            other_sources += f"\n### {path} (context)\n```\n{content}\n```\n"

    # Set up Claude agent
    system_prompt = """You are a code reviewer for a multi-agent project.

Your job is to review completed task implementations and either:
1. APPROVE if the code meets all acceptance criteria
2. REQUEST_FIX if there are issues that need to be corrected

Be pragmatic:
- Focus on whether acceptance criteria are met
- Don't nitpick style if functionality is correct
- Consider that this is a demo project, not production code
- If it mostly works, approve it

Only request fixes for real issues:
- Missing required functionality
- Obvious bugs that would prevent the code from working
- Critical missing imports or syntax errors"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    prompt = f"""Please review this completed task implementation.

## Task
{task.body}

## Files Owned by This Task
{owned_sources if owned_sources else "(No files found)"}

## Other Source Files (for context)
{other_sources if other_sources else "(No other files)"}

Review against the acceptance criteria and either approve or request fixes."""

    try:
        response = agent.send(prompt)
    except openai.BadRequestError as e:
        check_api_error(e)
    except openai.APIError as e:
        check_api_error(e)

    message = response.choices[0].message
    result = "approved"  # Default to approved

    if message.content:
        print(message.content)

    if message.tool_calls:
        for tool_call in message.tool_calls:
            name = tool_call.function.name
            try:
                args = json.loads(tool_call.function.arguments)
            except json.JSONDecodeError:
                args = {}

            if name == "approve":
                print(f"\n  ✓ APPROVED: {args.get('summary', 'Looks good')}")
                result = "approved"

                # Update tracking
                new_reviewed = reviewed | {task_path}
                scraps.commit(
                    f"Review approved: {task.title}",
                    {"reviews/.reviewed": "\n".join(sorted(new_reviewed))}
                )

            elif name == "request_fix":
                issues = args.get("issues", [])
                files_affected = args.get("files_affected", [])

                print(f"\n  ✗ FIX REQUESTED:")
                for issue in issues:
                    print(f"    - {issue}")

                result = "fix_requested"

                # Create a fix task
                create_fix_task(scraps, task, issues, files_affected)

    return result


def create_fix_task(scraps: ScrapsClient, original_task, issues: list[str], files_affected: list[str]):
    """Create a fix task for the original task."""
    task_num = original_task.get_task_number()
    fix_filename = f"tasks/{task_num}-fix-{int(time.time())}.md"

    issues_str = "\n".join(f"- {issue}" for issue in issues)
    files_str = ", ".join(files_affected) if files_affected else ", ".join(original_task.owns)

    content = f"""---
status: pending
claimed_by: null
priority: 1
depends_on: []
owns: [{files_str}]
---
# Fix: {original_task.title}

## Description
Fix issues found during code review of the original task.

## Issues to Fix
{issues_str}

## Original Task Reference
{original_task.path}

## Acceptance Criteria
- [ ] All listed issues are resolved
- [ ] Code still meets original acceptance criteria
"""

    scraps.commit(f"Create fix task for: {original_task.title}", {fix_filename: content})
    print(f"  Created fix task: {fix_filename}")


def main():
    print(f"Reviewer {AGENT_ID} starting on {STORE}/{REPO}")
    print("-" * 50)

    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="reviewer")

    reviews_done = 0
    fixes_requested = 0
    consecutive_empty = 0
    max_empty = 20

    try:
        while True:
            completed_tasks = get_completed_tasks(scraps)
            reviewed = get_reviewed_tasks(scraps)
            source_files = get_source_files(scraps)

            # Find tasks that need review
            needs_review = [(p, c) for p, c in completed_tasks if p not in reviewed]

            if not needs_review:
                consecutive_empty += 1
                if consecutive_empty >= max_empty:
                    print(f"\nNo tasks to review for {max_empty} polls, exiting")
                    break

                # Check if there are still pending/in_progress tasks
                all_tasks = scraps.get_all_tasks()
                pending = len([t for t in all_tasks if t.status == "pending"])
                in_progress = len([t for t in all_tasks if t.status == "in_progress"])

                if pending == 0 and in_progress == 0 and len(completed_tasks) > 0:
                    print(f"\nAll tasks completed and reviewed!")
                    break

                print(f"Waiting for tasks to review... (pending: {pending}, in_progress: {in_progress})")
                time.sleep(POLL_INTERVAL)
                continue

            consecutive_empty = 0

            # Review the first unreviewed task
            task_path, task_content = needs_review[0]
            print(f"\nFound task to review: {task_path}")

            result = review_task(scraps, task_path, task_content, source_files, reviewed)
            reviews_done += 1

            if result == "fix_requested":
                fixes_requested += 1

            reviewed.add(task_path)
            print(f"\nReview complete! ({reviews_done} reviewed, {fixes_requested} fixes requested)")

    except KeyboardInterrupt:
        print("\nInterrupted")
    except APICreditsError as e:
        print(f"\n{e}")
        scraps.stream_event("error", error="api_credits_exhausted")
    finally:
        scraps.stream_event("agent_leave", role="reviewer",
                           reviews_done=reviews_done, fixes_requested=fixes_requested)
        print(f"\nReviewer {AGENT_ID} finished. {reviews_done} reviews, {fixes_requested} fixes requested.")


if __name__ == "__main__":
    main()
