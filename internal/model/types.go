// Package model defines the data types used throughout the scraps CLI.
package model

import "time"

// User represents an authenticated user.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Store represents a scraps store (organization/namespace).
type Store struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	OwnerUserID *string `json:"owner_user_id,omitempty"`
	OwnerID     *string `json:"owner_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
	Role        string  `json:"role,omitempty"`
}

// StoreMember represents a member of a store.
type StoreMember struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// Repository represents a git repository within a store.
type Repository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch,omitempty"`
	CreatedAt     string `json:"created_at"`
	Store         string `json:"store,omitempty"` // Added by client for convenience
}

// Collaborator represents a collaborator on a repository.
type Collaborator struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// FileTreeEntry represents an entry in a file tree listing.
type FileTreeEntry struct {
	Type string `json:"type"` // "tree" (directory) or "blob" (file)
	Name string `json:"name"`
	SHA  string `json:"sha,omitempty"`
}

// Commit represents a git commit.
type Commit struct {
	SHA       string       `json:"sha,omitempty"`
	Commit    string       `json:"commit,omitempty"`
	Message   string       `json:"message,omitempty"`
	Author    CommitAuthor `json:"author,omitempty"`
	Date      string       `json:"date,omitempty"`
	Timestamp int64        `json:"timestamp,omitempty"`
}

// CommitAuthor can be a string or an object with name/email.
type CommitAuthor struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Raw   string `json:"-"` // Used when author is a plain string
}

// APIKey represents an API key.
type APIKey struct {
	ID         string  `json:"id"`
	Label      string  `json:"label,omitempty"`
	KeyPrefix  string  `json:"key_prefix"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

// ScopedToken represents a scoped access token.
type ScopedToken struct {
	ID        string           `json:"id"`
	Label     string           `json:"label,omitempty"`
	Scope     ScopedTokenScope `json:"scope"`
	CreatedAt string           `json:"created_at"`
	ExpiresAt *string          `json:"expires_at,omitempty"`
}

// ScopedTokenScope defines the scope of a scoped token.
type ScopedTokenScope struct {
	StoreID     *string  `json:"store_id,omitempty"`
	Repos       []string `json:"repos,omitempty"`
	Permissions []string `json:"permissions"`
}

// TokenCreateResponse is returned when creating a new token.
type TokenCreateResponse struct {
	RawKey    string           `json:"raw_key"`
	ID        string           `json:"id"`
	Label     string           `json:"label,omitempty"`
	Scope     ScopedTokenScope `json:"scope,omitempty"`
	ExpiresAt *string          `json:"expires_at,omitempty"`
}

// SignupResponse is returned after successful signup.
type SignupResponse struct {
	APIKey string `json:"api_key"`
	RawKey string `json:"raw_key"`
	User   User   `json:"user"`
}

// ResetConfirmResponse is returned after confirming API key reset.
type ResetConfirmResponse struct {
	APIKey   string `json:"api_key"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// ClaimRequest represents a request to claim file patterns.
type ClaimRequest struct {
	AgentID    string   `json:"agent_id"`
	Patterns   []string `json:"patterns"`
	Claim      string   `json:"claim"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// ClaimResponse is returned after a claim request.
type ClaimResponse struct {
	Type      string          `json:"type,omitempty"`
	Conflicts []ClaimConflict `json:"conflicts,omitempty"`
	ExpiresAt *string         `json:"expires_at,omitempty"`
}

// ClaimConflict represents a conflicting claim.
type ClaimConflict struct {
	AgentName string   `json:"agent_name,omitempty"`
	AgentID   string   `json:"agent_id,omitempty"`
	Patterns  []string `json:"patterns,omitempty"`
	Claim     string   `json:"claim,omitempty"`
}

// ReleaseRequest represents a request to release claimed patterns.
type ReleaseRequest struct {
	AgentID  string   `json:"agent_id"`
	Patterns []string `json:"patterns"`
}

// WsMessage represents a WebSocket message.
type WsMessage struct {
	Type string `json:"type"`
}

// CommitEvent represents a commit event from WebSocket.
type CommitEvent struct {
	Type    string       `json:"type"`
	SHA     string       `json:"sha,omitempty"`
	Message string       `json:"message,omitempty"`
	Branch  string       `json:"branch,omitempty"`
	Files   []FileChange `json:"files,omitempty"`
}

// FileChange represents a file change in a commit.
type FileChange struct {
	Action string `json:"action"` // "add", "delete", "modify"
	Path   string `json:"path"`
}

// BranchEvent represents a branch-related event.
type BranchEvent struct {
	Type   string `json:"type"`
	Branch string `json:"branch,omitempty"`
	Name   string `json:"name,omitempty"`
	Ref    string `json:"ref,omitempty"`
	OldSHA string `json:"oldSha,omitempty"`
	NewSHA string `json:"newSha,omitempty"`
	SHA    string `json:"sha,omitempty"`
}

// ActivityEvent represents a claim/release activity event.
type ActivityEvent struct {
	Type     string   `json:"type"`
	Activity Activity `json:"activity,omitempty"`
	Intents  []any    `json:"intents,omitempty"`
	Presence []any    `json:"presence,omitempty"`
}

// Activity represents claim/release activity details.
type Activity struct {
	Type     string   `json:"type"` // "claim" or "release"
	AgentID  string   `json:"agent_id,omitempty"`
	Patterns []string `json:"patterns,omitempty"`
	Claim    string   `json:"claim,omitempty"`
}

// Reference represents a parsed store/repo:branch:path reference.
type Reference struct {
	Store  string
	Repo   string
	Branch string
	Path   string
}

// ParsedTime parses a time string from the API.
func ParsedTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
