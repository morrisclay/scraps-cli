#!/usr/bin/env python3
"""
Universal PRD Worker - Claims and implements tasks.

Polls for available tasks, claims them using scraps coordination,
implements the code, and commits. Designed for parallel execution
with N other workers.

Usage:
    python worker.py <store> <repo>
    AGENT_ID=worker-1 python worker.py alice my-project
"""

import os
import sys
import json
import time
import random

# Add streaming-agent directory to path for shared utilities
PARENT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STREAMING_AGENT_DIR = os.path.join(PARENT_DIR, "streaming-agent")
sys.path.insert(0, STREAMING_AGENT_DIR)
from agent_base import ScrapsClient, ClaudeAgent, StreamDebouncer, parse_task_file

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


def find_available_tasks(scraps: ScrapsClient) -> list[tuple[str, str]]:
    """Find all available tasks (pending, deps met). Returns shuffled list."""
    files = scraps.list_files("tasks")

    # Get all tasks to check dependencies
    all_tasks = {}
    task_contents = {}
    for filepath in files:
        if not filepath.endswith(".md"):
            continue
        content = scraps.read_file(filepath)
        if content:
            task = parse_task_file(filepath, content)
            task_num = task.get_task_number()
            if task_num:
                all_tasks[task_num] = task
                task_contents[filepath] = content

    # Find available tasks
    available = []
    for filepath, content in task_contents.items():
        task = parse_task_file(filepath, content)

        if task.status != "pending":
            continue

        if task.claimed_by:
            continue

        # Check dependencies
        deps_met = True
        for dep_num in task.depends_on:
            dep_task = all_tasks.get(dep_num)
            if not dep_task or dep_task.status != "completed":
                deps_met = False
                break

        if deps_met:
            available.append((filepath, content))

    # Shuffle for load distribution across workers
    random.shuffle(available)
    return available


def claim_task(scraps: ScrapsClient, task_path: str, task_content: str) -> tuple[bool, list[str]]:
    """Try to claim a task and its owned files."""
    task = parse_task_file(task_path, task_content)

    patterns_to_claim = [task_path] + task.owns
    print(f"    Claiming: {patterns_to_claim}")

    if not scraps.claim(patterns_to_claim, f"Implementing: {task.title}"):
        return False, []

    # Update task status
    task.status = "in_progress"
    task.claimed_by = scraps.agent_id

    try:
        scraps.commit(f"Claim task: {task.title}", {task_path: task.to_markdown()})
        return True, patterns_to_claim
    except Exception as e:
        print(f"  Failed to commit claim: {e}")
        scraps.release(patterns_to_claim)
        return False, []


def complete_task(scraps: ScrapsClient, task_path: str, task_content: str,
                  pending_files: dict[str, str], commit_message: str,
                  claimed_patterns: list[str]) -> str:
    """Mark task complete and commit all files."""
    task = parse_task_file(task_path, task_content)
    task.status = "completed"

    pending_files[task_path] = task.to_markdown()

    sha = scraps.commit(commit_message, pending_files)
    scraps.release(claimed_patterns)

    return sha


def implement_task(scraps: ScrapsClient, task_path: str, task_content: str,
                   claimed_patterns: list[str]) -> bool:
    """Use LLM to implement a task."""
    task = parse_task_file(task_path, task_content)
    pending_files: dict[str, str] = {}
    debouncer = StreamDebouncer()

    print(f"\nImplementing: {task.title}")
    print(f"  Owns: {task.owns}")
    print("-" * 40)

    # Read existing files for context
    existing_files = {}

    # Try to read common dependency files
    common_paths = ["src/types.ts", "src/store.ts", "src/auth.ts", "package.json", "tsconfig.json"]
    all_src = scraps.list_files("src")

    for filepath in common_paths + all_src:
        if filepath in task.owns:
            continue  # Don't read our own file
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

    owned_files_str = "\n".join(f"- `{f}`" for f in task.owns) if task.owns else "- (none)"

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
4. Call done with a descriptive commit message

