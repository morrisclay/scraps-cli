// Package api provides the HTTP client for the scraps API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/morrisclay/scraps-cli/internal/config"
	"github.com/morrisclay/scraps-cli/internal/model"
)

// Client is the HTTP client for the scraps API.
type Client struct {
	host       string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new API client.
func NewClient(host, apiKey string) *Client {
	if host == "" {
		host = config.GetHost()
	}
	return &Client{
		host:       host,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// NewClientFromConfig creates a client using stored credentials.
func NewClientFromConfig(host string) (*Client, error) {
	if host == "" {
		host = config.GetHost()
	}

	cred, err := config.GetCredential(host)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, fmt.Errorf("not logged in to %s", host)
	}

	return NewClient(host, cred.APIKey), nil
}

// Host returns the API host.
func (c *Client) Host() string {
	return c.host
}

// APIKey returns the API key.
func (c *Client) APIKey() string {
	return c.apiKey
}

// HasAuth returns true if the client has authentication.
func (c *Client) HasAuth() bool {
	return c.apiKey != ""
}

// request performs an HTTP request.
func (c *Client) request(method, path string, body any) ([]byte, error) {
	u, err := url.JoinPath(c.host, path)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		msg := string(respBody)
		if json.Unmarshal(respBody, &errResp) == nil {
			if errResp.Message != "" {
				msg = errResp.Message
			} else if errResp.Error != "" {
				msg = errResp.Error
			}
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	return respBody, nil
}

// Get performs a GET request.
func (c *Client) Get(path string, result any) error {
	data, err := c.request("GET", path, nil)
	if err != nil {
		return err
	}
	if result != nil && len(data) > 0 {
		return json.Unmarshal(data, result)
	}
	return nil
}

// Post performs a POST request.
func (c *Client) Post(path string, body, result any) error {
	data, err := c.request("POST", path, body)
	if err != nil {
		return err
	}
	if result != nil && len(data) > 0 {
		return json.Unmarshal(data, result)
	}
	return nil
}

// Put performs a PUT request.
func (c *Client) Put(path string, body, result any) error {
	data, err := c.request("PUT", path, body)
	if err != nil {
		return err
	}
	if result != nil && len(data) > 0 {
		return json.Unmarshal(data, result)
	}
	return nil
}

// Patch performs a PATCH request.
func (c *Client) Patch(path string, body, result any) error {
	data, err := c.request("PATCH", path, body)
	if err != nil {
		return err
	}
	if result != nil && len(data) > 0 {
		return json.Unmarshal(data, result)
	}
	return nil
}

// Delete performs a DELETE request.
func (c *Client) Delete(path string, body any) error {
	_, err := c.request("DELETE", path, body)
	return err
}

// GetRaw performs a GET request and returns raw bytes (for file content).
func (c *Client) GetRaw(path string) ([]byte, error) {
	return c.request("GET", path, nil)
}

// --- User endpoints ---

// GetUser returns the current authenticated user.
func (c *Client) GetUser() (*model.User, error) {
	data, err := c.request("GET", "/api/v1/user", nil)
	if err != nil {
		return nil, err
	}

	// Try direct user object first
	var user model.User
	if err := json.Unmarshal(data, &user); err == nil && user.ID != "" {
		return &user, nil
	}

	// Try wrapped format {"user": {...}}
	var wrapper struct {
		User model.User `json:"user"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.User, nil
}

// Signup creates a new user account.
func (c *Client) Signup(username, email string) (*model.SignupResponse, error) {
	var resp model.SignupResponse
	err := c.Post("/api/v1/signup", map[string]string{
		"username": username,
		"email":    email,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ResetAPIKeyRequest requests an API key reset email.
func (c *Client) ResetAPIKeyRequest(email string) error {
	return c.Post("/api/v1/reset-api-key", map[string]string{"email": email}, nil)
}

// ResetAPIKeyConfirm confirms an API key reset with the token from email.
func (c *Client) ResetAPIKeyConfirm(token string) (*model.ResetConfirmResponse, error) {
	var resp model.ResetConfirmResponse
	if err := c.Get("/api/v1/confirm-reset?token="+url.QueryEscape(token), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Store endpoints ---

// ListStores returns all stores the user is a member of.
func (c *Client) ListStores() ([]model.Store, error) {
	// API may return {"stores": [...]} or just [...]
	data, err := c.request("GET", "/api/v1/stores", nil)
	if err != nil {
		return nil, err
	}

	// Try array first
	var stores []model.Store
	if err := json.Unmarshal(data, &stores); err == nil {
		return stores, nil
	}

	// Try object with stores key
	var wrapper struct {
		Stores []model.Store `json:"stores"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Stores, nil
}

// GetStore returns a store by slug.
func (c *Client) GetStore(slug string) (*model.Store, error) {
	data, err := c.request("GET", "/api/v1/stores/"+url.PathEscape(slug), nil)
	if err != nil {
		return nil, err
	}

	// Try direct store object first
	var store model.Store
	if err := json.Unmarshal(data, &store); err == nil && store.ID != "" {
		return &store, nil
	}

	// Try wrapped format {"store": {...}}
	var wrapper struct {
		Store model.Store `json:"store"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Store, nil
}

// CreateStore creates a new store.
func (c *Client) CreateStore(slug string) (*model.Store, error) {
	var store model.Store
	err := c.Post("/api/v1/stores", map[string]string{"slug": slug}, &store)
	if err != nil {
		return nil, err
	}
	return &store, nil
}

// DeleteStore deletes a store.
func (c *Client) DeleteStore(slug string) error {
	return c.Delete("/api/v1/stores/"+url.PathEscape(slug), nil)
}

// ListStoreMembers returns members of a store.
func (c *Client) ListStoreMembers(slug string) ([]model.StoreMember, error) {
	var members []model.StoreMember
	if err := c.Get("/api/v1/stores/"+url.PathEscape(slug)+"/members", &members); err != nil {
		return nil, err
	}
	return members, nil
}

// AddStoreMember adds a member to a store.
func (c *Client) AddStoreMember(slug, username, role string) (*model.StoreMember, error) {
	var member model.StoreMember
	err := c.Post("/api/v1/stores/"+url.PathEscape(slug)+"/members", map[string]string{
		"username": username,
		"role":     role,
	}, &member)
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// UpdateStoreMember updates a member's role.
func (c *Client) UpdateStoreMember(slug, memberID, role string) error {
	return c.Patch("/api/v1/stores/"+url.PathEscape(slug)+"/members/"+url.PathEscape(memberID), map[string]string{
		"role": role,
	}, nil)
}

// RemoveStoreMember removes a member from a store.
func (c *Client) RemoveStoreMember(slug, memberID string) error {
	return c.Delete("/api/v1/stores/"+url.PathEscape(slug)+"/members/"+url.PathEscape(memberID), nil)
}

// --- Repository endpoints ---

// ListRepos returns all repos in a store.
func (c *Client) ListRepos(store string) ([]model.Repository, error) {
	data, err := c.request("GET", "/api/v1/stores/"+url.PathEscape(store)+"/repos", nil)
	if err != nil {
		return nil, err
	}

	// Try array first
	var repos []model.Repository
	if err := json.Unmarshal(data, &repos); err == nil {
		// Add store name for convenience
		for i := range repos {
			repos[i].Store = store
		}
		return repos, nil
	}

	// Try object with repos key
	var wrapper struct {
		Repos []model.Repository `json:"repos"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	// Add store name for convenience
	for i := range wrapper.Repos {
		wrapper.Repos[i].Store = store
	}
	return wrapper.Repos, nil
}

// ListAllRepos returns all repos across all stores.
func (c *Client) ListAllRepos() ([]model.Repository, error) {
	stores, err := c.ListStores()
	if err != nil {
		return nil, err
	}

	var allRepos []model.Repository
	for _, store := range stores {
		repos, err := c.ListRepos(store.Slug)
		if err != nil {
			continue // Skip stores we can't access
		}
		allRepos = append(allRepos, repos...)
	}
	return allRepos, nil
}

// GetRepo returns a repository.
func (c *Client) GetRepo(store, name string) (*model.Repository, error) {
	path := "/api/v1/stores/" + url.PathEscape(store) + "/repos/" + url.PathEscape(name)
	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}

	// Try direct repo object first
	var repo model.Repository
	if err := json.Unmarshal(data, &repo); err == nil && repo.ID != "" {
		repo.Store = store
		return &repo, nil
	}

	// Try wrapped format {"repo": {...}}
	var wrapper struct {
		Repo model.Repository `json:"repo"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	wrapper.Repo.Store = store
	return &wrapper.Repo, nil
}

// CreateRepo creates a new repository.
func (c *Client) CreateRepo(store, name string) (*model.Repository, error) {
	var repo model.Repository
	err := c.Post("/api/v1/stores/"+url.PathEscape(store)+"/repos", map[string]string{
		"name": name,
	}, &repo)
	if err != nil {
		return nil, err
	}
	repo.Store = store
	return &repo, nil
}

// DeleteRepo deletes a repository.
func (c *Client) DeleteRepo(store, name string) error {
	return c.Delete("/api/v1/stores/"+url.PathEscape(store)+"/repos/"+url.PathEscape(name), nil)
}

// ListCollaborators returns collaborators of a repository.
func (c *Client) ListCollaborators(store, repo string) ([]model.Collaborator, error) {
	var collabs []model.Collaborator
	path := "/api/v1/stores/" + url.PathEscape(store) + "/repos/" + url.PathEscape(repo) + "/collaborators"
	if err := c.Get(path, &collabs); err != nil {
		return nil, err
	}
	return collabs, nil
}

// AddCollaborator adds a collaborator to a repository.
func (c *Client) AddCollaborator(store, repo, username, role string) (*model.Collaborator, error) {
	var collab model.Collaborator
	path := "/api/v1/stores/" + url.PathEscape(store) + "/repos/" + url.PathEscape(repo) + "/collaborators"
	err := c.Post(path, map[string]string{
		"username": username,
		"role":     role,
	}, &collab)
	if err != nil {
		return nil, err
	}
	return &collab, nil
}

// RemoveCollaborator removes a collaborator from a repository.
func (c *Client) RemoveCollaborator(store, repo, collabID string) error {
	path := "/api/v1/stores/" + url.PathEscape(store) + "/repos/" + url.PathEscape(repo) + "/collaborators/" + url.PathEscape(collabID)
	return c.Delete(path, nil)
}

// --- File endpoints ---

// GetFileTree returns the file tree for a path.
func (c *Client) GetFileTree(store, repo, branch, path string) ([]model.FileTreeEntry, error) {
	var entries []model.FileTreeEntry
	apiPath := "/api/v1/stores/" + url.PathEscape(store) + "/repos/" + url.PathEscape(repo) + "/tree/" + url.PathEscape(branch)
	if path != "" {
		apiPath += "/" + path
	}
	if err := c.Get(apiPath, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetFileContent returns the content of a file.
func (c *Client) GetFileContent(store, repo, branch, path string) ([]byte, error) {
	apiPath := "/api/v1/stores/" + url.PathEscape(store) + "/repos/" + url.PathEscape(repo) + "/files/" + url.PathEscape(branch) + "/" + path
	return c.GetRaw(apiPath)
}

// GetLog returns the commit log for a branch.
func (c *Client) GetLog(store, repo, branch string, limit int) ([]model.Commit, error) {
	var commits []model.Commit
	path := fmt.Sprintf("/api/v1/stores/%s/repos/%s/log/%s?limit=%d",
		url.PathEscape(store), url.PathEscape(repo), url.PathEscape(branch), limit)
	if err := c.Get(path, &commits); err != nil {
		return nil, err
	}
	return commits, nil
}

// --- Token endpoints ---

// ListAPIKeys returns all API keys.
func (c *Client) ListAPIKeys() ([]model.APIKey, error) {
	data, err := c.request("GET", "/api/v1/api-keys", nil)
	if err != nil {
		return nil, err
	}

	// Try array first
	var keys []model.APIKey
	if err := json.Unmarshal(data, &keys); err == nil {
		return keys, nil
	}

	// Try object with api_keys key
	var wrapper struct {
		APIKeys []model.APIKey `json:"api_keys"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.APIKeys, nil
}

// CreateAPIKey creates a new API key.
func (c *Client) CreateAPIKey(label string) (*model.TokenCreateResponse, error) {
	body := map[string]string{}
	if label != "" {
		body["label"] = label
	}

	data, err := c.request("POST", "/api/v1/api-keys", body)
	if err != nil {
		return nil, err
	}

	// Try direct response first
	var resp model.TokenCreateResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.RawKey != "" {
		return &resp, nil
	}

	// Try wrapped format {"api_key": {...}}
	var wrapper struct {
		APIKey model.TokenCreateResponse `json:"api_key"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.APIKey, nil
}

// RevokeAPIKey revokes an API key.
func (c *Client) RevokeAPIKey(id string) error {
	return c.Delete("/api/v1/api-keys/"+url.PathEscape(id), nil)
}

// ListScopedTokens returns all scoped tokens.
func (c *Client) ListScopedTokens() ([]model.ScopedToken, error) {
	data, err := c.request("GET", "/api/v1/scoped-tokens", nil)
	if err != nil {
		return nil, err
	}

	// Try array first
	var tokens []model.ScopedToken
	if err := json.Unmarshal(data, &tokens); err == nil {
		return tokens, nil
	}

	// Try object with scoped_tokens key
	var wrapper struct {
		ScopedTokens []model.ScopedToken `json:"scoped_tokens"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.ScopedTokens, nil
}

// CreateScopedToken creates a new scoped token.
func (c *Client) CreateScopedToken(label, storeID string, repos, permissions []string, expiresInDays int) (*model.TokenCreateResponse, error) {
	var resp model.TokenCreateResponse
	body := map[string]any{
		"permissions": permissions,
	}
	if label != "" {
		body["label"] = label
	}
	if storeID != "" {
		body["store_id"] = storeID
	}
	if len(repos) > 0 {
		body["repos"] = repos
	}
	if expiresInDays > 0 {
		body["expires_in_days"] = expiresInDays
	}
	if err := c.Post("/api/v1/scoped-tokens", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RevokeScopedToken revokes a scoped token.
func (c *Client) RevokeScopedToken(id string) error {
	return c.Delete("/api/v1/scoped-tokens/"+url.PathEscape(id), nil)
}

// --- Coordination endpoints ---

// Claim claims file patterns.
func (c *Client) Claim(store, repo, branch string, req model.ClaimRequest) (*model.ClaimResponse, error) {
	var resp model.ClaimResponse
	path := fmt.Sprintf("/stores/%s/repos/%s/branches/%s/coordinate/claim",
		url.PathEscape(store), url.PathEscape(repo), url.PathEscape(branch))
	if err := c.Post(path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Release releases claimed file patterns.
func (c *Client) Release(store, repo, branch string, req model.ReleaseRequest) error {
	path := fmt.Sprintf("/stores/%s/repos/%s/branches/%s/coordinate/claim",
		url.PathEscape(store), url.PathEscape(repo), url.PathEscape(branch))
	return c.Delete(path, req)
}

// --- Helper functions ---

// GetCloneURL returns the git clone URL for a repository.
func (c *Client) GetCloneURL(store, repo string) string {
	// Convert https:// to protocol with auth
	host := c.host
	protocol := "https"
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
		protocol = "http"
	}

	return fmt.Sprintf("%s://x:%s@%s/stores/%s/repos/%s",
		protocol, c.apiKey, host, url.PathEscape(store), url.PathEscape(repo))
}

// BuildWebSocketURL returns the WebSocket URL for watching a repository.
func (c *Client) BuildWebSocketURL(store, repo string, branch string) string {
	host := c.host
	protocol := "wss"
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
		protocol = "ws"
	}

	wsURL := fmt.Sprintf("%s://%s/stores/%s/repos/%s/ws?token=%s",
		protocol, host, url.PathEscape(store), url.PathEscape(repo), url.QueryEscape(c.apiKey))

	if branch != "" {
		wsURL += "&branch=" + url.QueryEscape(branch)
	}

	return wsURL
}

// BuildClaimsWebSocketURL returns the WebSocket URL for watching claims.
// Deprecated: Use BuildStreamURL instead.
func (c *Client) BuildClaimsWebSocketURL(store, repo, branch string) string {
	host := c.host
	protocol := "wss"
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
		protocol = "ws"
	}

	return fmt.Sprintf("%s://%s/stores/%s/repos/%s/branches/%s/coordinate/tail?token=%s",
		protocol, host, url.PathEscape(store), url.PathEscape(repo), url.PathEscape(branch), url.QueryEscape(c.apiKey))
}

// BuildStreamURL returns the URL for the event streaming endpoint.
func (c *Client) BuildStreamURL(store, repo string) string {
	return fmt.Sprintf("%s/api/v1/stores/%s/repos/%s/streams/events/live",
		c.host, url.PathEscape(store), url.PathEscape(repo))
}
