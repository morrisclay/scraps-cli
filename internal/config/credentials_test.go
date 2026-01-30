package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCredentialsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}

	if len(creds) != 0 {
		t.Errorf("Expected empty credentials, got %d entries", len(creds))
	}
}

func TestSetAndGetCredential(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	host := "https://test.example.com"
	cred := Credential{
		APIKey:   "scraps_test123",
		UserID:   "user-abc",
		Username: "testuser",
	}

	if err := SetCredential(host, cred); err != nil {
		t.Fatalf("SetCredential() error = %v", err)
	}

	// Verify file permissions
	credPath := filepath.Join(tmpDir, ".scraps", "credentials.json")
	info, err := os.Stat(credPath)
	if err != nil {
		t.Fatalf("Credentials file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Credentials file permissions = %v, want %v", info.Mode().Perm(), 0600)
	}

	// Get and verify
	got, err := GetCredential(host)
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetCredential() returned nil")
	}

	if got.APIKey != cred.APIKey {
		t.Errorf("APIKey = %v, want %v", got.APIKey, cred.APIKey)
	}
	if got.UserID != cred.UserID {
		t.Errorf("UserID = %v, want %v", got.UserID, cred.UserID)
	}
	if got.Username != cred.Username {
		t.Errorf("Username = %v, want %v", got.Username, cred.Username)
	}
}

func TestRemoveCredential(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	host := "https://test.example.com"
	cred := Credential{
		APIKey:   "scraps_test123",
		UserID:   "user-abc",
		Username: "testuser",
	}

	// Set credential
	if err := SetCredential(host, cred); err != nil {
		t.Fatalf("SetCredential() error = %v", err)
	}

	// Verify it exists
	if !HasCredential(host) {
		t.Fatal("HasCredential() = false, want true")
	}

	// Remove credential
	if err := RemoveCredential(host); err != nil {
		t.Fatalf("RemoveCredential() error = %v", err)
	}

	// Verify it's gone
	if HasCredential(host) {
		t.Error("HasCredential() = true after removal, want false")
	}

	got, err := GetCredential(host)
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}
	if got != nil {
		t.Error("GetCredential() returned non-nil after removal")
	}
}

func TestMultipleHosts(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	hosts := []string{
		"https://host1.example.com",
		"https://host2.example.com",
		"https://host3.example.com",
	}

	// Set credentials for multiple hosts
	for i, host := range hosts {
		cred := Credential{
			APIKey:   "key" + string(rune('A'+i)),
			UserID:   "user" + string(rune('1'+i)),
			Username: "user" + string(rune('a'+i)),
		}
		if err := SetCredential(host, cred); err != nil {
			t.Fatalf("SetCredential(%s) error = %v", host, err)
		}
	}

	// Verify all credentials exist
	for i, host := range hosts {
		got, err := GetCredential(host)
		if err != nil {
			t.Fatalf("GetCredential(%s) error = %v", host, err)
		}
		if got == nil {
			t.Fatalf("GetCredential(%s) = nil", host)
		}
		expectedKey := "key" + string(rune('A'+i))
		if got.APIKey != expectedKey {
			t.Errorf("GetCredential(%s).APIKey = %v, want %v", host, got.APIKey, expectedKey)
		}
	}
}
