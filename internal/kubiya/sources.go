package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	// Make sure we have correct type information
	for i := range sources {
		// Set default type if not available
		if sources[i].Type == "" {
			if sources[i].URL == "" || len(sources[i].InlineTools) > 0 {
				sources[i].Type = "inline"
			} else {
				sources[i].Type = "git"
			}
		}

		// Debug logging
		if c.debug {
			fmt.Printf("Source %s: Type=%s, URL=%s, Tools=%d, InlineTools=%d\n",
				sources[i].UUID,
				sources[i].Type,
				sources[i].URL,
				len(sources[i].Tools),
				len(sources[i].InlineTools))
		}
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

		// Also log inline tools if available
		if len(source.InlineTools) > 0 {
			fmt.Printf("Inline tools found: %d\n", len(source.InlineTools))
			for i, tool := range source.InlineTools {
				fmt.Printf("Inline Tool %d: %s (%s)\n", i+1, tool.Name, tool.Description)
			}
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

// LoadSource loads a source from a local path or from inline tools
func (c *Client) LoadSource(ctx context.Context, path string, options ...SourceOption) (*Source, error) {
	if c.debug {
		fmt.Printf("Loading source from path: %s\n", path)
	}

	// Check if this is an inline source request
	var source Source
	var inlineTools []Tool
	var runnerName string

	// Extract options before processing
	for _, option := range options {
		option(&source)
	}

	inlineTools = source.InlineTools
	runnerName = source.Runner

	// For inline sources (empty path or explicitly marked as inline)
	if path == "" || source.Type == "inline" {
		// Always mark as inline type if URL is empty
		source.Type = "inline"

		if len(inlineTools) > 0 {
			if c.debug {
				fmt.Printf("Loading inline source with %d tools\n", len(inlineTools))
			}

			// Use discovery API to validate the inline tools first
			discovered, err := c.DiscoverSource(ctx, "", source.DynamicConfig, runnerName, inlineTools)
			if err != nil {
				return nil, fmt.Errorf("failed to validate inline tools: %w", err)
			}

			// Return the discovered source
			return &Source{
				Name:          source.Name,
				Type:          "inline",
				InlineTools:   discovered.Tools,
				DynamicConfig: source.DynamicConfig,
				Runner:        runnerName,
			}, nil
		}
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
	source = Source{
		URL:           path,
		Type:          "git",
		Name:          source.Name,
		DynamicConfig: source.DynamicConfig,
		Runner:        runnerName,
	}

	// Discover the source before creating
	discovered, err := c.DiscoverSource(ctx, path, source.DynamicConfig, runnerName, nil)
	if err != nil {
		return nil, err
	}

	// Apply discovered information
	source.Tools = discovered.Tools

	// Create the source
	created, err := c.CreateSource(ctx, path,
		WithName(source.Name),
		WithDynamicConfig(source.DynamicConfig),
		WithRunner(runnerName))
	if err != nil {
		return nil, err
	}

	return c.GetSourceMetadata(ctx, created.UUID)
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

// CreateSource creates a new source from a URL or with inline tools
func (c *Client) CreateSource(ctx context.Context, url string, opts ...SourceOption) (*Source, error) {
	// Create the source with defaults
	source := Source{
		URL: url,
	}

	// If URL is empty, default to inline type
	if url == "" {
		source.Type = "inline"
	} else {
		source.Type = "git" // Explicitly set type for git sources
	}

	// Apply options
	for _, opt := range opts {
		opt(&source)
	}

	// Ensure type is set based on content
	if len(source.InlineTools) > 0 {
		source.Type = "inline"
	}

	if c.debug {
		fmt.Printf("Creating source: %+v\n", source)
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

// SourceOption is a functional option for configuring a source
type SourceOption func(*Source)

// WithName sets the name of a source
func WithName(name string) SourceOption {
	return func(s *Source) {
		s.Name = name
	}
}

// WithInlineTools sets inline tools for a source
func WithInlineTools(tools []Tool) SourceOption {
	return func(s *Source) {
		s.InlineTools = tools
		s.Type = "inline"
	}
}

// WithDynamicConfig sets the dynamic configuration for a source
func WithDynamicConfig(config map[string]interface{}) SourceOption {
	return func(s *Source) {
		s.DynamicConfig = config
	}
}

// WithRunner sets the runner for a source
func WithRunner(runner string) SourceOption {
	return func(s *Source) {
		s.Runner = runner
	}
}

// UpdateSource updates an existing source
func (c *Client) UpdateSource(ctx context.Context, uuid string, opts ...SourceOption) (*Source, error) {
	// First get the existing source
	source, err := c.GetSource(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Apply options
	for _, opt := range opts {
		opt(source)
	}

	// Ensure type is set correctly based on content
	if len(source.InlineTools) > 0 {
		source.Type = "inline"
	} else if source.URL == "" {
		source.Type = "inline"
	} else if source.Type == "" {
		source.Type = "git"
	}

	data, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/sources/%s", c.cfg.BaseURL, uuid), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	var updated Source
	if err := c.do(req, &updated); err != nil {
		return nil, err
	}

	return &updated, nil
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

// GetRawSourceMetadata retrieves metadata for a source as a raw JSON string
// This is useful for debugging when there are parsing errors
func (c *Client) GetRawSourceMetadata(ctx context.Context, uuid string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/sources/%s/metadata", c.baseURL, uuid), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body as raw string
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}
