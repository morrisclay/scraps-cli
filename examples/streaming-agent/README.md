# Streaming Agent Demo

Watch an AI agent write code in real-time, character by character as Claude generates it.

## One-Command Setup

```bash
# Set your Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-...

# Make sure you're logged into scraps CLI
scraps login

# Run the interactive demo
./demo.sh
```

The demo script will:
- Auto-detect your store name from your login
- Create a demo repository if needed
- Guide you through running the observer and agent

## Manual Quick Start

```bash
# 1. Set your Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-...

# 2. Login to scraps (if not already)
scraps login

# 3. Create a repo
scraps repo create mystore/agent-demo

# 4. Run the agent
python streaming.py mystore agent-demo "Create a hello world script"
```

## Step-by-Step Walkthrough

### Step 1: Create a test repository

```bash
# Create a repo using the CLI
scraps repo create mystore/agent-demo
```

Or check existing repos:
```bash
scraps repo list mystore
```

### Step 2: Open two terminals

**Terminal 1 - Observer** (watch the agent work):
```bash
# Using the scraps CLI watch command (recommended)
scraps watch mystore/agent-demo

# Or use the Python observer with more options
python observer.py --store mystore --repo agent-demo --poll --show-chunks
```

The `--show-chunks` flag shows live content as it's being generated:
```
Watching mystore/agent-demo (polling every 1.0s)...
Showing live content as it's generated...
============================================================
```

**Terminal 2 - Agent** (run the AI):
```bash
python streaming.py mystore agent-demo "Create a Python script that prints hello world"
```

### Step 3: Watch the magic

In Terminal 1 (observer), you'll see real-time events with live content:

```
[12:34:56] > agent-12345 joined
[12:34:57] ... agent-12345 streaming hello.py (45 chars, v1)
----------------------------------------------
    1 | def greet():
    2 |     print("Hello
----------------------------------------------
[12:34:58] ... agent-12345 streaming hello.py (89 chars, v2)
----------------------------------------------
    1 | def greet():
    2 |     print("Hello, World!")
    3 |
    4 | if __name__ == "__main__":
----------------------------------------------
[12:34:59] * agent-12345 wrote hello.py (5 lines, 95 chars)
[12:35:00] o agent-12345 committed a1b2c3d4 to main: Add hello.py (1 files)
[12:35:00] < agent-12345 left
```

### Step 4: Verify the commit

```bash
# Clone using the CLI
scraps clone mystore/agent-demo
cd agent-demo
cat hello.py
```

## How It Works

```
+-------------------------------------------------------------------+
|                       AGENT WORKFLOW                              |
|                                                                   |
|   1. Stream chunks    ->  Partial content via Electric Cloud      |
|   2. write_file(path) ->  Stage file for commit                   |
|   3. done(message)    ->  Commit all staged files (Git API)       |
|                                                                   |
+-------------------------------------------------------------------+
          |                                      |
          v                                      v
   +-------------+                        +-------------+
   |   Streams   |                        |   Git API   |
   |  (Events)   |                        |  (Commits)  |
   +-------------+                        +-------------+
```

**Why stream before commit?**
Observers see work-in-progress character by character. Useful for human oversight, demos, and debugging.

**Why commit at the end?**
Git commits are atomic. If agent crashes mid-task, uncommitted work is visible in stream but doesn't break the repo.

## Production Features

The streaming agent includes production-ready features:

- **Debouncing**: Batches stream updates (every 0.5s or 50 chars) to avoid rate limits
- **Exponential backoff**: Retries on 429/503 errors with increasing delays
- **Graceful degradation**: Stream errors don't block the agent's work

## Files

| File | Description |
|------|-------------|
| `demo.sh` | One-command demo launcher (start here) |
| `streaming.py` | Production-ready streaming agent with debouncing |
| `minimal.py` | ~100 lines, simplified version without streaming |
| `observer.py` | Watch agents work in real-time (`--poll --show-chunks`) |
| `requirements.txt` | Python dependencies |

## Observer Options

### CLI Observer (Recommended)

```bash
# Watch all events on a repo
scraps watch mystore/myrepo

# Watch specific branch
scraps watch mystore/myrepo:main

# Watch specific file path
scraps watch mystore/myrepo --path src/auth.ts

# JSON output for scripting
scraps watch mystore/myrepo -o json
```

### Python Observer (More Options)

```bash
python observer.py --store STORE --repo REPO [options]

Options:
  --poll           Use polling mode (recommended, more reliable than SSE)
  --show-content   Show final file contents after writes
  --show-chunks    Show live content as it streams (real-time typing view)
  --interval N     Poll interval in seconds (default: 1.0)
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Your Anthropic API key |
| `SCRAPS_BASE_URL` | No | API URL (default: https://api.scraps.sh) |
| `AGENT_ID` | No | Custom agent identifier |
| `BRANCH` | No | Git branch (default: main) |

Note: The scraps CLI handles authentication automatically after `scraps login`.

## Troubleshooting

**"not logged in"**
Run `scraps login` and enter your API key.

**"Rate limited" messages**
The agent handles rate limits automatically with exponential backoff. If you see many retries, wait a moment before running again.

**Observer shows nothing**
Make sure both terminals are using the same store/repo. Events only show for that specific repo.

**"Invalid credentials"**
Run `scraps status` to check your login. If needed, run `scraps login` again.
