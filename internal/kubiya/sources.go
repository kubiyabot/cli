package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// Source-related client methods
// All source-related operations should be defined here to maintain a single source of truth

// ListSources retrieves all available sources
func (c *Client) ListSources(ctx context.Context) ([]Source, error) {
	resp, err := c.get(ctx, "/sources")
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	defer resp.Body.Close()

	var sources []Source
	if err := json.NewDecoder(resp.Body).Decode(&sources); err != nil {
		return nil, fmt.Errorf("failed to decode sources response: %w", err)
	}

	return sources, nil
}

// GetSourceMetadata retrieves metadata for a specific source
func (c *Client) GetSourceMetadata(ctx context.Context, uuid string) (*Source, error) {
	// First get the source metadata
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/sources/%s/metadata", c.baseURL, uuid), nil)
	if err != nil {
		return nil, err
	}

	var metadata struct {
		UUID       string `json:"uuid"`
		SourceUUID string `json:"source_uuid"`
		Tools      []Tool `json:"tools"`
		Errors     []struct {
			File    string `json:"file"`
			Type    string `json:"type"`
			Error   string `json:"error"`
			Details string `json:"details"`
		} `json:"errors"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	if err := c.do(req, &metadata); err != nil {
		if c.debug {
			fmt.Printf("Error fetching source metadata: %v\n", err)
		}
		return nil, fmt.Errorf("failed to fetch source metadata: %w", err)
	}

	// Get the original source to merge with metadata
	source, err := c.GetSource(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Merge tools from metadata into source
	source.Tools = metadata.Tools

	if c.debug {
		fmt.Printf("Source UUID: %s\n", uuid)
		fmt.Printf("Number of tools found: %d\n", len(source.Tools))
		for i, tool := range source.Tools {
			fmt.Printf("Tool %d: %s (%s)\n", i+1, tool.Name, tool.Description)
			fmt.Printf("  Source: %s\n", tool.Source.URL)
			fmt.Printf("  Args: %d, Env: %d, Secrets: %d\n",
				len(tool.Args), len(tool.Env), len(tool.Secrets))
		}
		if len(metadata.Errors) > 0 {
			fmt.Printf("\nErrors found:\n")
			for _, err := range metadata.Errors {
				fmt.Printf("- %s: %s (%s)\n", err.File, err.Error, err.Type)
			}
		}
	}

	return source, nil
}

// DeleteSource removes a source by its UUID
func (c *Client) DeleteSource(ctx context.Context, sourceUUID string, runnerName string) error {
	url := fmt.Sprintf("/sources/%s", sourceUUID)
	if runnerName != "" {
		url = url + "?runner=" + runnerName
	}
	resp, err := c.delete(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// LoadSource loads a source from a local path
func (c *Client) LoadSource(ctx context.Context, path string) (*Source, error) {
	if c.debug {
		fmt.Printf("Loading source from path: %s\n", path)
	}

	// Handle local paths differently
	if path == "." || !strings.Contains(path, "://") {
		gitURL, branch, err := getGitInfo(path)
		if err != nil {
			return nil, fmt.Errorf("failed to get git info: %w", err)
		}

		if c.debug {
			fmt.Printf("Git URL: %s, Branch: %s\n", gitURL, branch)
		}

		// Convert git URL to HTTPS URL if needed
		if strings.HasPrefix(gitURL, "git@") {
			gitURL = convertGitToHTTPS(gitURL)
		}

		sources, err := c.ListSources(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list sources: %w", err)
		}

		// First try to find an exact match with branch and tools
		exactBranchPath := fmt.Sprintf("%s/tree/%s", gitURL, branch)
		var exactMatch *Source

		for _, s := range sources {
			if s.URL == exactBranchPath {
				// Get metadata to check for tools
				metadata, err := c.GetSourceMetadata(ctx, s.UUID)
				if err != nil {
					continue
				}

				if len(metadata.Tools) > 0 {
					if c.debug {
						fmt.Printf("Found exact branch match with tools: %s (%s)\n", s.UUID, s.URL)
					}
					return metadata, nil
				}
				exactMatch = metadata
			}
		}

		// If we found an exact match but without tools, keep looking for sources with tools
		var sourcesWithTools []*Source
		for _, s := range sources {
			if strings.Contains(s.URL, gitURL) {
				metadata, err := c.GetSourceMetadata(ctx, s.UUID)
				if err != nil {
					continue
				}
				if len(metadata.Tools) > 0 {
					sourcesWithTools = append(sourcesWithTools, metadata)
				}
			}
		}

		if len(sourcesWithTools) > 0 {
			// Prefer sources with tools
			if c.debug {
				fmt.Printf("Found %d sources with tools for repository %s\n", len(sourcesWithTools), gitURL)
				for _, s := range sourcesWithTools {
					fmt.Printf("- %s (%s) with %d tools\n", s.UUID, s.URL, len(s.Tools))
				}
			}
			return sourcesWithTools[0], nil
		}

		// If we found an exact match earlier, use it
		if exactMatch != nil {
			if c.debug {
				fmt.Printf("Using exact branch match: %s (%s)\n", exactMatch.UUID, exactMatch.URL)
			}
			return exactMatch, nil
		}

		// Create new source with the specific branch
		path = exactBranchPath
	}

	// Create new source if none found
	source, err := c.CreateSource(ctx, path)
	if err != nil {
		return nil, err
	}

	return c.GetSourceMetadata(ctx, source.UUID)
}

// Internal helper functions
func getGitInfo(path string) (string, string, error) {
	// Get the git remote URL
	cmd := exec.Command("git", "-C", path, "config", "--get", "remote.origin.url")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git remote: %w", err)
	}
	gitURL := strings.TrimSpace(string(out))

	// Get the current branch
	cmd = exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	out, err = cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))

	return gitURL, branch, nil
}

func convertGitToHTTPS(gitURL string) string {
	// Convert git@github.com:org/repo.git to https://github.com/org/repo
	gitURL = strings.TrimPrefix(gitURL, "git@")
	gitURL = strings.TrimSuffix(gitURL, ".git")
	gitURL = strings.Replace(gitURL, ":", "/", 1)
	return "https://" + gitURL
}

func normalizeGitHubURL(url string) string {
	// Convert github.com URLs to raw.githubusercontent.com
	if strings.Contains(url, "github.com") && !strings.Contains(url, "raw.githubusercontent.com") {
		// Replace github.com with raw.githubusercontent.com
		url = strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		// Remove "blob/" if present
		url = strings.Replace(url, "/blob/", "/", 1)
		// Remove "tree/" if present
		url = strings.Replace(url, "/tree/", "/", 1)
	}
	return url
}

// CreateSource creates a new source from a URL
func (c *Client) CreateSource(ctx context.Context, url string) (*Source, error) {
	// Create the source directly without pre-loading
	source := Source{
		URL: url,
	}

	data, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/sources", c.cfg.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	var created Source
	if err := c.do(req, &created); err != nil {
		return nil, err
	}

	return &created, nil
}

// GetSource retrieves a source by its UUID
func (c *Client) GetSource(ctx context.Context, uuid string) (*Source, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/sources/%s", c.baseURL, uuid), nil)
	if err != nil {
		return nil, err
	}

	var source Source
	if err := c.do(req, &source); err != nil {
		if c.debug {
			fmt.Printf("Error fetching source: %v\n", err)
		}
		return nil, fmt.Errorf("failed to fetch source: %w", err)
	}

	return &source, nil
}

// GetSourceByURL retrieves a source by its URL
func (c *Client) GetSourceByURL(ctx context.Context, url string) (*Source, error) {
	sources, err := c.ListSources(ctx)
	if err != nil {
		return nil, err
	}

	var matches []*Source
	for i, s := range sources {
		if strings.Contains(s.URL, url) {
			matches = append(matches, &sources[i])
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no sources found matching URL: %s", url)
	case 1:
		return matches[0], nil
	default:
		var details strings.Builder
		details.WriteString(fmt.Sprintf("Multiple sources found matching %s:\n", url))
		for _, s := range matches {
			details.WriteString(fmt.Sprintf("- %s (%s)\n", s.UUID, s.URL))
		}
		return nil, fmt.Errorf(details.String())
	}
}

// GetSourceMetadataCached retrieves source metadata with caching
func (c *Client) GetSourceMetadataCached(ctx context.Context, sourceUUID string) (*Source, error) {
	// Try to get from cache first
	if cached, ok := c.cache.Get(sourceUUID); ok {
		if source, ok := cached.(*Source); ok {
			return source, nil
		}
	}

	// If not in cache, fetch from API
	source, err := c.GetSourceMetadata(ctx, sourceUUID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.cache.Set(sourceUUID, source)
	return source, nil
}
