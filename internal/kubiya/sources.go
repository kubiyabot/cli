package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Source-related client methods
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

func (c *Client) GetSourceMetadata(ctx context.Context, uuid string) (*Source, error) {
	// First get the source to get its URL
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/sources/%s", c.baseURL, uuid), nil)
	if err != nil {
		return nil, err
	}

	var source Source
	if err := c.do(req, &source); err != nil {
		return nil, err
	}

	// Now load the source to get tools
	loadURL := fmt.Sprintf("%s/sources/load?url=%s", c.baseURL, url.QueryEscape(source.URL))
	loadReq, err := http.NewRequestWithContext(ctx, "GET", loadURL, nil)
	if err != nil {
		return nil, err
	}

	var loadedSource Source
	if err := c.do(loadReq, &loadedSource); err != nil {
		return nil, err
	}

	// Merge the loaded source data with the original source
	source.Tools = loadedSource.Tools

	if c.debug {
		fmt.Printf("Source URL: %s\n", source.URL)
		fmt.Printf("Number of tools: %d\n", len(source.Tools))
	}

	return &source, nil
}

func (c *Client) DeleteSource(ctx context.Context, sourceUUID string) error {
	resp, err := c.delete(ctx, fmt.Sprintf("/sources/%s", sourceUUID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Helper method for DELETE requests
func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp, nil
}

// SyncSource triggers a sync for a specific source
func (c *Client) SyncSource(ctx context.Context, sourceUUID string) (*Source, error) {
	// We use PUT to trigger a sync
	req, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/sources/%s", c.cfg.BaseURL, sourceUUID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var source Source
	if err := json.NewDecoder(resp.Body).Decode(&source); err != nil {
		return nil, err
	}

	return &source, nil
}

// LoadSource loads a source from a URL or local path
func (c *Client) LoadSource(ctx context.Context, url string) (*Source, error) {
	reqURL := fmt.Sprintf("%s/sources/load?url=%s", c.baseURL, url)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var source Source
	if err := c.do(req, &source); err != nil {
		return nil, err
	}

	// Add debug logging
	if c.debug {
		fmt.Printf("Load source response: %+v\n", source)
		fmt.Printf("Number of tools: %d\n", len(source.Tools))
	}

	return &source, nil
}

// CreateSource creates a new source from a URL
func (c *Client) CreateSource(ctx context.Context, url string) (*Source, error) {
	// First load the source to validate it
	metadata, err := c.LoadSource(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to load source: %w", err)
	}

	// Create the source
	source := Source{
		URL:  url,
		Name: metadata.Name,
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var created Source
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, err
	}

	return &created, nil
}
