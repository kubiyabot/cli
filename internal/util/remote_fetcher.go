package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IsRemoteSource checks if the given source is a remote URL or git repository
func IsRemoteSource(source string) bool {
	// HTTP(S) URLs
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return true
	}

	// Git URLs
	if strings.HasPrefix(source, "git@") || strings.HasPrefix(source, "git+") {
		return true
	}

	// GitHub shorthand: user/repo//path or user/repo@ref//path
	// Check for // which distinguishes repo from file path
	if strings.Contains(source, "//") {
		return true
	}

	return false
}

// FetchRemoteContent detects the source type and fetches content accordingly
func FetchRemoteContent(ctx context.Context, source string) ([]byte, error) {
	// GitHub shorthand: user/repo//path or user/repo@ref//path
	if strings.Contains(source, "//") && !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		return fetchFromGitHubShorthand(ctx, source)
	}

	// Git clone URLs: git@github.com:user/repo.git or https://github.com/user/repo.git
	if strings.HasPrefix(source, "git@") || strings.HasPrefix(source, "git+") {
		return fetchFromGitCloneURL(ctx, source)
	}

	// HTTP(S) URLs
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return FetchHTTP(ctx, source)
	}

	return nil, fmt.Errorf("unsupported source format: %s", source)
}

// FetchHTTP fetches content from an HTTP(S) URL
func FetchHTTP(ctx context.Context, urlStr string) ([]byte, error) {
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitHub token auth if URL is from GitHub
	if strings.Contains(urlStr, "github.com") || strings.Contains(urlStr, "githubusercontent.com") {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
		}
	}

	// Perform request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
			return nil, fmt.Errorf("timeout fetching remote plan (network issue or slow connection)")
		}
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("authentication failed (HTTP %d) - set GITHUB_TOKEN environment variable for private repos", resp.StatusCode)
		case http.StatusNotFound:
			return nil, fmt.Errorf("plan file not found at URL (HTTP 404)")
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
			return nil, fmt.Errorf("remote server error (HTTP %d)", resp.StatusCode)
		default:
			return nil, fmt.Errorf("unexpected HTTP status: %d %s", resp.StatusCode, resp.Status)
		}
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// fetchFromGitHubShorthand handles GitHub shorthand format: user/repo//path or user/repo@ref//path
func fetchFromGitHubShorthand(ctx context.Context, source string) ([]byte, error) {
	repoURL, filePath, ref, err := parseGitHubShorthand(source)
	if err != nil {
		return nil, err
	}

	return fetchFromGitRepo(ctx, repoURL, filePath, ref)
}

// fetchFromGitCloneURL handles git clone URLs with file paths
func fetchFromGitCloneURL(ctx context.Context, source string) ([]byte, error) {
	// Parse git clone URL format: repo.git//path or git@host:repo.git//path
	parts := strings.Split(source, "//")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid git URL format (expected: <repo-url>//path/to/file)")
	}

	repoURL := parts[0]
	filePath := parts[1]

	// Extract ref if specified: repo.git@branch//path
	ref := "main" // default
	if strings.Contains(repoURL, "@") && !strings.HasPrefix(repoURL, "git@") {
		// Split by @ but not for git@github.com style URLs
		atIndex := strings.LastIndex(repoURL, "@")
		ref = repoURL[atIndex+1:]
		repoURL = repoURL[:atIndex]
	}

	return fetchFromGitRepo(ctx, repoURL, filePath, ref)
}

// parseGitHubShorthand parses GitHub shorthand into components
// Input: "user/repo//path/file.json" or "user/repo@branch//path/file.json"
// Output: repoURL, filePath, ref
func parseGitHubShorthand(source string) (repoURL, filePath, ref string, err error) {
	parts := strings.Split(source, "//")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid GitHub shorthand format (expected: user/repo//path or user/repo@ref//path)")
	}

	repoSpec := parts[0]
	filePath = parts[1]

	// Check for ref: user/repo@branch
	if strings.Contains(repoSpec, "@") {
		repoParts := strings.SplitN(repoSpec, "@", 2)
		repoSpec = repoParts[0]
		ref = repoParts[1]
	} else {
		ref = "main" // default branch
	}

	// Construct GitHub HTTPS URL
	repoURL = fmt.Sprintf("https://github.com/%s.git", repoSpec)
	return repoURL, filePath, ref, nil
}

// fetchFromGitRepo clones a git repository and extracts the specified file
func fetchFromGitRepo(ctx context.Context, repoURL, filePath, ref string) ([]byte, error) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "kubiya-plan-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Prepare git clone command
	// Use --depth 1 for shallow clone to improve performance
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", ref, "--single-branch", repoURL, tmpDir)

	// Set environment to use credentials if available
	cmd.Env = append(os.Environ(), prepareGitEnv()...)

	// Execute clone
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// Provide helpful error messages
		if strings.Contains(outputStr, "authentication") || strings.Contains(outputStr, "Permission denied") {
			return nil, fmt.Errorf("git authentication failed - check SSH keys or set GITHUB_TOKEN environment variable")
		}
		if strings.Contains(outputStr, "Repository not found") || strings.Contains(outputStr, "not found") {
			return nil, fmt.Errorf("repository not found - check URL or permissions: %s", repoURL)
		}
		if strings.Contains(outputStr, "couldn't find remote ref") {
			return nil, fmt.Errorf("branch/tag '%s' not found in repository", ref)
		}
		return nil, fmt.Errorf("git clone failed: %s", outputStr)
	}

	// Read the requested file
	fullPath := filepath.Join(tmpDir, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file '%s' not found in repository", filePath)
		}
		return nil, fmt.Errorf("failed to read file from repository: %w", err)
	}

	return data, nil
}

// prepareGitEnv prepares environment variables for git authentication
func prepareGitEnv() []string {
	var envVars []string

	// If GITHUB_TOKEN is set, configure git to use it for HTTPS authentication
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		// Git credential helper will use this token
		envVars = append(envVars, fmt.Sprintf("GIT_ASKPASS=echo"))
		envVars = append(envVars, fmt.Sprintf("GIT_USERNAME=git"))
		envVars = append(envVars, fmt.Sprintf("GIT_PASSWORD=%s", token))
	}

	// Also support generic GIT_TOKEN
	if token := os.Getenv("GIT_TOKEN"); token != "" && os.Getenv("GITHUB_TOKEN") == "" {
		envVars = append(envVars, fmt.Sprintf("GIT_ASKPASS=echo"))
		envVars = append(envVars, fmt.Sprintf("GIT_USERNAME=git"))
		envVars = append(envVars, fmt.Sprintf("GIT_PASSWORD=%s", token))
	}

	return envVars
}
