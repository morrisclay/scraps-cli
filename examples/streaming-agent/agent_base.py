#!/usr/bin/env python3
"""
Shared utilities for multi-agent collaboration.

Provides:
- Scraps API client (claim, release, commit, stream events, read files)
- Task file parsing (YAML frontmatter + markdown body)
- LLM client wrapper (OpenRouter)
- Retry/backoff logic
"""

import os
import sys
import json
import time
import random
import re
import subprocess
import platform
from dataclasses import dataclass
from typing import Optional
from openai import OpenAI
import httpx


# Default model to use via OpenRouter
# Options: google/gemini-2.0-flash-001 (fast+cheap), deepseek/deepseek-chat (cheapest), anthropic/claude-3.5-haiku (reliable)
DEFAULT_MODEL = os.environ.get("OPENROUTER_MODEL", "google/gemini-2.0-flash-001")


# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

def get_scraps_config() -> tuple[str, str]:
    """Get API key and host from scraps CLI config or environment."""
    if "SCRAPS_API_KEY" in os.environ:
        return os.environ["SCRAPS_API_KEY"], os.environ.get("SCRAPS_BASE_URL", "https://api.scraps.sh")

    # Scraps CLI stores credentials in ~/.scraps/credentials.json
    creds_file = os.path.expanduser("~/.scraps/credentials.json")

    try:
        if os.path.exists(creds_file):
            with open(creds_file) as f:
                creds = json.load(f)
            host = os.environ.get("SCRAPS_BASE_URL", "https://api.scraps.sh")
            if host in creds:
                return creds[host].get("api_key"), host
            # Try first available host
            for h, c in creds.items():
                if c.get("api_key"):
                    return c["api_key"], h
    except (FileNotFoundError, json.JSONDecodeError) as e:
        print(f"Error reading credentials: {e}")

    print("Error: Not logged in. Run 'scraps login' or set SCRAPS_API_KEY")
    sys.exit(1)


# ---------------------------------------------------------------------------
# Task file parsing (YAML frontmatter)
# ---------------------------------------------------------------------------

@dataclass
class TaskFile:
    """Parsed task file with YAML frontmatter and markdown body."""
    path: str
    status: str  # pending, in_progress, completed
    claimed_by: Optional[str]
    priority: int
    depends_on: list[str]  # Task numbers this depends on (e.g., ["001", "002"])
    owns: list[str]  # File patterns this task owns (e.g., ["src/auth/*.py"])
    title: str
    body: str
    raw_content: str

    def to_markdown(self) -> str:
        """Convert back to markdown with YAML frontmatter."""
        depends = ", ".join(self.depends_on) if self.depends_on else ""
        owns = ", ".join(self.owns) if self.owns else ""
        lines = [
            "---",
            f"status: {self.status}",
            f"claimed_by: {self.claimed_by or 'null'}",
            f"priority: {self.priority}",
            f"depends_on: [{depends}]",
            f"owns: [{owns}]",
            "---",
            self.body,
        ]
        return "\n".join(lines)

    def get_task_number(self) -> str:
        """Extract task number from path (e.g., 'tasks/001-setup.md' -> '001')."""
        import os
        filename = os.path.basename(self.path)
        if "-" in filename:
            return filename.split("-")[0]
        return ""


def parse_task_file(path: str, content: str) -> TaskFile:
    """Parse a task file with YAML frontmatter."""
    # Split frontmatter from body
    frontmatter = {}
    body = content
    title = ""

    if content.startswith("---"):
        parts = content.split("---", 2)
        if len(parts) >= 3:
            fm_text = parts[1].strip()
            body = parts[2].strip()

            # Simple YAML parsing for our known fields
            for line in fm_text.split("\n"):
                if ":" in line:
                    key, value = line.split(":", 1)
                    key = key.strip()
                    value = value.strip()

                    if value == "null" or value == "":
                        frontmatter[key] = None
                    elif value.startswith("[") and value.endswith("]"):
                        # Parse simple list
                        inner = value[1:-1].strip()
                        if inner:
                            frontmatter[key] = [v.strip() for v in inner.split(",")]
                        else:
                            frontmatter[key] = []
                    elif value.isdigit():
                        frontmatter[key] = int(value)
                    else:
                        frontmatter[key] = value

    # Extract title from body
    for line in body.split("\n"):
        if line.startswith("# "):
            title = line[2:].strip()
            break

    return TaskFile(
        path=path,
        status=frontmatter.get("status", "pending"),
        claimed_by=frontmatter.get("claimed_by"),
        priority=frontmatter.get("priority", 3),
        depends_on=frontmatter.get("depends_on", []),
        owns=frontmatter.get("owns", []),
        title=title,
        body=body,
        raw_content=content,
    )


# ---------------------------------------------------------------------------
# Scraps API Client
# ---------------------------------------------------------------------------

