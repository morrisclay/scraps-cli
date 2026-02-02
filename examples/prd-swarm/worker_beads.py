#!/usr/bin/env python3
"""
Beads Worker - Phase-aware task implementation.

Unlike atomic workers, beads workers:
- Wait for entire previous phase to complete before starting
- Can write multiple files per task
- Have richer context from previous phases
- Implement related functionality together

Usage:
    python worker_beads.py <store> <repo>
    AGENT_ID=worker-1 python worker_beads.py alice my-project
"""

import os
import sys
import json
import time
import random
import re
from urllib.parse import quote

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
        raise APICreditsError("API credits exhausted.")
    raise e


if len(sys.argv) < 3:
    print(f"Usage: {sys.argv[0]} <store> <repo>")
    sys.exit(1)

STORE = sys.argv[1]
REPO = sys.argv[2]
BRANCH = os.environ.get("BRANCH", "main")
AGENT_ID = os.environ.get("AGENT_ID", f"worker-{os.getpid()}")
MAX_TASKS = int(os.environ.get("MAX_TASKS", "0"))

POLL_INTERVAL = 3.0
MAX_WAIT_FOR_TASKS = 300  # 5 minutes - longer for complex tasks
COMMIT_RETRY_DELAY = 2.0  # Staggered commits to reduce contention


class BeadsTask:
    """Parsed beads-style task file."""

    def __init__(self, path: str, content: str):
        self.path = path
        self.content = content
        self.status = "pending"
        self.claimed_by = None
        self.phase = 0
        self.phase_name = ""
        self.depends_on = []
        self.owns = []
        self.title = ""
        self.body = ""

        self._parse(content)

    def _parse(self, content: str):
        """Parse YAML frontmatter and markdown body."""
        if content.startswith("---"):
            parts = content.split("---", 2)
            if len(parts) >= 3:
                fm_text = parts[1].strip()
                self.body = parts[2].strip()

                for line in fm_text.split("\n"):
                    if ":" in line:
                        key, value = line.split(":", 1)
                        key = key.strip()
                        value = value.strip()

                        if key == "status":
                            self.status = value
                        elif key == "claimed_by":
                            self.claimed_by = None if value == "null" else value
                        elif key == "phase":
                            self.phase = int(value) if value.isdigit() else 0
                        elif key == "phase_name":
                            self.phase_name = value
                        elif key == "depends_on":
                            if value.startswith("[") and value.endswith("]"):
                                inner = value[1:-1].strip()
                                self.depends_on = [v.strip() for v in inner.split(",")] if inner else []
                        elif key == "owns":
                            if value.startswith("[") and value.endswith("]"):
                                inner = value[1:-1].strip()
                                self.owns = [v.strip() for v in inner.split(",")] if inner else []

        # Extract title
        for line in self.body.split("\n"):
            if line.startswith("# "):
                self.title = line[2:].strip()
                break

    def to_markdown(self) -> str:
        """Convert back to markdown."""
        deps_str = ", ".join(self.depends_on) if self.depends_on else ""
        owns_str = ", ".join(self.owns) if self.owns else ""
        return f"""---
status: {self.status}
claimed_by: {self.claimed_by or 'null'}
phase: {self.phase}
phase_name: {self.phase_name}
depends_on: [{deps_str}]
owns: [{owns_str}]
---
{self.body}
"""


TOOLS = [
    {
        "name": "write_file",
        "description": "Write content to a file. You can write MULTIPLE files for this task.",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "File path (e.g., 'src/types/point.rs')",
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
        "description": "Read an existing file. Use to understand code from previous phases.",
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
        "description": "Mark task complete and commit ALL files you've written.",
        "input_schema": {
            "type": "object",
            "properties": {
                "commit_message": {
                    "type": "string",
                    "description": "Commit message describing all implementations",
                },
                "files_created": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of files you created",
                },
            },
            "required": ["commit_message"],
        },
    },
]


def list_tree(scraps: ScrapsClient, path: str = "") -> list[dict]:
    """List all entries (files and directories) in a path."""
    try:
        url = f"/api/v1/stores/{scraps.store}/repos/{scraps.repo}/tree/{scraps.branch}"
        if path:
            # URL-encode the path to handle nested directories (e.g., tasks/001-foundation)
            encoded_path = quote(path, safe='')
            url += f"/{encoded_path}"
        r = scraps.http.get(url)
        if r.status_code == 200:
            data = r.json()
            return data.get("entries", [])
    except Exception:
        pass
    return []


