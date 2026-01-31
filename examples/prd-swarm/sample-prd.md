# Sample PRD: Task Management API

## Overview
Build a complete REST API for task management with user authentication.

## Technical Stack
- TypeScript
- Hono framework (lightweight HTTP server)
- In-memory storage (for simplicity)
- JWT for authentication

## Features

### 1. User Authentication
- Register new users (POST /auth/register)
- Login and get JWT token (POST /auth/login)
- Protected routes require Bearer token

### 2. Task Management
- Create task (POST /tasks) - requires auth
- List all tasks (GET /tasks) - requires auth, returns user's tasks
- Get task by ID (GET /tasks/:id) - requires auth
- Update task (PUT /tasks/:id) - requires auth
- Delete task (DELETE /tasks/:id) - requires auth

### 3. Task Categories
- Create category (POST /categories)
- List categories (GET /categories)
- Assign task to category (PUT /tasks/:id with categoryId)

## Data Models

```typescript
interface User {
  id: string;
  email: string;
  passwordHash: string;
  createdAt: string;
}

interface Task {
  id: string;
  title: string;
  description?: string;
  status: 'pending' | 'in_progress' | 'completed';
  categoryId?: string;
  userId: string;
  createdAt: string;
  updatedAt: string;
}

interface Category {
  id: string;
  name: string;
  color: string;
}
```

## API Response Format
```typescript
// Success
{ "data": T }

// Error
{ "error": { "code": string, "message": string } }
```

## Validation Requirements
- Email must be valid format
- Password minimum 8 characters
- Task title required, max 200 chars
- Category name required, max 50 chars
