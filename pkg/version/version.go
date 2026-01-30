// Package version provides version information for the scraps CLI.
package version

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

var (
	// Version is the semantic version, set via ldflags
	Version = "dev"
	// Commit is the git commit hash, set via ldflags
	Commit = "unknown"
	// Date is the build date, set via ldflags
	Date = "unknown"
)

const (
	releasesURL    = "https://api.github.com/repos/morrisclay/scraps-cli/releases/latest"
	requestTimeout = 2 * time.Second
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// CheckLatest fetches the latest release version from GitHub.
// Returns the latest version string (without 'v' prefix) and any error.
func CheckLatest() (string, error) {
	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Get(releasesURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// IsOutdated compares the current version against the latest.
// Returns true if current is older than latest.
// Returns false for dev builds or if versions can't be compared.
func IsOutdated(current, latest string) bool {
	if current == "dev" || current == "" || latest == "" {
		return false
	}
	// Strip 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	return compareSemver(current, latest) < 0
}

// compareSemver compares two semver strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareSemver(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			aNum = parseVersionPart(aParts[i])
		}
		if i < len(bParts) {
			bNum = parseVersionPart(bParts[i])
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}

func parseVersionPart(s string) int {
	// Handle pre-release suffixes like "1-beta"
	if idx := strings.IndexAny(s, "-+"); idx != -1 {
		s = s[:idx]
	}
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