def get_phase_status(scraps: ScrapsClient) -> dict:
    """Get status of all phases and their tasks."""
    phases = {}

    def list_all_tasks(path="tasks"):
        """Recursively list all .md files under tasks/"""
        all_files = []
        entries = list_tree(scraps, path)
        for entry in entries:
            name = entry.get("name", "")
            entry_type = entry.get("type", "")
            full_path = f"{path}/{name}" if path else name

            if entry_type == "blob" and name.endswith(".md"):
                all_files.append(full_path)
            elif entry_type == "tree":
                # Recursively list subdirectory
                subfiles = list_all_tasks(full_path)
                all_files.extend(subfiles)
        return all_files

    all_task_files = list_all_tasks()

    for filepath in all_task_files:
        content = scraps.read_file(filepath)
        if not content:
            continue

        task = BeadsTask(filepath, content)
        phase_num = task.phase

        if phase_num not in phases:
            phases[phase_num] = {
                "name": task.phase_name,
                "tasks": [],
                "completed": 0,
                "total": 0,
            }

        phases[phase_num]["tasks"].append(task)
        phases[phase_num]["total"] += 1
        if task.status == "completed":
            phases[phase_num]["completed"] += 1

    return phases


def is_phase_complete(phases: dict, phase_num: int) -> bool:
    """Check if a phase is fully complete."""
    if phase_num not in phases:
        return True  # Phase doesn't exist, consider complete
    phase = phases[phase_num]
    return phase["completed"] == phase["total"]


def get_available_tasks(phases: dict) -> list[BeadsTask]:
    """Get tasks that are available to work on (phase deps met, pending, unclaimed)."""
    available = []

    for phase_num in sorted(phases.keys()):
        phase = phases[phase_num]

        # Check if previous phase is complete
        if phase_num > 1 and not is_phase_complete(phases, phase_num - 1):
            continue  # Can't start this phase yet

        for task in phase["tasks"]:
            if task.status == "pending" and not task.claimed_by:
                available.append(task)

    # Shuffle for load distribution
    random.shuffle(available)
    return available


def claim_task(scraps: ScrapsClient, task: BeadsTask) -> tuple[bool, list[str]]:
    """Try to claim a task and its owned files."""
    patterns_to_claim = [task.path] + task.owns
    print(f"    Claiming: {patterns_to_claim[:3]}{'...' if len(patterns_to_claim) > 3 else ''}")

    # Add random delay to reduce contention
    time.sleep(random.uniform(0, COMMIT_RETRY_DELAY))

    if not scraps.claim(patterns_to_claim, f"Phase {task.phase}: {task.title}"):
        return False, []

    task.status = "in_progress"
    task.claimed_by = scraps.agent_id

    try:
        scraps.commit(f"Claim: {task.title}", {task.path: task.to_markdown()})
        return True, patterns_to_claim
    except Exception as e:
        print(f"  Failed to commit claim: {e}")
        scraps.release(patterns_to_claim)
        return False, []


def complete_task(scraps: ScrapsClient, task: BeadsTask,
                  pending_files: dict[str, str], commit_message: str,
                  claimed_patterns: list[str]) -> str:
    """Mark task complete and commit all files."""
    task.status = "completed"
    pending_files[task.path] = task.to_markdown()

    # Add staggered delay before commit
    time.sleep(random.uniform(0.5, COMMIT_RETRY_DELAY))

    # Retry commit with longer backoff
    max_retries = 10
    for attempt in range(max_retries):
        try:
            sha = scraps.commit(commit_message, pending_files)
            scraps.release(claimed_patterns)
            return sha
        except Exception as e:
            if "Concurrent modification" in str(e) and attempt < max_retries - 1:
                delay = (2 ** attempt) + random.uniform(0, 2)
                print(f"  Retry commit in {delay:.1f}s...")
                time.sleep(delay)
            else:
                raise

    return ""


def get_phase_context(scraps: ScrapsClient, current_phase: int) -> str:
    """Build context from all previous phases."""
    context = ""

    # Read all source files from previous phases
    try:
        all_files = scraps.list_files("src")
        if all_files:
            context = "\n\n## Existing Code from Previous Phases\n"
            context += "Import and use these as needed:\n\n"

            for filepath in sorted(all_files)[:20]:  # Limit to avoid huge context
                content = scraps.read_file(filepath)
                if content:
                    lang = "rust" if filepath.endswith(".rs") else "typescript" if filepath.endswith(".ts") else ""
                    context += f"### {filepath}\n```{lang}\n{content}\n```\n\n"
    except Exception:
        pass

    return context


