#!/usr/bin/env python3
"""
Worker Agent - Claims and implements tasks.

Polls the tasks/ directory for pending tasks, claims them using scraps
coordination, implements the task by creating/modifying source files,
updates task status to completed, and loops to find the next task.

Usage:
    python worker.py <store> <repo>
    python worker.py alice demo-project

Environment:
    AGENT_ID     - Unique agent identifier (default: worker-<pid>)
    BRANCH       - Git branch (default: main)
    MAX_TASKS    - Max tasks to complete before exiting (default: unlimited)
"""

import os
import sys
import json
import time

import openai
from agent_base import ScrapsClient, ClaudeAgent, StreamDebouncer, parse_task_file


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
AGENT_ID = os.environ.get("AGENT_ID", f"worker-{os.getpid()}")
MAX_TASKS = int(os.environ.get("MAX_TASKS", "0"))  # 0 = unlimited

POLL_INTERVAL = 3.0  # seconds between polling for tasks

# ---------------------------------------------------------------------------
# Tools for Claude
# ---------------------------------------------------------------------------

TOOLS = [
    {
        "name": "write_file",
        "description": "Write content to a file in the src/ directory.",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "File path (e.g., 'src/auth.py')",
                },
                "content": {
                    "type": "string",
                    "description": "File content",
                },
            },
            "required": ["path", "content"],
        },
    },
    {
        "name": "read_file",
        "description": "Read an existing file from the repo.",
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
        "description": "Mark the task as complete and commit all changes.",
        "input_schema": {
            "type": "object",
            "properties": {
                "commit_message": {
                    "type": "string",
                    "description": "Commit message describing the implementation",
                },
            },
            "required": ["commit_message"],
        },
    },
]


def find_pending_task(scraps: ScrapsClient) -> tuple[str, str] | None:
    """Find a pending task that can be claimed. Returns (path, content) or None."""
    files = scraps.list_files("tasks")

    for filepath in sorted(files):
        if not filepath.endswith(".md"):
            continue

        content = scraps.read_file(filepath)
        if not content:
            continue

        task = parse_task_file(filepath, content)

        # Skip if not pending or already claimed
        if task.status != "pending":
            continue
        if task.claimed_by:
            continue

        return filepath, content

    return None


def claim_task(scraps: ScrapsClient, task_path: str, task_content: str) -> bool:
    """Try to claim a task. Returns True if successful."""
    # Try to claim the file pattern
    if not scraps.claim([task_path], f"Implementing task: {task_path}"):
        return False

    # Update task status to in_progress
    task = parse_task_file(task_path, task_content)
    task.status = "in_progress"
    task.claimed_by = scraps.agent_id

    updated_content = task.to_markdown()

    try:
        scraps.commit(
            f"Claim task: {task.title}",
            {task_path: updated_content}
        )
        return True
    except Exception as e:
        print(f"  Failed to commit claim: {e}")
        scraps.release([task_path])
        return False


def complete_task(scraps: ScrapsClient, task_path: str, task_content: str,
                  pending_files: dict[str, str], commit_message: str) -> str:
    """Mark task as complete and commit all files. Returns commit SHA."""
    task = parse_task_file(task_path, task_content)
    task.status = "completed"

    # Add updated task file to pending files
    pending_files[task_path] = task.to_markdown()

    # Commit everything
    sha = scraps.commit(commit_message, pending_files)

    # Release the claim
    scraps.release([task_path])

    return sha


def implement_task(scraps: ScrapsClient, task_path: str, task_content: str) -> bool:
    """Use Claude to implement a task. Returns True if successful."""
    task = parse_task_file(task_path, task_content)
    pending_files: dict[str, str] = {}
    debouncer = StreamDebouncer()

    print(f"\nImplementing: {task.title}")
    print("-" * 40)

    # Set up Claude agent
    system_prompt = """You are a coding agent implementing a specific task from a project.

Your job is to:
1. Understand the task requirements and acceptance criteria
2. Write clean, working code to implement the task
3. Create all necessary files in the src/ directory
4. Call done when the implementation is complete

Guidelines:
- Write simple, readable code
- Include necessary imports
- Add brief comments for clarity
- Focus on the acceptance criteria
- Keep files small and focused"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    try:
        return _implement_task_loop(agent, scraps, task, task_path, task_content, pending_files, debouncer)
    except openai.BadRequestError as e:
        check_api_error(e)
    except openai.APIError as e:
        check_api_error(e)
    return False


def _implement_task_loop(agent, scraps, task, task_path, task_content, pending_files, debouncer):
    """Inner loop for task implementation."""
    prompt = f"""Please implement this task:

{task.body}

Create the necessary source files and call done when finished."""

    while True:
        # Stream the response
        content_text = ""
        tool_calls = {}  # id -> {name, arguments}
        current_tool_id = None

        stream = agent.stream(prompt)
        for chunk in stream:
            if not chunk.choices:
                continue

            delta = chunk.choices[0].delta

            # Handle text content
            if delta.content:
                print(delta.content, end="", flush=True)
                content_text += delta.content

            # Handle tool calls
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

                                # Stream file content as it's generated
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

        # Process completed tool calls
        tool_results = []
        finished = False

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

                sha = complete_task(scraps, task_path, task_content, pending_files, commit_msg)
                print(f"  Committed: {sha[:8]}")

                tool_results.append({
                    "tool_use_id": tc_id,
                    "content": json.dumps({"ok": True, "commit": sha, "finished": True}),
                })
                finished = True

            else:
                print(f")", flush=True)

        # Build response object for add_assistant_response
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
            return True

        if tool_results:
            agent.add_tool_results(tool_results)
            prompt = ""  # Clear prompt for next iteration
        elif not tool_calls:
            print("  Warning: Agent ended without calling done")
            return False


def main():
    print(f"Worker {AGENT_ID} starting on {STORE}/{REPO}")
    print("-" * 50)

    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="worker")

    tasks_completed = 0
    consecutive_empty = 0
    max_empty = 10  # Exit after 10 consecutive polls with no tasks

    try:
        while True:
            # Check if we've hit the task limit
            if MAX_TASKS > 0 and tasks_completed >= MAX_TASKS:
                print(f"\nCompleted {tasks_completed} tasks, exiting")
                break

            # Find a pending task
            result = find_pending_task(scraps)

            if result is None:
                consecutive_empty += 1
                if consecutive_empty >= max_empty:
                    print(f"\nNo tasks found for {max_empty} polls, exiting")
                    break
                print(f"No pending tasks, waiting... ({consecutive_empty}/{max_empty})")
                time.sleep(POLL_INTERVAL)
                continue

            consecutive_empty = 0
            task_path, task_content = result
            task = parse_task_file(task_path, task_content)

            print(f"\nFound task: {task_path}")
            print(f"  Title: {task.title}")

            # Try to claim it
            print(f"  Claiming...")
            if not claim_task(scraps, task_path, task_content):
                print(f"  Failed to claim (another agent got it?)")
                time.sleep(1)  # Brief pause before trying again
                continue

            print(f"  Claimed!")

            # Implement the task
            if implement_task(scraps, task_path, task_content):
                tasks_completed += 1
                print(f"\nTask completed! ({tasks_completed} total)")
            else:
                print(f"\nTask implementation failed")
                # Release the claim on failure
                scraps.release([task_path])

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
