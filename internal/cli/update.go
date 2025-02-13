package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

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
			if !force && latestVersion <= currentVersion {
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

			// Copy the downloaded binary to temp file
			if _, err := io.Copy(tmpFile, resp.Body); err != nil {
				return fmt.Errorf("failed to save update: %w", err)
			}
			tmpFile.Close()

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
