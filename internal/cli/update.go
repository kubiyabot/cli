package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/version"
	"github.com/spf13/cobra"
)

const (
	githubAPIURL = "https://api.github.com/repos/kubiyabot/cli/releases/latest"
	owner        = "kubiyabot"
	repo         = "cli"
)

type githubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func newUpdateCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "🔄 Update Kubiya CLI to the latest version",
		Long: `Check for and install the latest version of Kubiya CLI.
The command will only update if a newer version is available, unless --force is used.`,
		Example: `  # Check for updates
  kubiya update

  # Force update to latest version
  kubiya update --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get current version
			currentVersion := version.Version

			// Get latest release info from GitHub
			release, err := getLatestRelease()
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			latestVersion := release.TagName

			// Compare versions
			needsUpdate, err := isUpdateAvailable(currentVersion, latestVersion)
			if err != nil {
				return fmt.Errorf("failed to compare versions: %w", err)
			}

			if !force && !needsUpdate {
				fmt.Printf("✨ You're already running the latest version (%s)\n", currentVersion)
				return nil
			}

			fmt.Printf("🔄 Updating from %s to %s...\n", currentVersion, latestVersion)

			// Find the appropriate asset for the current platform
			assetURL := ""
			expectedName := fmt.Sprintf("kubiya-cli-%s-%s", runtime.GOOS, runtime.GOARCH)
			if runtime.GOOS == "windows" {
				expectedName += ".exe"
			}

			for _, asset := range release.Assets {
				if strings.Contains(asset.Name, expectedName) {
					assetURL = asset.BrowserDownloadURL
					break
				}
			}

			if assetURL == "" {
				return fmt.Errorf("no compatible binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
			}

			// Download checksums file
			fmt.Printf("📥 Downloading checksums...\n")
			checksums, err := downloadChecksums(latestVersion)
			if err != nil {
				fmt.Printf("⚠️  Warning: Could not verify checksums: %v\n", err)
				fmt.Printf("    Continuing without checksum verification...\n")
			}

			// Download the new binary
			fmt.Printf("📥 Downloading new version...\n")
			resp, err := http.Get(assetURL)
			if err != nil {
				return fmt.Errorf("failed to download update: %w", err)
			}
			defer resp.Body.Close()

			// Create temporary file
			tmpFile, err := os.CreateTemp("", "kubiya-update-*")
			if err != nil {
				return fmt.Errorf("failed to create temporary file: %w", err)
			}
			defer os.Remove(tmpFile.Name())

			// Set executable permissions
			if runtime.GOOS != "windows" {
				if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
					return fmt.Errorf("failed to set executable permissions: %w", err)
				}
			}

			// Copy the downloaded binary to temp file and calculate checksum
			hasher := sha256.New()
			multiWriter := io.MultiWriter(tmpFile, hasher)
			if _, err := io.Copy(multiWriter, resp.Body); err != nil {
				return fmt.Errorf("failed to save update: %w", err)
			}
			tmpFile.Close()

			// Verify checksum if available
			if checksums != nil {
				binaryName := expectedName
				downloadedChecksum := hex.EncodeToString(hasher.Sum(nil))
				expectedChecksum, found := checksums[binaryName]

				if found {
					if downloadedChecksum != expectedChecksum {
						return fmt.Errorf("checksum verification failed!\nExpected: %s\nGot: %s", expectedChecksum, downloadedChecksum)
					}
					fmt.Printf("✓ Checksum verified\n")
				} else {
					fmt.Printf("⚠️  Warning: No checksum found for %s\n", binaryName)
				}
			}

			// Get the path to the current executable
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			// Rename the current executable (as backup)
			backupPath := execPath + ".bak"
			if err := os.Rename(execPath, backupPath); err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}

			// Move the new executable into place
			if err := os.Rename(tmpFile.Name(), execPath); err != nil {
				// Try to restore backup if update fails
				os.Rename(backupPath, execPath)
				return fmt.Errorf("failed to install update: %w", err)
			}

			// Remove backup
			os.Remove(backupPath)

			fmt.Printf("✅ Successfully updated to version %s!\n", latestVersion)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force update even if already on latest version")
	return cmd
}

func getLatestRelease() (*githubRelease, error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// isUpdateAvailable compares two semantic version strings
// Returns true if latestVersion > currentVersion
func isUpdateAvailable(currentVersion, latestVersion string) (bool, error) {
	// Skip comparison for dev version
	if currentVersion == "dev" {
		return true, nil // Always update from dev version
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

// downloadChecksums downloads and parses the checksums.txt file from the release
// Returns a map of filename -> checksum
func downloadChecksums(version string) (map[string]string, error) {
	checksumURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/checksums.txt", owner, repo, version)

	resp, err := http.Get(checksumURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums file not found (status %d)", resp.StatusCode)
	}

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(resp.Body)

	// Parse checksums file format: "<checksum>  <filename>"
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			checksum := parts[0]
			filename := parts[1]
			checksums[filename] = checksum
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return checksums, nil
}
