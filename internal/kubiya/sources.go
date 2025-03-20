package kubiya

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
			if sources[i].URL == "" || len(sources[i].InlineTools) > 0 || strings.HasSuffix(sources[i].URL, ".zip") {
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
		// Check if this is a directory or a Python file
		fileInfo, err := os.Stat(path)
		if err == nil && (fileInfo.IsDir() || (filepath.Ext(path) == ".py")) {
			// It's a directory or Python file, need to zip it
			if c.debug {
				fmt.Printf("Path is a directory or Python file, creating zip: %s\n", path)
			}

			// Create a temporary zip file
			tempZipFile, err := createTempZip(path)
			if err != nil {
				return nil, fmt.Errorf("failed to create zip file: %w", err)
			}
			defer os.Remove(tempZipFile) // Clean up the temp file after use

			// Upload the zip file
			return c.LoadZipSource(ctx, tempZipFile, source.Name, runnerName)
		}

		// Regular Git repository handling
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

	// Check if the URL is a local path and it's a directory or Python file
	if url != "" && (url == "." || !strings.Contains(url, "://")) {
		fileInfo, err := os.Stat(url)
		if err == nil && (fileInfo.IsDir() || (filepath.Ext(url) == ".py")) {
			if c.debug {
				fmt.Printf("URL is a directory or Python file, creating as zip source: %s\n", url)
			}

			// Create a temporary zip file
			tempZipFile, err := createTempZip(url)
			if err != nil {
				return nil, fmt.Errorf("failed to create zip file: %w", err)
			}
			defer os.Remove(tempZipFile) // Clean up the temp file after use

			// Upload the zip file instead
			return c.CreateZipSource(ctx, tempZipFile, source.Name, source.Runner)
		}
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

// CreateZipSource creates a new source from a zip file
// This is useful for uploading inline sources as a folder or Python file(s)
func (c *Client) CreateZipSource(ctx context.Context, zipPath string, name string, runnerName string) (*Source, error) {
	if c.debug {
		fmt.Printf("Creating zip source from path: %s\n", zipPath)
	}

	// Open the zip file
	file, err := os.Open(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close()

	// Prepare the multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create a form file
	part, err := writer.CreateFormFile("file", filepath.Base(zipPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy the file content to the form
	if _, err = io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy zip content: %w", err)
	}

	// Close the writer
	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build the URL with query parameters
	urlPath := "/sources/zip"
	if runnerName != "" {
		urlPath += fmt.Sprintf("?runner=%s", url.QueryEscape(runnerName))
	}
	if name != "" {
		if strings.Contains(urlPath, "?") {
			urlPath += fmt.Sprintf("&name=%s", url.QueryEscape(name))
		} else {
			urlPath += fmt.Sprintf("?name=%s", url.QueryEscape(name))
		}
	}

	// Create a new HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s%s", c.baseURL, urlPath), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set the content type
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	// Make the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var source Source
	if err := json.NewDecoder(resp.Body).Decode(&source); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Ensure type is set correctly
	if source.Type == "" {
		source.Type = "inline"
	}

	return &source, nil
}

// LoadZipSource loads a source from a zip file without creating it permanently
// This is useful for testing inline sources as a folder or Python file(s)
func (c *Client) LoadZipSource(ctx context.Context, zipPath string, name string, runnerName string) (*Source, error) {
	if c.debug {
		fmt.Printf("Loading zip source from path: %s\n", zipPath)
	}

	// Open the zip file
	file, err := os.Open(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close()

	// Prepare the multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create a form file
	part, err := writer.CreateFormFile("file", filepath.Base(zipPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy the file content to the form
	if _, err = io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy zip content: %w", err)
	}

	// Close the writer
	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build the URL with query parameters
	urlPath := "/sources/zip/load"
	if runnerName != "" {
		urlPath += fmt.Sprintf("?runner=%s", url.QueryEscape(runnerName))
	}
	if name != "" {
		if strings.Contains(urlPath, "?") {
			urlPath += fmt.Sprintf("&name=%s", url.QueryEscape(name))
		} else {
			urlPath += fmt.Sprintf("?name=%s", url.QueryEscape(name))
		}
	}

	// Create a new HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s%s", c.baseURL, urlPath), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set the content type
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	// Make the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var source Source
	if err := json.NewDecoder(resp.Body).Decode(&source); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Ensure type is set correctly
	if source.Type == "" {
		source.Type = "inline"
	}

	return &source, nil
}

// Helper function to create a temporary zip file from a directory or file
func createTempZip(srcPath string) (string, error) {
	// Create a temporary file for the zip
	tempFile, err := os.CreateTemp("", "kubiya-source-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFile.Close()

	// Get the absolute path of the source
	srcPath, err = filepath.Abs(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create a new zip file
	zipFile, err := os.Create(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Check if source path is a file or directory
	fileInfo, err := os.Stat(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat source path: %w", err)
	}

	if fileInfo.IsDir() {
		// Check if the directory is a Python project
		isPythonProject := isPythonProject(srcPath)

		// Walk through the directory
		err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Create a relative path for the file within the zip
			relPath, err := filepath.Rel(srcPath, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path: %w", err)
			}

			// Skip hidden files and directories
			if strings.HasPrefix(relPath, ".") || strings.Contains(relPath, "/.") {
				return nil
			}

			// For Python projects, filter only Python-related files
			if isPythonProject {
				if !isPythonFile(relPath) && !isPythonProjectFile(relPath) {
					return nil
				}
			}

			// Create a new file in the zip
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			// Create a header for the file
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return fmt.Errorf("failed to create file header: %w", err)
			}

			// Set the name in the header to the relative path
			header.Name = relPath
			header.Method = zip.Deflate

			// Create a writer for the file in the zip
			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return fmt.Errorf("failed to create file in zip: %w", err)
			}

			// Copy the file contents to the zip
			_, err = io.Copy(writer, file)
			if err != nil {
				return fmt.Errorf("failed to write file to zip: %w", err)
			}

			return nil
		})

		if err != nil {
			return "", fmt.Errorf("failed to add files to zip: %w", err)
		}
	} else {
		// It's a single file, add it directly
		file, err := os.Open(srcPath)
		if err != nil {
			return "", fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// Create a header for the file
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return "", fmt.Errorf("failed to create file header: %w", err)
		}

		// Use the file name as the zip entry name
		header.Name = filepath.Base(srcPath)
		header.Method = zip.Deflate

		// Create a writer for the file in the zip
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return "", fmt.Errorf("failed to create file in zip: %w", err)
		}

		// Copy the file contents to the zip
		_, err = io.Copy(writer, file)
		if err != nil {
			return "", fmt.Errorf("failed to write file to zip: %w", err)
		}
	}

	return tempFile.Name(), nil
}

// isPythonProject checks if a directory is a Python project
func isPythonProject(dirPath string) bool {
	// Common Python project files
	pythonProjectFiles := []string{
		"setup.py",
		"requirements.txt",
		"pyproject.toml",
		"setup.cfg",
		"Pipfile",
		"poetry.lock",
		"__init__.py",
	}

	// Check for presence of any of the Python project files
	for _, file := range pythonProjectFiles {
		if _, err := os.Stat(filepath.Join(dirPath, file)); err == nil {
			return true
		}
	}

	// Count Python files
	count := 0
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".py" {
			count++
		}
		return nil
	})

	// If more than 2 Python files found, consider it a Python project
	return count > 2
}

// isPythonFile checks if a file is a Python file
func isPythonFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".py" || ext == ".pyi" || ext == ".pyx" || ext == ".pxd"
}

// isPythonProjectFile checks if a file is a Python project file
func isPythonProjectFile(path string) bool {
	// Common Python project file patterns
	pythonProjectFiles := []string{
		"requirements.txt",
		"setup.py",
		"setup.cfg",
		"pyproject.toml",
		"Pipfile",
		"Pipfile.lock",
		"poetry.lock",
		"tox.ini",
		".flake8",
		"pytest.ini",
		"conftest.py",
		"__init__.py",
		"README.md",
		"README.rst",
		"LICENSE",
	}

	filename := filepath.Base(path)
	for _, projectFile := range pythonProjectFiles {
		if filename == projectFile {
			return true
		}
	}

	// Also include YAML and JSON files as they might be config files
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}