class ScrapsClient:
    """Client for Scraps API with retry logic."""

    def __init__(self, store: str, repo: str, branch: str = "main", agent_id: Optional[str] = None):
        api_key, base_url = get_scraps_config()
        self.store = store
        self.repo = repo
        self.branch = branch
        self.agent_id = agent_id or f"agent-{os.getpid()}"
        self.http = httpx.Client(
            base_url=base_url,
            headers={"Authorization": f"Bearer {api_key}"},
            timeout=30.0,
        )

    def _retry(self, fn, max_attempts: int = 5, base_delay: float = 2.0):
        """Execute fn with exponential backoff on rate limit errors."""
        last_error = None
        for attempt in range(max_attempts):
            try:
                return fn()
            except Exception as e:
                last_error = e
                error_str = str(e).lower()
                status_code = getattr(e, "response", None)
                if status_code:
                    status_code = getattr(status_code, "status_code", None)

                if "rate limit" not in error_str and "429" not in error_str and status_code != 429:
                    raise
                delay = base_delay * (2 ** attempt) + random.uniform(0, 1)
                print(f"  ... Rate limited, retrying in {delay:.1f}s")
                time.sleep(delay)
        raise last_error

    def stream_event(self, event_type: str, **data):
        """Publish event to stream."""
        try:
            r = self.http.post(
                f"/api/v1/stores/{self.store}/repos/{self.repo}/streams/events",
                json={"type": event_type, "agent_id": self.agent_id, **data},
            )
            if r.status_code != 200:
                print(f"  [stream] Failed to send {event_type}: {r.status_code}")
        except httpx.RequestError as e:
            print(f"  [stream] Error sending {event_type}: {e}")

    def claim(self, patterns: list[str], reason: str) -> bool:
        """Claim exclusive access to files."""
        try:
            r = self.http.post(
                f"/stores/{self.store}/repos/{self.repo}/branches/{self.branch}/coordinate/claim",
                json={"agent_id": self.agent_id, "patterns": patterns, "claim": reason},
            )
            if r.status_code == 200:
                self.stream_event("agent_claim", patterns=patterns, reason=reason)
                return True
        except httpx.RequestError:
            pass
        return False

    def release(self, patterns: list[str]):
        """Release file claims."""
        try:
            self.http.request(
                "DELETE",
                f"/stores/{self.store}/repos/{self.repo}/branches/{self.branch}/coordinate/claim",
                json={"agent_id": self.agent_id, "patterns": patterns},
            )
            self.stream_event("agent_release", patterns=patterns)
        except httpx.RequestError:
            pass

    def read_file(self, path: str) -> Optional[str]:
        """Read a file from the repo."""
        try:
            # Endpoint: /api/v1/stores/{store}/repos/{repo}/files/{branch}/{path}
            # Note: path must be URL-encoded (slashes become %2F)
            from urllib.parse import quote
            encoded_path = quote(path, safe='')
            r = self.http.get(
                f"/api/v1/stores/{self.store}/repos/{self.repo}/files/{self.branch}/{encoded_path}",
            )
            if r.status_code == 200:
                data = r.json()
                return data.get("content")
        except httpx.RequestError:
            pass
        return None

    def list_files(self, path: str = "") -> list[str]:
        """List files in the repo directory."""
        try:
            # Endpoint: /api/v1/stores/{store}/repos/{repo}/tree/{branch}/{path}
            url = f"/api/v1/stores/{self.store}/repos/{self.repo}/tree/{self.branch}"
            if path:
                url += f"/{path}"
            r = self.http.get(url)
            if r.status_code == 200:
                data = r.json()
                # Response format: {"entries": [{"name": "file.md", "type": "blob", ...}, ...]}
                entries = data.get("entries", [])
                files = []
                for entry in entries:
                    # type "blob" = file, "tree" = directory
                    if entry.get("type") == "blob":
                        name = entry.get("name")
                        if path:
                            files.append(f"{path}/{name}")
                        else:
                            files.append(name)
                return files
        except httpx.RequestError as e:
            print(f"  Error listing files: {e}")
        return []

    def commit(self, message: str, files: dict[str, str]) -> str:
        """Commit files to git. Returns commit SHA."""
        def do_commit():
            r = self.http.post(
                f"/api/v1/stores/{self.store}/repos/{self.repo}/commits",
                json={
                    "branch": self.branch,
                    "message": message,
                    "author": {"name": self.agent_id, "email": f"{self.agent_id}@agent.local"},
                    "files": [{"path": p, "content": c} for p, c in files.items()],
                },
            )
            data = r.json()
            if "error" in data:
                raise Exception(f"Commit failed: {data['error']}")
            return data["commit"]["commit_sha"]

        return self._retry(do_commit)

    def get_stream_events(self, offset: Optional[str] = None, limit: int = 100) -> tuple[list[dict], Optional[str]]:
        """Get events from stream. Returns (events, next_offset)."""
        try:
            params = {"limit": limit}
            if offset:
                params["offset"] = offset
            r = self.http.get(
                f"/api/v1/stores/{self.store}/repos/{self.repo}/streams/events",
                params=params,
            )
            if r.status_code == 200:
                data = r.json()
                next_offset = r.headers.get("Stream-Next-Offset")
                return data.get("events", []), next_offset
        except httpx.RequestError:
            pass
        return [], None

    def get_all_tasks(self) -> list["TaskFile"]:
        """Get all task files from the repo."""
        tasks = []
        files = self.list_files("tasks")
        for filepath in sorted(files):
            if not filepath.endswith(".md"):
                continue
            content = self.read_file(filepath)
            if content:
                tasks.append(parse_task_file(filepath, content))
        return tasks

    def get_task_by_number(self, task_number: str) -> Optional["TaskFile"]:
        """Get a specific task by its number (e.g., '001')."""
        files = self.list_files("tasks")
        for filepath in files:
            if filepath.startswith(f"tasks/{task_number}-"):
                content = self.read_file(filepath)
                if content:
                    return parse_task_file(filepath, content)
        return None

    def wait_for_dependencies(self, task: "TaskFile", poll_interval: float = 3.0, max_wait: float = 300.0) -> bool:
        """Wait for all dependencies of a task to be completed. Returns True if all completed."""
        if not task.depends_on:
            return True

        start_time = time.time()
        while time.time() - start_time < max_wait:
            all_completed = True
            for dep_num in task.depends_on:
                dep_task = self.get_task_by_number(dep_num)
                if not dep_task or dep_task.status != "completed":
                    all_completed = False
                    print(f"    Waiting for task {dep_num} to complete...")
                    break

            if all_completed:
                return True

            time.sleep(poll_interval)

        print(f"    Timeout waiting for dependencies: {task.depends_on}")
        return False