def implement_task(scraps: ScrapsClient, task: BeadsTask,
                   claimed_patterns: list[str]) -> bool:
    """Use LLM to implement a task."""
    pending_files: dict[str, str] = {}
    debouncer = StreamDebouncer()

    print(f"\nImplementing Phase {task.phase}: {task.title}")
    print(f"  Files to create: {task.owns}")
    print("-" * 40)

    # Get context from previous phases
    phase_context = get_phase_context(scraps, task.phase)

    # Build file list
    owned_files_str = "\n".join(f"- `{f}`" for f in task.owns) if task.owns else "- (see task description)"

    system_prompt = """You are implementing a task for a complex software project.

CRITICAL RULES:
1. Use write_file for EVERY file you create
2. You may create MULTIPLE files for this task
3. Call done when ALL files are written
4. Implement complete, working code - not stubs

WORKFLOW:
1. Read the task description carefully
2. Review existing code if needed (read_file)
3. Write all required files (write_file - call multiple times if needed)
4. Call done with a commit message listing all files created

QUALITY:
- Write production-quality code
- Include proper error handling
- Use appropriate types and documentation
- Follow the project's patterns from existing code"""

    agent = ClaudeAgent(system_prompt, TOOLS)

    try:
        return _implement_task_loop(
            agent, scraps, task, pending_files, debouncer,
            claimed_patterns, phase_context, owned_files_str
        )
    except openai.BadRequestError as e:
        check_api_error(e)
    except openai.APIError as e:
        check_api_error(e)
    return False


def _implement_task_loop(agent, scraps, task, pending_files, debouncer,
                         claimed_patterns, phase_context, owned_files_str):
    """Inner implementation loop."""

    prompt = f"""Implement this task:

{task.body}

## Files to Create
{owned_files_str}
{phase_context}

Create all required files using write_file, then call done."""

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
                commit_msg = args.get("commit_message", f"Implement: {task.title}")
                files_created = args.get("files_created", list(pending_files.keys()))
                print(f"\n  Committing: {commit_msg}")
                print(f"  Files: {files_created}")

                sha = complete_task(scraps, task, pending_files, commit_msg, claimed_patterns)
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
    print(f"Beads Worker {AGENT_ID} starting on {STORE}/{REPO}")
    print("-" * 50)

    scraps = ScrapsClient(STORE, REPO, BRANCH, AGENT_ID)
    scraps.stream_event("agent_join", agent_name=AGENT_ID, role="worker-beads")

    tasks_completed = 0
    start_time = time.time()

    try:
        while True:
            if MAX_TASKS > 0 and tasks_completed >= MAX_TASKS:
                print(f"\nCompleted {tasks_completed} tasks, exiting")
                break

            # Get phase status
            phases = get_phase_status(scraps)

            if not phases:
                elapsed = time.time() - start_time
                if elapsed > 30:
                    print("No phases found, waiting...")
                time.sleep(POLL_INTERVAL)
                continue

            # Get available tasks
            available = get_available_tasks(phases)

            if not available:
                elapsed = time.time() - start_time

                # Check overall status
                total_tasks = sum(p["total"] for p in phases.values())
                completed_tasks = sum(p["completed"] for p in phases.values())
                in_progress = total_tasks - completed_tasks

                # Find which phase we're waiting on
                waiting_phase = None
                for phase_num in sorted(phases.keys()):
                    if not is_phase_complete(phases, phase_num):
                        waiting_phase = phase_num
                        break

                if completed_tasks == total_tasks:
                    print(f"\nAll {total_tasks} tasks completed!")
                    break

                if elapsed > MAX_WAIT_FOR_TASKS:
                    print(f"\nTimeout after {int(elapsed)}s")
                    break

                if waiting_phase:
                    phase = phases[waiting_phase]
                    print(f"Waiting for Phase {waiting_phase} ({phase['name']}): "
                          f"{phase['completed']}/{phase['total']} complete... ({int(elapsed)}s)")
                else:
                    print(f"Waiting... ({int(elapsed)}s)")

                time.sleep(POLL_INTERVAL)
                continue

            # Try to claim a task
            claimed_task = None
            for task in available:
                print(f"\nTrying: Phase {task.phase} - {task.title}")

                success, claimed_patterns = claim_task(scraps, task)
                if success:
                    print(f"  Claimed!")
                    claimed_task = (task, claimed_patterns)
                    break
                else:
                    print(f"  Already taken, trying next...")

            if not claimed_task:
                time.sleep(1.0)
                continue

            start_time = time.time()
            task, claimed_patterns = claimed_task

            if implement_task(scraps, task, claimed_patterns):
                tasks_completed += 1
                print(f"\nTask completed! ({tasks_completed} total)")
            else:
                print(f"\nTask failed - resetting to pending")
                task.status = "pending"
                task.claimed_by = None
                try:
                    scraps.commit(f"Reset: {task.title}", {task.path: task.to_markdown()})
                except Exception:
                    pass
                scraps.release(claimed_patterns)

    except KeyboardInterrupt:
        print("\nInterrupted")
    except APICreditsError as e:
        print(f"\n{e}")
        scraps.stream_event("error", error="api_credits_exhausted")
    finally:
        scraps.stream_event("agent_leave", role="worker-beads", tasks_completed=tasks_completed)
        print(f"\nBeads Worker {AGENT_ID} finished. Completed {tasks_completed} tasks.")


if __name__ == "__main__":
    main()