Example:
- Task owns: src/auth.ts
- You call: write_file("src/auth.ts", "import { User } from './types';...")
- Then call: done("Implement JWT authentication")"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    try:
        return _implement_task_loop(
            agent, scraps, task, task_path, task_content,
            pending_files, debouncer, claimed_patterns,
            existing_context, owned_files_str
        )
    except openai.BadRequestError as e:
        check_api_error(e)
    except openai.APIError as e:
        check_api_error(e)
    return False


def _implement_task_loop(agent, scraps, task, task_path, task_content,
                         pending_files, debouncer, claimed_patterns,
                         existing_context, owned_files_str):
    """Inner implementation loop."""

    prompt = f"""Implement this task:

# Task: {task.title}

## Description
{task.body}

## Files You Own (only write to these)
{owned_files_str}
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

                sha = complete_task(scraps, task_path, task_content, pending_files,
                                    commit_msg, claimed_patterns)
                print(f"  Committed: {sha[:8]}")

                tool_results.append({
                    "tool_use_id": tc_id,
                    "content": json.dumps({"ok": True, "commit": sha, "finished": True}),
                })
                finished = True

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
            return True

        if tool_results:
            agent.add_tool_results(tool_results)
            prompt = ""
        elif not tool_calls:
            print("  Warning: Agent ended without calling done")
            return False


def main():
    print(f"Worker {AGENT_ID} starting on {STORE}/{REPO}")
    print("-" * 50)

    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="worker")

    tasks_completed = 0
    start_time = time.time()

    try:
        while True:
            if MAX_TASKS > 0 and tasks_completed >= MAX_TASKS:
                print(f"\nCompleted {tasks_completed} tasks, exiting")
                break

            available = find_available_tasks(scraps)

            if not available:
                elapsed = time.time() - start_time

                all_tasks = scraps.get_all_tasks()
                pending = [t for t in all_tasks if t.status == "pending"]
                in_progress = [t for t in all_tasks if t.status == "in_progress"]
                completed = [t for t in all_tasks if t.status == "completed"]

                if len(pending) == 0 and len(in_progress) == 0:
                    if len(completed) > 0:
                        print(f"\nAll tasks completed!")
                    else:
                        print(f"\nNo tasks found")
                    break

                if elapsed > MAX_WAIT_FOR_TASKS:
                    print(f"\nTimeout waiting for tasks after {int(elapsed)}s")
                    break

                if in_progress:
                    print(f"Waiting for {len(in_progress)} in-progress task(s)... ({int(elapsed)}s)")
                else:
                    print(f"Waiting for tasks... ({int(elapsed)}s)")

                time.sleep(POLL_INTERVAL)
                continue

            # Try to claim a task
            claimed_task = None
            for task_path, task_content in available:
                task = parse_task_file(task_path, task_content)

                print(f"\nTrying: {task_path}")
                print(f"  Title: {task.title}")

                success, claimed_patterns = claim_task(scraps, task_path, task_content)
                if success:
                    print(f"  Claimed!")
                    claimed_task = (task_path, task_content, claimed_patterns)
                    break
                else:
                    print(f"  Already taken, trying next...")

            if not claimed_task:
                time.sleep(0.5)
                continue

            start_time = time.time()
            task_path, task_content, claimed_patterns = claimed_task

            if implement_task(scraps, task_path, task_content, claimed_patterns):
                tasks_completed += 1
                print(f"\nTask completed! ({tasks_completed} total)")
            else:
                print(f"\nTask failed - resetting to pending")
                task = parse_task_file(task_path, task_content)
                task.status = "pending"
                task.claimed_by = None
                try:
                    scraps.commit(f"Reset failed task: {task.title}", {task_path: task.to_markdown()})
                except Exception:
                    pass
                scraps.release(claimed_patterns)

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
