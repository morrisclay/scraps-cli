package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com", "test-key")

	if client.Host() != "https://api.example.com" {
		t.Errorf("Host() = %v, want %v", client.Host(), "https://api.example.com")
	}

	if client.APIKey() != "test-key" {
		t.Errorf("APIKey() = %v, want %v", client.APIKey(), "test-key")
	}

	if !client.HasAuth() {
		t.Error("HasAuth() = false, want true")
	}
}

func TestClientHasAuthFalse(t *testing.T) {
	client := NewClient("https://api.example.com", "")

	if client.HasAuth() {
		t.Error("HasAuth() = true, want false")
	}
}

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %v, want GET", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header = %v, want 'Bearer test-key'", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/v1/user" {
			t.Errorf("Path = %v, want /api/v1/user", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":       "user-123",
			"username": "testuser",
			"email":    "test@example.com",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	user, err := client.GetUser()
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("user.ID = %v, want user-123", user.ID)
	}
	if user.Username != "testuser" {
		t.Errorf("user.Username = %v, want testuser", user.Username)
	}
	if user.Email != "test@example.com" {
		t.Errorf("user.Email = %v, want test@example.com", user.Email)
	}
}

func TestClientPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %v, want POST", r.Method)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["slug"] != "test-store" {
			t.Errorf("body.slug = %v, want test-store", body["slug"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":         "store-123",
			"slug":       "test-store",
			"created_at": "2024-01-01T00:00:00Z",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	store, err := client.CreateStore("test-store")
	if err != nil {
		t.Fatalf("CreateStore() error = %v", err)
	}

	if store.ID != "store-123" {
		t.Errorf("store.ID = %v, want store-123", store.ID)
	}
	if store.Slug != "test-store" {
		t.Errorf("store.Slug = %v, want test-store", store.Slug)
	}
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Store not found",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.GetStore("nonexistent")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Expected *APIError, got %T", err)
	}

	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %v, want 404", apiErr.StatusCode)
	}
	if !apiErr.IsNotFound() {
		t.Error("IsNotFound() = false, want true")
	}
}

func TestClientUnauthorizedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid API key",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key")
	_, err := client.GetUser()

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Expected *APIError, got %T", err)
	}

	if !apiErr.IsUnauthorized() {
		t.Error("IsUnauthorized() = false, want true")
	}
}

func TestGetCloneURL(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		apiKey string
		store  string
		repo   string
		want   string
	}{
		{
			name:   "https host",
			host:   "https://api.scraps.sh",
			apiKey: "scraps_abc123",
			store:  "mystore",
			repo:   "myrepo",
			want:   "https://x:scraps_abc123@api.scraps.sh/stores/mystore/repos/myrepo",
		},
		{
			name:   "http host",
			host:   "http://localhost:8080",
			apiKey: "test-key",
			store:  "test",
			repo:   "repo",
			want:   "http://x:test-key@localhost:8080/stores/test/repos/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.host, tt.apiKey)
			got := client.GetCloneURL(tt.store, tt.repo)
			if got != tt.want {
				t.Errorf("GetCloneURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildWebSocketURL(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		apiKey string
		store  string
		repo   string
		branch string
		want   string
	}{
		{
			name:   "https without branch",
			host:   "https://api.scraps.sh",
			apiKey: "key123",
			store:  "mystore",
			repo:   "myrepo",
			branch: "",
			want:   "wss://api.scraps.sh/stores/mystore/repos/myrepo/ws?token=key123",
		},
		{
			name:   "https with branch",
			host:   "https://api.scraps.sh",
			apiKey: "key123",
			store:  "mystore",
			repo:   "myrepo",
			branch: "main",
			want:   "wss://api.scraps.sh/stores/mystore/repos/myrepo/ws?token=key123&branch=main",
		},
		{
			name:   "http host",
			host:   "http://localhost:8080",
			apiKey: "key123",
			store:  "mystore",
			repo:   "myrepo",
			branch: "",
			want:   "ws://localhost:8080/stores/mystore/repos/myrepo/ws?token=key123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.host, tt.apiKey)
			got := client.BuildWebSocketURL(tt.store, tt.repo, tt.branch)
			if got != tt.want {
				t.Errorf("BuildWebSocketURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
