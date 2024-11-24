package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/kubiyabot/cli/internal/config"
)

// Client represents a Kubiya API client
type Client struct {
	cfg     *config.Config
	client  *http.Client
	baseURL string
	debug   bool
	cache   *Cache
}

// NewClient creates a new Kubiya API client
func NewClient(cfg *config.Config) *Client {
	client := &Client{
		cfg:     cfg,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: cfg.BaseURL,
		debug:   cfg.Debug,
		cache:   NewCache(5 * time.Minute),
	}
	return client
}

// do performs an HTTP request and decodes the response into v
func (c *Client) do(req *http.Request, v interface{}) error {
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return err
		}
	}

	return nil
}

// newJSONRequest creates a new HTTP request with JSON payload
func (c *Client) newJSONRequest(ctx context.Context, method, url string, payload interface{}) (*http.Request, error) {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, fmt.Errorf("failed to encode request payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	return req, nil
}

// GetSourceMetadataCached retrieves the source metadata from the cache if available, otherwise fetches it from the API
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

// GetTeammates retrieves the list of teammates from the API
func (c *Client) GetTeammates(ctx context.Context) ([]Teammate, error) {
	resp, err := c.get(ctx, "/agents?mode=all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var agents []struct {
		UUID        string            `json:"uuid"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Environment map[string]string `json:"environment_variables"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}

	// Convert agents to teammates
	teammates := make([]Teammate, 0, len(agents))
	for _, agent := range agents {
		if agent.UUID != "" && agent.Name != "" { // Filter out empty entries
			teammates = append(teammates, Teammate{
				UUID:        agent.UUID,
				Name:        agent.Name,
				Desc:        agent.Description,
				Environment: agent.Environment,
			})
		}
	}

	return teammates, nil
}

// GetTeammateEnvVar retrieves the value of an environment variable for a teammate from the API
func (c *Client) GetTeammateEnvVar(ctx context.Context, teammateID, varName string) (string, error) {
	// First get the full teammate details to access environment variables
	resp, err := c.get(ctx, fmt.Sprintf("/agents/%s", teammateID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var agent struct {
		Environment map[string]string `json:"environment_variables"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return "", err
	}

	// Look for the specific environment variable
	if value, exists := agent.Environment[varName]; exists {
		return value, nil
	}
	return "", fmt.Errorf("environment variable %s not found for teammate", varName)
}

// Add these methods to the Client struct

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

// Add this helper method for POST requests
func (c *Client) post(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, fmt.Errorf("failed to encode payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
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

// Add these methods to handle secrets
func (c *Client) GetSecretValue(ctx context.Context, name string) (string, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/secrets/get_value/%s", name))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Value, nil
}

func (c *Client) UpdateSecret(ctx context.Context, name, value, description string) error {
	payload := struct {
		Value       string `json:"value"`
		Description string `json:"description,omitempty"`
	}{
		Value:       value,
		Description: description,
	}

	resp, err := c.put(ctx, fmt.Sprintf("/secrets/%s", name), payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) put(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, fmt.Errorf("failed to encode payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, &body)
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

func (c *Client) GetSecret(ctx context.Context, name string) (*Secret, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/secrets/%s", name))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var secret Secret
	if err := json.NewDecoder(resp.Body).Decode(&secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func (c *Client) CreateSecret(ctx context.Context, name, value, description string) error {
	payload := struct {
		Value       string `json:"value"`
		Description string `json:"description,omitempty"`
	}{
		Value:       value,
		Description: description,
	}

	resp, err := c.post(ctx, "/secrets", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Add this method to the Client type
func (c *Client) ListTools(ctx context.Context, sourceURL string) ([]Tool, error) {
	// First load the source to get its tools
	loadURL := fmt.Sprintf("/sources/load?url=%s", url.QueryEscape(sourceURL))

	resp, err := c.get(ctx, loadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load source tools: %w", err)
	}
	defer resp.Body.Close()

	var source Source
	if err := json.NewDecoder(resp.Body).Decode(&source); err != nil {
		return nil, fmt.Errorf("failed to decode source response: %w", err)
	}

	if c.debug {
		fmt.Printf("Source URL: %s\n", sourceURL)
		fmt.Printf("Number of tools: %d\n", len(source.Tools))
	}

	return source.Tools, nil
}
