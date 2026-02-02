#!/usr/bin/env python3
"""
Beads Swarm Worker - Claims and implements tasks using Beads + Scraps.

Uses:
- Beads tasks.json (exported to Scraps) for task discovery
- Scraps file claims for coordination between workers
- Scraps commits for atomic file updates

Usage:
    python worker.py <store> <repo>
    AGENT_ID=worker-1 python worker.py alice my-project
"""

import os
import sys
import json
import time
import random
import subprocess
import tempfile

# Add streaming-agent directory to path for shared utilities
PARENT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STREAMING_AGENT_DIR = os.path.join(PARENT_DIR, "streaming-agent")
sys.path.insert(0, STREAMING_AGENT_DIR)
from agent_base import ScrapsClient, ClaudeAgent, StreamDebouncer

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


if len(sys.argv) < 3:
    print(f"Usage: {sys.argv[0]} <store> <repo>")
    sys.exit(1)

STORE = sys.argv[1]
REPO = sys.argv[2]
BRANCH = os.environ.get("BRANCH", "main")
AGENT_ID = os.environ.get("AGENT_ID", f"worker-{os.getpid()}")
MAX_TASKS = int(os.environ.get("MAX_TASKS", "0"))  # 0 = unlimited
WORK_DIR = os.environ.get("BEADS_WORK_DIR", "")

POLL_INTERVAL = 2.0
MAX_WAIT_FOR_TASKS = 180  # 3 minutes


TOOLS = [
    {
        "name": "write_file",
        "description": "Write content to a file. Use this to create your implementation.",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "File path (e.g., 'src/auth.ts', 'package.json')",
                },
                "content": {
                    "type": "string",
                    "description": "Complete file content",
                },
            },
            "required": ["path", "content"],
        },
    },
    {
        "name": "read_file",
        "description": "Read an existing file from the repo. Use to understand existing code you need to import from.",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "File path to read",
                },
            },
            "required": ["path"],
        },
    },
    {
        "name": "done",
        "description": "Mark task complete and commit. Call this when you've written all required files.",
        "input_schema": {
            "type": "object",
            "properties": {
                "commit_message": {
                    "type": "string",
                    "description": "Commit message describing implementation",
                },
            },
            "required": ["commit_message"],
        },
    },
]


class BeadsTask:
    """Represents a Beads task from tasks.json."""
    def __init__(self, data: dict):
        self.id = data.get("id", "")
        self.title = data.get("title", "")
        self.description = data.get("description", "")
        self.status = data.get("status", "open")
        self.priority = data.get("priority", 2)
        self.assignee = data.get("assignee", "")
        self.notes = data.get("notes", "")
        self.labels = data.get("labels", [])

        # Parse owned file from notes (format: "owns:path/to/file")
        self.owns = ""
        if self.notes and "owns:" in self.notes:
            for part in self.notes.split("\n"):
                if part.startswith("owns:"):
                    self.owns = part[5:].strip()
                    break


def get_tasks_from_scraps(scraps: ScrapsClient) -> list[BeadsTask]:
    """Get all tasks from the tasks.json file in scraps."""
    content = scraps.read_file(".beads/tasks.json")
    if not content:
        return []
    try:
        tasks_data = json.loads(content)
        return [BeadsTask(t) for t in tasks_data]
    except json.JSONDecodeError:
        return []


def get_ready_tasks(scraps: ScrapsClient, completed_files: set[str]) -> list[BeadsTask]:
    """
    Get tasks that are ready to work on.

    A task is ready if:
    - Its owned file hasn't been completed yet
    - Its owned file can be claimed in Scraps
    """
    all_tasks = get_tasks_from_scraps(scraps)

    ready = []
    for task in all_tasks:
        if not task.owns:
            continue
        # Skip if we already completed this file
        if task.owns in completed_files:
            continue
        # Check if file already exists (someone else completed it)
        existing = scraps.read_file(task.owns)
        if existing:
            completed_files.add(task.owns)
            continue
        ready.append(task)

    # Shuffle for load distribution
    random.shuffle(ready)
    return ready


def try_claim_task(scraps: ScrapsClient, task: BeadsTask, completed_files: set[str]) -> bool:
    """
    Try to claim a task's file using Scraps coordination.

    Returns True if successfully claimed, False otherwise.
    """
    if not task.owns:
        return False

    # Try to claim the file in Scraps
    if scraps.claim([task.owns], f"Implementing: {task.title}"):
        return True

    # Claim failed - check if file was completed by someone else
    existing = scraps.read_file(task.owns)
    if existing:
        completed_files.add(task.owns)

    return False


