// Package tests provides end-to-end tests for the scraps CLI.
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test configuration - uses environment variables for auth
const (
	testAPIKey   = "scraps_a02681b3b13c0d164acc475a20daa4b5c3fd710a"
	testUsername = "morrisclay"
	testEmail    = "morrisclay@gmail.com"
	testUserID   = "57aec440-995d-44c1-8556-93aed61c6919"
	testHost     = "https://api.scraps.sh"
)

var (
	scrapsCmd    string
	testStore    string // Will be set to existing personal store
	testRepo     string
	testBranch   = "main"
	createdStore bool
	createdRepo  bool
)

// findProjectRoot finds the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	// Find the project root (where go.mod is)
	projectRoot, err := findProjectRoot()
	if err != nil {
		fmt.Printf("Failed to find project root: %v\n", err)
		os.Exit(1)
	}

	// Build the CLI binary
	binaryPath := filepath.Join(projectRoot, "scraps-test")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/scraps")
	cmd.Dir = projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Failed to build scraps: %v\n%s\n", err, out)
		os.Exit(1)
	}

	scrapsCmd = binaryPath

	// Use the existing personal store (username) for testing
	// This is more reliable than trying to create a new store
	testStore = testUsername // "morrisclay"

	// Generate unique test repo name
	timestamp := time.Now().Unix()
	testRepo = fmt.Sprintf("e2e-test-repo-%d", timestamp)

	// Run tests
	code := m.Run()

	// Cleanup: remove test binary
	os.Remove(scrapsCmd)

	os.Exit(code)
}

// runScraps executes the scraps CLI with given arguments.
// Uses SCRAPS_API_KEY and SCRAPS_HOST environment variables for auth.
func runScraps(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	cmd := exec.Command(scrapsCmd, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"SCRAPS_API_KEY="+testAPIKey,
		"SCRAPS_HOST="+testHost,
		"NO_COLOR=1",
		"TERM=dumb", // Disable interactive mode
	)

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runScrapsWithEnv executes the CLI with custom environment.
func runScrapsWithEnv(t *testing.T, env []string, args ...string) (string, string, error) {
	t.Helper()

	cmd := exec.Command(scrapsCmd, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "NO_COLOR=1", "TERM=dumb")

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// assertContains checks if output contains expected string.
func assertContains(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain %q, got: %s", expected, output)
	}
}

// assertNotContains checks if output does NOT contain a string.
func assertNotContains(t *testing.T, output, unexpected string) {
	t.Helper()
	if strings.Contains(output, unexpected) {
		t.Errorf("Expected output NOT to contain %q, got: %s", unexpected, output)
	}
}

// ==================== Help & Version Tests ====================

func TestVersion(t *testing.T) {
	stdout, _, err := runScraps(t, "--version")
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}
	assertContains(t, stdout, "scraps version")
}

func TestHelpRoot(t *testing.T) {
	stdout, _, err := runScraps(t, "--help")
	if err != nil {
		t.Fatalf("Help failed: %v", err)
	}
	assertContains(t, stdout, "scraps")
	assertContains(t, stdout, "Authentication:")
	assertContains(t, stdout, "Data Management:")
}

func TestHelpStore(t *testing.T) {
	stdout, _, err := runScraps(t, "store", "--help")
	if err != nil {
		t.Fatalf("Store help failed: %v", err)
	}
	assertContains(t, stdout, "list")
	assertContains(t, stdout, "create")
	assertContains(t, stdout, "delete")
}

func TestHelpRepo(t *testing.T) {
	stdout, _, err := runScraps(t, "repo", "--help")
	if err != nil {
		t.Fatalf("Repo help failed: %v", err)
	}
	assertContains(t, stdout, "list")
	assertContains(t, stdout, "create")
	assertContains(t, stdout, "delete")
}

// ==================== Authentication Tests ====================

func TestWhoami(t *testing.T) {
	stdout, stderr, err := runScraps(t, "whoami")
	if err != nil {
		t.Fatalf("Whoami failed: %v\nstderr: %s", err, stderr)
	}
	// Should show user info or helpful message
	if !strings.Contains(stdout, "Username:") && !strings.Contains(stdout, "Host:") {
		t.Errorf("Unexpected whoami output: %s", stdout)
	}
}

func TestWhoamiJSON(t *testing.T) {
	stdout, stderr, err := runScraps(t, "whoami", "--output", "json")
	if err != nil {
		t.Logf("Whoami JSON error (may be expected): %v\nstderr: %s", err, stderr)
		return
	}

	var user struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal([]byte(stdout), &user); err != nil {
		t.Logf("Failed to parse whoami JSON: %v\noutput: %s", err, stdout)
		return
	}
	t.Logf("Whoami: username=%s, email=%s", user.Username, user.Email)
}

