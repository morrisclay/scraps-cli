package cli

import (
	"fmt"
	"strings"

	"github.com/scraps-sh/scraps-cli/internal/model"
)

// parseStoreRepo parses a "store/repo" reference.
func parseStoreRepo(ref string) (store, repo string, err error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid reference: expected store/repo, got %q", ref)
	}
	return parts[0], parts[1], nil
}

// parseStoreRepoBranch parses a "store/repo:branch" reference.
func parseStoreRepoBranch(ref string) (store, repo, branch string, err error) {
	// Split on colon first
	colonParts := strings.SplitN(ref, ":", 2)
	storeRepo := colonParts[0]

	if len(colonParts) == 2 {
		branch = colonParts[1]
	}

	// Split store/repo
	slashParts := strings.SplitN(storeRepo, "/", 2)
	if len(slashParts) != 2 {
		return "", "", "", fmt.Errorf("invalid reference: expected store/repo[:branch], got %q", ref)
	}

	return slashParts[0], slashParts[1], branch, nil
}

// parseStoreRepoBranchPath parses a "store/repo:branch:path" reference.
func parseStoreRepoBranchPath(ref string) (store, repo, branch, path string, err error) {
	// Split on colons
	parts := strings.SplitN(ref, ":", 3)
	if len(parts) < 2 {
		return "", "", "", "", fmt.Errorf("invalid reference: expected store/repo:branch[:path], got %q", ref)
	}

	storeRepo := parts[0]
	branch = parts[1]
	if len(parts) == 3 {
		path = parts[2]
	}

	// Split store/repo
	slashParts := strings.SplitN(storeRepo, "/", 2)
	if len(slashParts) != 2 {
		return "", "", "", "", fmt.Errorf("invalid reference: expected store/repo:branch[:path], got %q", ref)
	}

	return slashParts[0], slashParts[1], branch, path, nil
}

// parseReference parses any reference format and returns a Reference.
func parseReference(ref string) (*model.Reference, error) {
	r := &model.Reference{}

	// Count colons to determine format
	colonCount := strings.Count(ref, ":")

	switch colonCount {
	case 0:
		// store/repo format
		store, repo, err := parseStoreRepo(ref)
		if err != nil {
			return nil, err
		}
		r.Store = store
		r.Repo = repo

	case 1:
		// store/repo:branch format
		store, repo, branch, err := parseStoreRepoBranch(ref)
		if err != nil {
			return nil, err
		}
		r.Store = store
		r.Repo = repo
		r.Branch = branch

	default:
		// store/repo:branch:path format
		store, repo, branch, path, err := parseStoreRepoBranchPath(ref)
		if err != nil {
			return nil, err
		}
		r.Store = store
		r.Repo = repo
		r.Branch = branch
		r.Path = path
	}

	return r, nil
}

// formatStoreRepo formats a store/repo reference.
func formatStoreRepo(store, repo string) string {
	return store + "/" + repo
}

// formatStoreRepoBranch formats a store/repo:branch reference.
func formatStoreRepoBranch(store, repo, branch string) string {
	return store + "/" + repo + ":" + branch
}
