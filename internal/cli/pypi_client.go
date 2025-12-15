package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// PyPIPackageInfo represents the structure of PyPI JSON API response
type PyPIPackageInfo struct {
	Info struct {
		Version string `json:"version"`
		Name    string `json:"name"`
	} `json:"info"`
}

// Cache entry for PyPI version queries
type pypiCacheEntry struct {
	version   string
	timestamp time.Time
}

// pypiCache stores recent version queries to avoid hammering PyPI
var (
	pypiCache      = make(map[string]pypiCacheEntry)
	pypiCacheMutex sync.RWMutex
	pypiCacheTTL   = 5 * time.Minute
)

// GetLatestPackageVersion queries PyPI for the latest version of a package
// Uses a default timeout of 5 seconds for network operations
func GetLatestPackageVersion(packageName string) (string, error) {
	return GetLatestPackageVersionWithTimeout(packageName, 5*time.Second)
}

// GetLatestPackageVersionWithTimeout queries PyPI with a custom timeout
// Returns the latest version string or an error if the query fails
func GetLatestPackageVersionWithTimeout(packageName string, timeout time.Duration) (string, error) {
	// Check cache first
	pypiCacheMutex.RLock()
	if entry, exists := pypiCache[packageName]; exists {
		if time.Since(entry.timestamp) < pypiCacheTTL {
			pypiCacheMutex.RUnlock()
			return entry.version, nil
		}
	}
	pypiCacheMutex.RUnlock()

	// Create HTTP client with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query PyPI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PyPI returned status %d", resp.StatusCode)
	}

	var packageInfo PyPIPackageInfo
	if err := json.NewDecoder(resp.Body).Decode(&packageInfo); err != nil {
		return "", fmt.Errorf("failed to parse PyPI response: %w", err)
	}

	version := packageInfo.Info.Version
	if version == "" {
		return "", fmt.Errorf("no version information found in PyPI response")
	}

	// Store in cache
	pypiCacheMutex.Lock()
	pypiCache[packageName] = pypiCacheEntry{
		version:   version,
		timestamp: time.Now(),
	}
	pypiCacheMutex.Unlock()

	return version, nil
}

// checkPyPIAvailable performs a quick check to see if PyPI is reachable
// Returns true if PyPI responds within the timeout, false otherwise
func checkPyPIAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://pypi.org", nil)
	if err != nil {
		return false
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// ClearPyPICache clears the in-memory cache of PyPI version queries
func ClearPyPICache() {
	pypiCacheMutex.Lock()
	defer pypiCacheMutex.Unlock()
	pypiCache = make(map[string]pypiCacheEntry)
}