def implement_task(scraps: ScrapsClient, task: BeadsTask) -> tuple[bool, str]:
    """
    Use LLM to implement a task.

    Returns (success, commit_sha).
    """
    pending_files: dict[str, str] = {}
    debouncer = StreamDebouncer()

    print(f"\nImplementing: {task.title}")
    print(f"  ID: {task.id}")
    print(f"  Owns: {task.owns}")
    print("-" * 40)

    # Read existing files for context
    existing_files = {}
    common_paths = ["src/types.ts", "src/store.ts", "src/auth.ts", "package.json", "tsconfig.json"]
    all_src = scraps.list_files("src")

    for filepath in common_paths + all_src:
        if filepath == task.owns:
            continue
        content = scraps.read_file(filepath)
        if content:
            existing_files[filepath] = content
            print(f"  Context: {filepath}")

    # Build context string
    existing_context = ""
    if existing_files:
        existing_context = "\n\n## Existing Code (import from these as needed)\n"
        for path, content in existing_files.items():
            lang = "typescript" if path.endswith(".ts") else "json" if path.endswith(".json") else ""
            existing_context += f"\n### {path}\n```{lang}\n{content}\n```\n"

    system_prompt = """You are a coding agent implementing a single task. You MUST use the provided tools.

CRITICAL RULES:
1. ALWAYS use write_file tool to create files - never output code as text
2. ALWAYS call done when finished to commit your work
3. Only write to files you own (listed in the task)
4. Import from existing files when needed

WORKFLOW:
1. Understand the task requirements
2. Read existing files if you need to import from them
3. Write your implementation using write_file
4. Call done with a descriptive commit message"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    try:
        return _implement_task_loop(
            agent, scraps, task, pending_files, debouncer, existing_context
        )
    except openai.BadRequestError as e:
        check_api_error(e)
    except openai.APIError as e:
        check_api_error(e)
    return False, ""


def _implement_task_loop(agent, scraps, task, pending_files, debouncer, existing_context):
    """Inner implementation loop."""

    prompt = f"""Implement this task:

# Task: {task.title}

## Description
{task.description}

## File You Own (only write to this)
- `{task.owns}`
{existing_context}