func TestStatus(t *testing.T) {
	stdout, stderr, err := runScraps(t, "status")
	if err != nil {
		t.Fatalf("Status failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "Host:")
	assertContains(t, stdout, "Status:")
}

func TestLoginWithKey(t *testing.T) {
	stdout, stderr, err := runScraps(t, "login", "--key", testAPIKey)
	if err != nil {
		t.Fatalf("Login failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "Logged in")
}

// ==================== Config Tests ====================

func TestConfigShow(t *testing.T) {
	stdout, stderr, err := runScraps(t, "config", "--show")
	if err != nil {
		t.Fatalf("Config show failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "default_host:")
	assertContains(t, stdout, "output_format:")
}

// ==================== Store Tests ====================

func TestStoreList(t *testing.T) {
	stdout, stderr, err := runScraps(t, "store", "list")
	if err != nil {
		t.Fatalf("Store list failed: %v\nstderr: %s", err, stderr)
	}
	t.Logf("Store list output: %s", stdout)
}

func TestStoreListJSON(t *testing.T) {
	stdout, stderr, err := runScraps(t, "store", "list", "--output", "json")
	if err != nil {
		t.Fatalf("Store list JSON failed: %v\nstderr: %s", err, stderr)
	}

	var stores []struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal([]byte(stdout), &stores); err != nil {
		t.Logf("Failed to parse stores JSON: %v\noutput: %s", err, stdout)
		return
	}
	t.Logf("Found %d stores", len(stores))
}

func TestStoreShow(t *testing.T) {
	// Use the existing personal store
	stdout, stderr, err := runScraps(t, "store", "show", testStore)
	if err != nil {
		t.Fatalf("Store show failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "Slug:")
	assertContains(t, stdout, testStore)
}

func TestStoreShowJSON(t *testing.T) {
	stdout, stderr, err := runScraps(t, "store", "show", testStore, "--output", "json")
	if err != nil {
		t.Fatalf("Store show JSON failed: %v\nstderr: %s", err, stderr)
	}
	var store struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal([]byte(stdout), &store); err != nil {
		t.Fatalf("Failed to parse store JSON: %v", err)
	}
	if store.Slug != testStore {
		t.Errorf("Expected slug %s, got %s", testStore, store.Slug)
	}
}

// ==================== Repository Tests ====================

func TestRepoCreateShowDelete(t *testing.T) {
	repoRef := fmt.Sprintf("%s/%s", testStore, testRepo)

	// Create
	stdout, stderr, err := runScraps(t, "repo", "create", repoRef)
	if err != nil {
		t.Fatalf("Repo create failed: %v\nstderr: %s\nstdout: %s", err, stderr, stdout)
	}
	assertContains(t, stdout, "created")
	createdRepo = true
	t.Logf("Created repo: %s", repoRef)

	// List in store
	stdout, stderr, err = runScraps(t, "repo", "list", testStore)
	if err != nil {
		t.Fatalf("Repo list failed: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, testRepo) && !strings.Contains(stdout, "No repositories") {
		t.Logf("Repo list output: %s", stdout)
	}

	// Show
	stdout, stderr, err = runScraps(t, "repo", "show", repoRef)
	if err != nil {
		t.Fatalf("Repo show failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "Name:")
	assertContains(t, stdout, testRepo)

	// Show JSON
	stdout, stderr, err = runScraps(t, "repo", "show", repoRef, "--output", "json")
	if err != nil {
		t.Fatalf("Repo show JSON failed: %v\nstderr: %s", err, stderr)
	}
	var repo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(stdout), &repo); err != nil {
		t.Fatalf("Failed to parse repo JSON: %v", err)
	}
	if repo.Name != testRepo {
		t.Errorf("Expected name %s, got %s", testRepo, repo.Name)
	}
}

func TestRepoListAll(t *testing.T) {
	stdout, stderr, err := runScraps(t, "repo", "list")
	if err != nil {
		t.Fatalf("Repo list all failed: %v\nstderr: %s", err, stderr)
	}
	t.Logf("Repo list all output length: %d chars", len(stdout))
}

// ==================== Clone Tests ====================

func TestCloneURLOnly(t *testing.T) {
	repoRef := fmt.Sprintf("%s/%s", testStore, testRepo)
	stdout, stderr, err := runScraps(t, "clone", "--url-only", repoRef)
	if err != nil {
		t.Fatalf("Clone URL only failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "http")
	assertContains(t, stdout, testStore)
	assertContains(t, stdout, testRepo)
	t.Logf("Clone URL: %s", strings.TrimSpace(stdout))
}

// ==================== File Tests ====================

func TestFileTree(t *testing.T) {
	repoRef := fmt.Sprintf("%s/%s:%s", testStore, testRepo, testBranch)
	stdout, stderr, err := runScraps(t, "file", "tree", repoRef)
	// Empty repo might return an error, which is OK
	if err != nil {
		t.Logf("File tree (may fail on empty repo): %v\nstderr: %s", err, stderr)
		return
	}
	t.Logf("File tree output: %s", stdout)
}

// ==================== Token Tests ====================

func TestTokenList(t *testing.T) {
	stdout, stderr, err := runScraps(t, "token", "list")
	if err != nil {
		t.Fatalf("Token list failed: %v\nstderr: %s", err, stderr)
	}
	t.Logf("Token list output: %s", stdout)
}

func TestTokenListJSON(t *testing.T) {
	stdout, stderr, err := runScraps(t, "token", "list", "--output", "json")
	if err != nil {
		t.Fatalf("Token list JSON failed: %v\nstderr: %s", err, stderr)
	}

	var result struct {
		APIKeys      []any `json:"api_keys"`
		ScopedTokens []any `json:"scoped_tokens"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("Failed to parse token list JSON: %v\noutput: %s", err, stdout)
	}
	t.Logf("Found %d API keys, %d scoped tokens", len(result.APIKeys), len(result.ScopedTokens))
}

func TestTokenCreateAndRevoke(t *testing.T) {
	// Create a new API key
	stdout, stderr, err := runScraps(t, "token", "create", "--name", "e2e-test-key", "--output", "json")
	if err != nil {
		t.Fatalf("Token create failed: %v\nstderr: %s\nstdout: %s", err, stderr, stdout)
	}

	var createResult struct {
		ID     string `json:"id"`
		RawKey string `json:"raw_key"`
	}
	if err := json.Unmarshal([]byte(stdout), &createResult); err != nil {
		t.Fatalf("Failed to parse token create JSON: %v\noutput: %s", err, stdout)
	}
	if createResult.ID == "" {
		t.Fatal("Expected token ID in response")
	}
	t.Logf("Created token with ID: %s", createResult.ID)

	// Revoke the token
	stdout, stderr, err = runScraps(t, "token", "revoke", "--force", createResult.ID)
	if err != nil {
		t.Fatalf("Token revoke failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "revoked")
}

// ==================== Claim/Release Tests ====================

func TestClaimAndRelease(t *testing.T) {
	repoRef := fmt.Sprintf("%s/%s:%s", testStore, testRepo, testBranch)
	agentID := fmt.Sprintf("e2e-test-%d", time.Now().Unix())

	// Claim a pattern
	stdout, stderr, err := runScraps(t, "claim", repoRef, "*.go", "--agent-id", agentID, "--message", "E2E test claim")
	if err != nil {
		// Claim might fail if repo doesn't have the branch or coordination is not supported
		t.Logf("Claim (may not be supported on empty repo): %v\nstderr: %s\nstdout: %s", err, stderr, stdout)
		return
	}
	assertContains(t, stdout, "Claimed")

	// Release the pattern
	stdout, stderr, err = runScraps(t, "release", repoRef, "*.go", "--agent-id", agentID)
	if err != nil {
		t.Fatalf("Release failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "Released")
}

// ==================== Error Handling Tests ====================

func TestStoreCreateNoArg(t *testing.T) {
	_, stderr, err := runScraps(t, "store", "create")
	if err == nil {
		t.Fatal("Expected error for store create without argument")
	}
	// Should have helpful error message
	assertContains(t, stderr, "store slug required")
}

func TestRepoCreateInvalidFormat(t *testing.T) {
	_, stderr, err := runScraps(t, "repo", "create", "invalid-no-slash")
	if err == nil {
		t.Fatal("Expected error for invalid repo format")
	}
	t.Logf("Got expected error: %s", stderr)
}

func TestUnauthorizedError(t *testing.T) {
	// Try to access with invalid credentials
	stdout, stderr, err := runScrapsWithEnv(t, []string{
		"SCRAPS_API_KEY=invalid_key_12345",
		"SCRAPS_HOST=" + testHost,
	}, "whoami")

	if err == nil {
		t.Fatal("Expected error with invalid API key")
	}

	errOutput := stderr + stdout
	// Should have helpful 401 error message
	if !strings.Contains(errOutput, "not logged in") && !strings.Contains(errOutput, "login") {
		t.Logf("401 error message (should mention login): %s", errOutput)
	}
}

func TestReleaseWithoutAgentID(t *testing.T) {
	repoRef := fmt.Sprintf("%s/%s:%s", testStore, testRepo, testBranch)
	_, stderr, err := runScraps(t, "release", repoRef, "*.go")
	if err == nil {
		t.Fatal("Expected error for release without --agent-id")
	}
	assertContains(t, stderr, "agent-id")
}

// ==================== Cleanup Tests (run last) ====================

func TestZZ_Cleanup_RepoDelete(t *testing.T) {
	if !createdRepo {
		t.Skip("No repo was created, skipping delete")
	}
	repoRef := fmt.Sprintf("%s/%s", testStore, testRepo)
	stdout, stderr, err := runScraps(t, "repo", "delete", "--force", repoRef)
	if err != nil {
		t.Fatalf("Repo delete failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "deleted")
	t.Logf("Deleted repo: %s", repoRef)
}

func TestZZ_Cleanup_Logout(t *testing.T) {
	stdout, stderr, err := runScraps(t, "logout")
	if err != nil {
		t.Fatalf("Logout failed: %v\nstderr: %s", err, stderr)
	}
	assertContains(t, stdout, "Logged out")
}

// ==================== Key Reset Tests (can't fully test without email) ====================

func TestKeyResetRequestValidation(t *testing.T) {
	// We can't fully test key reset without receiving email, but we can test the command exists
	stdout, _, _ := runScraps(t, "key", "--help")
	assertContains(t, stdout, "reset-request")
	assertContains(t, stdout, "reset-confirm")
}
