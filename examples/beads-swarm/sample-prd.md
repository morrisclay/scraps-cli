# PRD: Task Management CLI

Build a command-line task management tool in TypeScript.

## Overview

A simple CLI tool for managing personal tasks with categories, priorities, and due dates.

## Features

### 1. Task CRUD Operations
- Create tasks with title, description, priority (1-5), and optional due date
- List all tasks with filtering by status, priority, or category
- Update task properties
- Delete tasks
- Mark tasks as complete

### 2. Categories
- Create and manage task categories
- Assign tasks to categories
- List tasks by category

### 3. Data Storage
- Store tasks in a local JSON file (~/.tasks.json)
- Auto-create file on first use

### 4. CLI Interface
- `task add "Title" --priority 1 --due "2024-01-15" --category work`
- `task list [--status pending|done] [--priority 1-5] [--category name]`
- `task done <id>`
- `task delete <id>`
- `task categories`
- `task category add <name>`

## Technical Requirements

- TypeScript with strict mode
- Commander.js for CLI parsing
- No external database (JSON file only)
- Cross-platform (macOS, Linux, Windows)

## File Structure

```
src/
  index.ts       - Entry point, CLI setup
  types.ts       - TypeScript interfaces
  store.ts       - JSON file storage
  task.ts        - Task operations
  category.ts    - Category operations
  utils.ts       - Helper functions
package.json
tsconfig.json
```
