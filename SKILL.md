# AGENTS.md - Multi-Agent Collaboration Guide

This document instructs AI coding assistants (Claude Code, Gemini CLI, Cursor, etc.) on how to safely collaborate on shared codebases using the scraps.sh git server.

## Overview

Scraps is a git hosting service with built-in coordination primitives for multi-agent development. When multiple AI agents work on the same repository simultaneously, they must coordinate to avoid conflicts. The core primitives are:

- **Claim** - Reserve file patterns before editing
- **Release** - Relinquish claims when done
- **Watch** - Monitor repository activity and claim changes in real-time

## Prerequisites

Before using scraps commands, ensure you're authenticated:

```bash
scraps login
scraps status  # Verify connection
```

## Core Workflow

### 1. Before Making Changes: Claim Your Files

**Always claim the files you intend to modify before editing them.**

```bash
scraps claim <store/repo:branch> <patterns...> -m "<description>"
```

Examples:
```bash
# Claim a specific file
scraps claim alice/webapp:main "src/components/Button.tsx" -m "Fixing button styling"

# Claim a directory
scraps claim alice/webapp:main "src/api/**" -m "Adding authentication endpoints"

# Claim multiple patterns
scraps claim alice/webapp:main "src/utils/**" "tests/utils/**" -m "Refactoring utility functions"
```

**Options:**
- `-m, --message <msg>` - Describe your planned changes (required for coordination)
- `--ttl <seconds>` - Claim duration (default: 300s / 5 minutes)
- `--agent-id <id>` - Your identifier (auto-generated if omitted)

**Important:** Save the agent-id from your claim - you'll need it to release.

### 2. Handle Claim Conflicts

If another agent has already claimed overlapping files, you'll receive a `claim_conflict` error showing:
- Which patterns conflict
- Which agent holds the conflicting claim
- Their stated intent

**When you encounter a conflict:**
1. Wait for the other agent to release their claim
2. Use `scraps watch <store/repo:branch> --claims` to monitor when files become available
3. Choose different files to work on
4. Coordinate with the other agent if possible

### 3. Make Your Changes

Once your claim is successful:
1. Make your code changes
2. Commit and push using standard git commands
3. Release your claim

### 4. Release When Done

**Always release your claims after pushing changes or abandoning work.**

```bash
scraps release <store/repo:branch> <patterns...> --agent-id <your-agent-id>
```

Example:
```bash
scraps release alice/webapp:main "src/api/**" --agent-id cli-abc12345
```

## Monitoring Repository Activity

### Watch for Changes

Monitor commits and branch updates:
```bash
scraps watch alice/webapp           # All branches
scraps watch alice/webapp -b main   # Specific branch
```

### Watch for Claim Activity

Monitor when files become available or claimed:
```bash
scraps watch alice/webapp:main --claims
```

This streams events when agents claim or release files, helping you know when contested files become available.

## Git Best Practices for Multi-Agent Collaboration

### Pull Before Claiming

```bash
git pull origin main
scraps claim myteam/project:main "src/**" -m "Adding feature X"
```

### Small, Focused Claims

Claim only what you need. Broad claims like `"**"` block other agents unnecessarily.

**Good:**
```bash
scraps claim team/app:main "src/auth/login.ts" -m "Fixing login bug"
```

**Avoid:**
```bash
scraps claim team/app:main "src/**" -m "Fixing login bug"  # Too broad
```

### Short Claim Durations

Use appropriate TTLs. If your task is quick, use a shorter TTL:
```bash
scraps claim team/app:main "README.md" -m "Updating docs" --ttl 60
```

### Commit Frequently, Release Promptly

1. Make incremental commits
2. Push when you reach a stable state
3. Release claims immediately after pushing

```bash
git add -A && git commit -m "Add login validation"
git push origin main
scraps release team/app:main "src/auth/**" --agent-id cli-abc123
```

### Handle Stale Claims

Claims expire after their TTL. If you need more time:
1. Release your current claim
2. Re-claim with a fresh TTL

## Reference Formats

| Format | Example | Usage |
|--------|---------|-------|
| Store/Repo | `alice/my-project` | Repo operations, clone |
| Store/Repo:Branch | `alice/my-project:main` | Claims, releases, branch-specific watch |
| Store/Repo:Branch:Path | `alice/my-project:main:src/index.ts` | File read operations |

## Pattern Syntax

Claims use glob patterns:

| Pattern | Matches |
|---------|---------|
| `src/index.ts` | Exact file |
| `src/*.ts` | All .ts files in src/ |
| `src/**` | Everything under src/ recursively |
| `**/*.test.ts` | All test files anywhere |
| `src/{api,utils}/**` | Both api and utils directories |

## Command Reference

### Claim
```bash
scraps claim <store/repo:branch> <patterns...> [options]
  -m, --message <msg>     Description of planned changes
  --agent-id <id>         Your agent identifier
  --ttl <seconds>         Claim duration (default: 300)
```

### Release
```bash
scraps release <store/repo:branch> <patterns...> --agent-id <id>
```

### Watch
```bash
scraps watch <store/repo[:branch]> [options]
  -b, --branch <branch>   Filter to specific branch
  --claims                Watch claim/release activity (requires branch)
  --last-event <id>       Resume from event ID
```

### Clone
```bash
scraps clone <store/repo> [directory]
  --url-only              Print clone URL only
```

### File Operations
```bash
scraps file read <store/repo:branch:path>    # Read file content
scraps file tree <store/repo:branch> [path]  # List directory
scraps log <store/repo:branch> [-n <count>]  # Commit history
```

## Example Multi-Agent Session

**Agent A** (working on API):
```bash
scraps claim team/app:main "src/api/**" "tests/api/**" -m "Adding user endpoints"
# ... makes changes ...
git add -A && git commit -m "Add user CRUD endpoints"
git push origin main
scraps release team/app:main "src/api/**" "tests/api/**" --agent-id cli-aaa111
```

**Agent B** (working on UI, runs concurrently):
```bash
scraps claim team/app:main "src/components/**" -m "Building user profile component"
# ... makes changes ...
git add -A && git commit -m "Add UserProfile component"
git push origin main
scraps release team/app:main "src/components/**" --agent-id cli-bbb222
```

**Agent C** (wants API files, must wait):
```bash
scraps claim team/app:main "src/api/users.ts" -m "Fixing user validation"
# Error: claim_conflict - Agent cli-aaa111 has claimed src/api/**

# Watch for availability:
scraps watch team/app:main --claims
# ... waits for release event ...

# Try again after Agent A releases:
scraps claim team/app:main "src/api/users.ts" -m "Fixing user validation"
# Success!
```

## Error Handling

| Error | Meaning | Action |
|-------|---------|--------|
| `claim_conflict` | Another agent holds conflicting claim | Wait, watch, or choose different files |
| `not_found` | Repo or branch doesn't exist | Verify the reference format |
| `unauthorized` | Not logged in or no access | Run `scraps login` or check permissions |
| `release_failed` | Agent ID doesn't match claim | Use the same agent-id from your claim |

## Summary

1. **Always claim before editing** - Prevents conflicts with other agents
2. **Use specific patterns** - Don't over-claim
3. **Release promptly** - Free files for others when done
4. **Watch for availability** - Monitor contested files
5. **Pull before claiming** - Start from latest code
6. **Push before releasing** - Ensure your changes are saved