# ---------------------------------------------------------------------------
# Stream Debouncer (for live streaming)
# ---------------------------------------------------------------------------

class StreamDebouncer:
    """Debounce streaming updates to avoid overwhelming the server."""

    def __init__(self, min_interval: float = 0.5, min_chars: int = 50):
        self.min_interval = min_interval
        self.min_chars = min_chars
        self.last_send_time = 0.0
        self.last_send_length = 0
        self.pending_event: Optional[dict] = None

    def should_send(self, content_length: int) -> bool:
        now = time.time()
        time_elapsed = now - self.last_send_time
        chars_added = content_length - self.last_send_length
        return time_elapsed >= self.min_interval or chars_added >= self.min_chars

    def mark_sent(self, content_length: int):
        self.last_send_time = time.time()
        self.last_send_length = content_length

    def set_pending(self, event: dict):
        self.pending_event = event

    def get_pending(self) -> Optional[dict]:
        event = self.pending_event
        self.pending_event = None
        return event


# ---------------------------------------------------------------------------
# LLM Client Wrapper (OpenRouter)
# ---------------------------------------------------------------------------

def convert_anthropic_tools_to_openai(tools: list[dict]) -> list[dict]:
    """Convert Anthropic tool format to OpenAI function format."""
    openai_tools = []
    for tool in tools:
        openai_tools.append({
            "type": "function",
            "function": {
                "name": tool["name"],
                "description": tool.get("description", ""),
                "parameters": tool.get("input_schema", {"type": "object", "properties": {}}),
            }
        })
    return openai_tools


class LLMAgent:
    """Wrapper for OpenRouter API with tool support."""

    def __init__(self, system_prompt: str, tools: list[dict], model: str = None):
        api_key = os.environ.get("OPENROUTER_API_KEY")
        if not api_key:
            print("Error: OPENROUTER_API_KEY not set")
            sys.exit(1)

        self.client = OpenAI(
            base_url="https://openrouter.ai/api/v1",
            api_key=api_key,
        )
        self.system_prompt = system_prompt
        self.tools = convert_anthropic_tools_to_openai(tools)
        self.model = model or DEFAULT_MODEL
        self.messages: list[dict] = [{"role": "system", "content": system_prompt}]

    def send(self, content: str):
        """Send a message and get response."""
        self.messages.append({"role": "user", "content": content})
        response = self.client.chat.completions.create(
            model=self.model,
            max_tokens=4096,
            tools=self.tools if self.tools else None,
            messages=self.messages,
        )
        return response

    def add_assistant_response(self, response):
        """Add assistant response to message history."""
        msg = response.choices[0].message
        assistant_msg = {"role": "assistant", "content": msg.content or ""}
        if msg.tool_calls:
            assistant_msg["tool_calls"] = [
                {
                    "id": tc.id,
                    "type": "function",
                    "function": {"name": tc.function.name, "arguments": tc.function.arguments}
                }
                for tc in msg.tool_calls
            ]
        self.messages.append(assistant_msg)

    def add_tool_results(self, results: list[dict]):
        """Add tool results to message history."""
        for result in results:
            self.messages.append({
                "role": "tool",
                "tool_call_id": result["tool_use_id"],
                "content": result["content"] if isinstance(result["content"], str) else json.dumps(result["content"]),
            })

    def stream(self, content: str):
        """Stream a message response."""
        self.messages.append({"role": "user", "content": content})
        return self.client.chat.completions.create(
            model=self.model,
            max_tokens=4096,
            tools=self.tools if self.tools else None,
            messages=self.messages,
            stream=True,
        )


# Alias for backward compatibility
ClaudeAgent = LLMAgent
