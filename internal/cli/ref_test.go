package cli

import (
	"testing"
)

func TestParseStoreRepo(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		wantStore string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid reference",
			ref:       "mystore/myrepo",
			wantStore: "mystore",
			wantRepo:  "myrepo",
			wantErr:   false,
		},
		{
			name:      "with hyphens",
			ref:       "my-store/my-repo",
			wantStore: "my-store",
			wantRepo:  "my-repo",
			wantErr:   false,
		},
		{
			name:    "missing slash",
			ref:     "mystore",
			wantErr: true,
		},
		{
			name:    "empty string",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, repo, err := parseStoreRepo(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStoreRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if store != tt.wantStore {
					t.Errorf("parseStoreRepo() store = %v, want %v", store, tt.wantStore)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseStoreRepo() repo = %v, want %v", repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestParseStoreRepoBranch(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantStore  string
		wantRepo   string
		wantBranch string
		wantErr    bool
	}{
		{
			name:       "with branch",
			ref:        "mystore/myrepo:main",
			wantStore:  "mystore",
			wantRepo:   "myrepo",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "without branch",
			ref:        "mystore/myrepo",
			wantStore:  "mystore",
			wantRepo:   "myrepo",
			wantBranch: "",
			wantErr:    false,
		},
		{
			name:       "feature branch",
			ref:        "mystore/myrepo:feature/new-thing",
			wantStore:  "mystore",
			wantRepo:   "myrepo",
			wantBranch: "feature/new-thing",
			wantErr:    false,
		},
		{
			name:    "missing slash",
			ref:     "mystore:main",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, repo, branch, err := parseStoreRepoBranch(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStoreRepoBranch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if store != tt.wantStore {
					t.Errorf("parseStoreRepoBranch() store = %v, want %v", store, tt.wantStore)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseStoreRepoBranch() repo = %v, want %v", repo, tt.wantRepo)
				}
				if branch != tt.wantBranch {
					t.Errorf("parseStoreRepoBranch() branch = %v, want %v", branch, tt.wantBranch)
				}
			}
		})
	}
}

func TestParseStoreRepoBranchPath(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantStore  string
		wantRepo   string
		wantBranch string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "full reference",
			ref:        "mystore/myrepo:main:src/index.ts",
			wantStore:  "mystore",
			wantRepo:   "myrepo",
			wantBranch: "main",
			wantPath:   "src/index.ts",
			wantErr:    false,
		},
		{
			name:       "without path",
			ref:        "mystore/myrepo:main",
			wantStore:  "mystore",
			wantRepo:   "myrepo",
			wantBranch: "main",
			wantPath:   "",
			wantErr:    false,
		},
		{
			name:       "nested path",
			ref:        "mystore/myrepo:main:src/commands/auth.ts",
			wantStore:  "mystore",
			wantRepo:   "myrepo",
			wantBranch: "main",
			wantPath:   "src/commands/auth.ts",
			wantErr:    false,
		},
		{
			name:       "path with colons",
			ref:        "mystore/myrepo:main:file:with:colons.txt",
			wantStore:  "mystore",
			wantRepo:   "myrepo",
			wantBranch: "main",
			wantPath:   "file:with:colons.txt",
			wantErr:    false,
		},
		{
			name:    "missing branch",
			ref:     "mystore/myrepo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, repo, branch, path, err := parseStoreRepoBranchPath(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStoreRepoBranchPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if store != tt.wantStore {
					t.Errorf("parseStoreRepoBranchPath() store = %v, want %v", store, tt.wantStore)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseStoreRepoBranchPath() repo = %v, want %v", repo, tt.wantRepo)
				}
				if branch != tt.wantBranch {
					t.Errorf("parseStoreRepoBranchPath() branch = %v, want %v", branch, tt.wantBranch)
				}
				if path != tt.wantPath {
					t.Errorf("parseStoreRepoBranchPath() path = %v, want %v", path, tt.wantPath)
				}
			}
		})
	}
}

func TestFormatStoreRepo(t *testing.T) {
	got := formatStoreRepo("mystore", "myrepo")
	want := "mystore/myrepo"
	if got != want {
		t.Errorf("formatStoreRepo() = %v, want %v", got, want)
	}
}

func TestFormatStoreRepoBranch(t *testing.T) {
	got := formatStoreRepoBranch("mystore", "myrepo", "main")
	want := "mystore/myrepo:main"
	if got != want {
		t.Errorf("formatStoreRepoBranch() = %v, want %v", got, want)
	}
}
