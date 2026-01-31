#!/usr/bin/env python3
"""
Generate pre-baked task files for parallel execution.
No LLM needed - just creates the task files directly.
"""

import os
import sys

TASKS = [
    ("slugify", "Convert string to URL-friendly slug", "Remove special chars, replace spaces with hyphens, lowercase"),
    ("capitalize", "Capitalize first letter of each word", "Handle edge cases like empty strings"),
    ("truncate", "Truncate string with ellipsis", "Take string and maxLength, add '...' if truncated"),
    ("debounce", "Debounce function calls", "Return debounced function that delays execution"),
    ("throttle", "Throttle function calls", "Return throttled function that limits execution rate"),
    ("deepClone", "Deep clone an object", "Handle nested objects and arrays, not functions"),
    ("flatten", "Flatten nested arrays", "Recursively flatten to single level"),
    ("unique", "Get unique values from array", "Remove duplicates, preserve order"),
    ("groupBy", "Group array items by key", "Return object with keys and grouped arrays"),
    ("chunk", "Split array into chunks", "Return array of arrays with specified size"),
    ("shuffle", "Randomly shuffle array", "Fisher-Yates shuffle algorithm"),
    ("pick", "Pick specific keys from object", "Return new object with only specified keys"),
    ("omit", "Omit specific keys from object", "Return new object without specified keys"),
    ("merge", "Deep merge objects", "Recursively merge, later values override"),
    ("isEmpty", "Check if value is empty", "Handle null, undefined, empty string/array/object"),
    ("isEqual", "Deep equality check", "Compare objects and arrays recursively"),
    ("randomInt", "Generate random integer in range", "Inclusive min and max"),
    ("sleep", "Promise-based delay", "Return promise that resolves after ms"),
    ("retry", "Retry async function with backoff", "Configurable attempts and delay"),
    ("memoize", "Memoize function results", "Cache based on argument values"),
]


def generate_task_file(num: int, name: str, title: str, detail: str) -> tuple[str, str]:
    """Generate task filename and content."""
    filename = f"tasks/{num:03d}-{name}.md"

    content = f"""---
status: pending
claimed_by: null
priority: 2
depends_on: []
owns: [src/utils/{name}.ts]
---
# Task: Implement {name} utility

## Description
Create `src/utils/{name}.ts` that exports the `{name}` function.

{title}. {detail}.

## Requirements
- Export a single function named `{name}`
- Include TypeScript types
- Handle edge cases
- Keep it simple and focused

## Example
```typescript
export function {name}(...args): ReturnType {{
  // implementation
}}
```

## Acceptance Criteria
- [ ] Function works correctly
- [ ] TypeScript types included
- [ ] Edge cases handled
"""
    return filename, content


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <store> <repo>")
        sys.exit(1)

    store = sys.argv[1]
    repo = sys.argv[2]

    # Import scraps client
    from agent_base import ScrapsClient

    scraps = ScrapsClient(store, repo, "main", "task-generator")

    # Generate all task files
    files = {}

    # Add PRD
    script_dir = os.path.dirname(os.path.abspath(__file__))
    with open(os.path.join(script_dir, "prd-parallel.md")) as f:
        files["prd.md"] = f.read()

    # Generate tasks
    for i, (name, title, detail) in enumerate(TASKS, 1):
        filename, content = generate_task_file(i, name, title, detail)
        files[filename] = content
        print(f"  + {filename}")

    # Commit all at once
    print(f"\nCommitting {len(files)} files...")
    sha = scraps.commit(f"Add PRD and {len(TASKS)} parallel tasks", files)
    print(f"Committed: {sha[:8]}")
    print(f"\nReady for {len(TASKS)} workers!")


if __name__ == "__main__":
    main()