Remember:
- Use write_file to create your implementation
- Import from existing files as needed
- Call done when finished"""

    while True:
        content_text = ""
        tool_calls = {}
        current_tool_id = None

        stream = agent.stream(prompt)
        for chunk in stream:
            if not chunk.choices:
                continue

            delta = chunk.choices[0].delta

            if delta.content:
                print(delta.content, end="", flush=True)
                content_text += delta.content

            if delta.tool_calls:
                for tc in delta.tool_calls:
                    tc_id = tc.id or current_tool_id
                    if tc.id:
                        current_tool_id = tc.id
                        tool_calls[tc_id] = {"name": "", "arguments": ""}

                    if tc_id and tc_id in tool_calls:
                        if tc.function:
                            if tc.function.name:
                                tool_calls[tc_id]["name"] = tc.function.name
                                if tc.function.name != "write_file":
                                    print(f"\n-> {tc.function.name}(", end="", flush=True)
                            if tc.function.arguments:
                                tool_calls[tc_id]["arguments"] += tc.function.arguments

                                # Stream file writes
                                if tool_calls[tc_id]["name"] == "write_file":
                                    try:
                                        partial = json.loads(tool_calls[tc_id]["arguments"] + '"}')
                                        if "content" in partial:
                                            current_path = partial.get("path", "")
                                            current_content = partial["content"]
                                            if current_path and debouncer.should_send(len(current_content)):
                                                scraps.stream_event(
                                                    "file_chunk",
                                                    path=current_path,
                                                    content=current_content,
                                                    version=len(current_content),
                                                )
                                                debouncer.mark_sent(len(current_content))
                                                print(f"\r  Writing {current_path}: {len(current_content)} chars", end="", flush=True)
                                    except json.JSONDecodeError:
                                        pass

        # Process tool calls
        tool_results = []
        finished = False
        commit_sha = ""

        for tc_id, tc_data in tool_calls.items():
            name = tc_data["name"]
            try:
                args = json.loads(tc_data["arguments"]) if tc_data["arguments"] else {}
            except json.JSONDecodeError:
                args = {}

            if name == "write_file":
                path = args.get("path", "")
                content = args.get("content", "")
                if path:
                    pending_files[path] = content
                    scraps.stream_event("file_write", path=path, content=content)
                    print(f"\n  + {path} ({len(content)} chars)")

                tool_results.append({
                    "tool_use_id": tc_id,
                    "content": json.dumps({"ok": True, "path": path}),
                })

            elif name == "read_file":
                path = args.get("path", "")
                content = scraps.read_file(path)
                if content:
                    print(f"  < Read {path}")
                    tool_results.append({
                        "tool_use_id": tc_id,
                        "content": content,
                    })
                else:
                    tool_results.append({
                        "tool_use_id": tc_id,
                        "content": json.dumps({"error": "File not found"}),
                    })

            elif name == "done":
                commit_msg = args.get("commit_message", "Implementation complete")
                print(f"\n  Committing: {commit_msg}")

                # Commit to scraps
                if pending_files:
                    commit_sha = scraps.commit(f"{commit_msg} (closes {task.id})", pending_files)
                    print(f"  Committed: {commit_sha[:8]}")

                    # Release scraps claim
                    scraps.release([task.owns])

                    tool_results.append({
                        "tool_use_id": tc_id,
                        "content": json.dumps({"ok": True, "commit": commit_sha, "finished": True}),
                    })
                    finished = True
                else:
                    tool_results.append({
                        "tool_use_id": tc_id,
                        "content": json.dumps({"error": "No files to commit"}),
                    })

            else:
                print(")", flush=True)

        # Build response for history
        class FakeResponse:
            class FakeChoice:
                class FakeMessage:
                    def __init__(self, content, tool_calls_list):
                        self.content = content
                        self.tool_calls = tool_calls_list
                def __init__(self, content, tool_calls_list):
                    self.message = self.FakeMessage(content, tool_calls_list)
            def __init__(self, content, tool_calls_dict):
                tc_list = []
                for tc_id, tc_data in tool_calls_dict.items():
                    class FakeTC:
                        def __init__(self, id, name, args):
                            self.id = id
                            class FakeFunc:
                                def __init__(self, n, a):
                                    self.name = n
                                    self.arguments = a
                            self.function = FakeFunc(name, args)
                    tc_list.append(FakeTC(tc_id, tc_data["name"], tc_data["arguments"]))
                self.choices = [self.FakeChoice(content, tc_list)]

        agent.add_assistant_response(FakeResponse(content_text, tool_calls))

        if finished:
            return True, commit_sha

        if tool_results:
            agent.add_tool_results(tool_results)
            prompt = ""
        elif not tool_calls:
            print("  Warning: Agent ended without calling done")
            return False, ""


def main():
    print(f"Worker {AGENT_ID} starting on {STORE}/{REPO}")
    print("-" * 50)

    # Initialize scraps client
    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="worker")

    # Track completed files locally
    completed_files: set[str] = set()

    tasks_completed = 0
    start_time = time.time()

    try:
        while True:
            if MAX_TASKS > 0 and tasks_completed >= MAX_TASKS:
                print(f"\nCompleted {tasks_completed} tasks, exiting")
                break

            # Get ready tasks from scraps
            ready_tasks = get_ready_tasks(scraps, completed_files)

            if not ready_tasks:
                elapsed = time.time() - start_time

                # Check total task count
                all_tasks = get_tasks_from_scraps(scraps)
                total = len(all_tasks)
                completed = len(completed_files)

                if completed >= total and total > 0:
                    print(f"\nAll tasks completed! ({completed}/{total})")
                    break

                if elapsed > MAX_WAIT_FOR_TASKS:
                    print(f"\nTimeout waiting for tasks after {int(elapsed)}s")
                    print(f"  Completed: {completed}/{total}")
                    break

                remaining = total - completed
                print(f"Waiting for tasks... ({remaining} remaining, {int(elapsed)}s)")
                time.sleep(POLL_INTERVAL)
                continue

            # Try to claim a task using Scraps coordination
            claimed_task = None
            for task in ready_tasks:
                print(f"\nTrying: {task.id}")
                print(f"  Title: {task.title}")
                print(f"  File: {task.owns}")

                if try_claim_task(scraps, task, completed_files):
                    print(f"  Claimed!")
                    claimed_task = task
                    break
                else:
                    print(f"  Already taken, trying next...")

            if not claimed_task:
                time.sleep(0.5)
                continue

            # Reset timer when we get work
            start_time = time.time()

            success, commit_sha = implement_task(scraps, claimed_task)
            if success:
                tasks_completed += 1
                completed_files.add(claimed_task.owns)
                print(f"\nTask completed! ({tasks_completed} total)")
            else:
                print(f"\nTask failed - releasing claim")
                scraps.release([claimed_task.owns])

    except KeyboardInterrupt:
        print("\nInterrupted")
    except APICreditsError as e:
        print(f"\n{e}")
        scraps.stream_event("error", error="api_credits_exhausted")
    finally:
        scraps.stream_event("agent_leave", role="worker", tasks_completed=tasks_completed)
        print(f"\nWorker {AGENT_ID} finished. Completed {tasks_completed} tasks.")


if __name__ == "__main__":
    main()
