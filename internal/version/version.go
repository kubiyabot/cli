package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
)

var (
	// Version is the current version of the CLI
	// This will be overridden by ldflags during build
	Version = "dev"

	// These variables are set by goreleaser
	commit  = "unknown"
	date    = "unknown"
	builtBy = "unknown"

	// Cache check results
	lastCheck     time.Time
	latestVersion string
	checkMutex    sync.Mutex
	checkInterval = 24 * time.Hour
)

// SetBuildInfo sets the build information
func SetBuildInfo(commitHash, buildDate, builder string) {
	commit = commitHash
	date = buildDate
	builtBy = builder
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// GetVersion returns the full version string
func GetVersion() string {
	return fmt.Sprintf("%s (commit: %s, built: %s, by: %s)",
		Version, commit, date, builtBy)
}

// CheckForUpdate checks if a new version is available
// Returns: latestVersion, hasUpdate, error
func CheckForUpdate() (string, bool, error) {
	checkMutex.Lock()
	defer checkMutex.Unlock()

	// Use cached result if recent enough
	if time.Since(lastCheck) < checkInterval && latestVersion != "" {
		return latestVersion, latestVersion > Version, nil
	}

	// Check GitHub API for latest release
	resp, err := http.Get("https://api.github.com/repos/kubiyabot/cli/releases/latest")
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false, err
	}

	// Update cache
	lastCheck = time.Now()
	latestVersion = release.TagName

	// Compare versions using semver
	hasUpdate, err := compareVersions(Version, latestVersion)
	if err != nil {
		return latestVersion, false, err
	}

	return latestVersion, hasUpdate, nil
}

// compareVersions compares two semantic version strings
// Returns true if latestVersion > currentVersion
func compareVersions(currentVersion, latestVersion string) (bool, error) {
	// Skip comparison for dev version
	if currentVersion == "dev" {
		return false, nil
	}

	// Parse versions
	current, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse current version %s: %w", currentVersion, err)
	}

	latest, err := semver.NewVersion(latestVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse latest version %s: %w", latestVersion, err)
	}

	// Check if latest is greater than current
	return latest.GreaterThan(current), nil
}

// GetUpdateMessage returns a formatted message about available updates
func GetUpdateMessage() string {
	latest, hasUpdate, err := CheckForUpdate()
	if err != nil || !hasUpdate {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("╔═══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║  📢 UPDATE AVAILABLE!                                    ║\n")
	sb.WriteString("╠═══════════════════════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║  Current version: %-39s ║\n", Version))
	sb.WriteString(fmt.Sprintf("║  Latest version:  %-39s ║\n", latest))
	sb.WriteString("║                                                           ║\n")
	sb.WriteString("║  Run 'kubiya update' to update to the latest version     ║\n")
	sb.WriteString("╚═══════════════════════════════════════════════════════════╝\n")
	sb.WriteString("\n")

	return sb.String()
}
