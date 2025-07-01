package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	githubAPI           = "https://api.github.com/repos/kubiyabot/cli/releases/latest"
	downloadURLTemplate = "https://github.com/kubiyabot/cli/releases/download/%s/kubiya-cli-%s-%s"
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// EnsureKubiyaCLI ensures the Kubiya CLI is available and up to date
func EnsureKubiyaCLI(forceUpdate bool) (string, error) {
	// Check if KUBIYA_CLI_PATH is set
	if cliPath := os.Getenv("KUBIYA_CLI_PATH"); cliPath != "" {
		if _, err := os.Stat(cliPath); err == nil {
			logInfo(fmt.Sprintf("Using Kubiya CLI from KUBIYA_CLI_PATH: %s", cliPath))
			return cliPath, nil
		}
	}

	// Determine install location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	binDir := filepath.Join(homeDir, ".kubiya", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}

	kubiyaPath := filepath.Join(binDir, "kubiya")
	if runtime.GOOS == "windows" {
		kubiyaPath += ".exe"
	}

	// Check if binary exists and we're not forcing update
	if !forceUpdate {
		if _, err := os.Stat(kubiyaPath); err == nil {
			// Check if it's in PATH
			if _, err := exec.LookPath("kubiya"); err == nil {
				logInfo("Kubiya CLI found in PATH")
				return "kubiya", nil
			}
			logInfo(fmt.Sprintf("Using existing Kubiya CLI: %s", kubiyaPath))
			return kubiyaPath, nil
		}
	}

	// Get version to download
	version := os.Getenv("KUBIYA_CLI_VERSION")
	if version == "" {
		logInfo("Fetching latest Kubiya CLI version...")
		latestVersion, err := getLatestVersion()
		if err != nil {
			return "", fmt.Errorf("failed to get latest version: %w", err)
		}
		version = latestVersion
		logInfo(fmt.Sprintf("Latest version: %s", version))
	} else {
		logInfo(fmt.Sprintf("Using specified version: %s", version))
	}

	// Download the CLI
	logInfo(fmt.Sprintf("Downloading Kubiya CLI %s...", version))
	if err := downloadCLI(version, kubiyaPath); err != nil {
		return "", fmt.Errorf("failed to download CLI: %w", err)
	}

	// Make executable
	if err := os.Chmod(kubiyaPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make CLI executable: %w", err)
	}

	logInfo(fmt.Sprintf("✅ Kubiya CLI %s installed to %s", version, kubiyaPath))

	// Add to PATH for this session
	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, binDir) {
		os.Setenv("PATH", binDir+string(os.PathListSeparator)+currentPath)
	}

	return kubiyaPath, nil
}

// getLatestVersion fetches the latest version from GitHub API
func getLatestVersion() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", githubAPI, nil)
	if err != nil {
		return "", err
	}

	// GitHub API prefers this header
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag name found in release")
	}

	return release.TagName, nil
}

// downloadCLI downloads the CLI binary for the current platform
func downloadCLI(version, destination string) error {
	// Determine OS and architecture
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go OS/arch to download naming convention
	switch osName {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	case "windows":
		osName = "windows"
	default:
		return fmt.Errorf("unsupported OS: %s", osName)
	}

	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	case "386":
		arch = "386"
	default:
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Build download URL
	downloadURL := fmt.Sprintf(downloadURLTemplate, version, osName, arch)
	if osName == "windows" {
		downloadURL += ".exe"
	}

	logInfo(fmt.Sprintf("Downloading from: %s", downloadURL))

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// Download the file
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp(filepath.Dir(destination), "kubiya-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Copy download to temp file
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write download: %w", err)
	}
	tmpFile.Close()

	// Move temp file to final destination
	if err := os.Rename(tmpPath, destination); err != nil {
		// On Windows, we might need to remove existing file first
		if runtime.GOOS == "windows" {
			os.Remove(destination)
			if err := os.Rename(tmpPath, destination); err != nil {
				return fmt.Errorf("failed to install CLI: %w", err)
			}
		} else {
			return fmt.Errorf("failed to install CLI: %w", err)
		}
	}

	return nil
}

// CheckForUpdates checks if a newer version is available
func CheckForUpdates(currentVersion string) (bool, string, error) {
	latestVersion, err := getLatestVersion()
	if err != nil {
		return false, "", err
	}

	// Simple string comparison - assumes semantic versioning
	if latestVersion != currentVersion {
		return true, latestVersion, nil
	}

	return false, "", nil
}

// GetInstalledVersion gets the version of the installed CLI
func GetInstalledVersion(cliPath string) (string, error) {
	cmd := exec.Command(cliPath, "version", "--short")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// logInfo logs informational messages to stderr
func logInfo(message string) {
	log.SetOutput(os.Stderr)
	log.Printf("[INFO] %s\n", message)
}
