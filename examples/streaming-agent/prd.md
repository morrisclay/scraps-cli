# Product Requirements: Task API

## Overview
A simple REST API for managing tasks, built with Hono (TypeScript).

## Features
1. Task CRUD - create, list, get, update, delete tasks
2. Task status - mark tasks as pending/in_progress/completed
3. Basic validation - required fields, status enum

## Technical Requirements
- Hono framework (lightweight, fast)
- TypeScript
- In-memory storage (Map)
- No database needed

## API Endpoints
- GET /tasks - list all tasks
- POST /tasks - create task
- GET /tasks/:id - get task by id
- PUT /tasks/:id - update task
- DELETE /tasks/:id - delete task

## Task Schema
```typescript
interface Task {
  id: string;
  title: string;
  description?: string;
  status: 'pending' | 'in_progress' | 'completed';
  createdAt: string;
}
```
