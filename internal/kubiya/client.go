package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	return &Client{
		cfg:     cfg,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: cfg.BaseURL,
		debug:   cfg.Debug,
		cache:   NewCache(5 * time.Minute),
	}
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
	resp, err := c.get(ctx, "/agents")
	if err != nil {
		if c.debug {
			fmt.Printf("Error getting teammates: %v\n", err)
			fmt.Printf("BaseURL: %s\n", c.baseURL)
			fmt.Printf("Full URL: %s/agents\n", c.baseURL)
			fmt.Printf("API Key present: %v\n", c.cfg.APIKey != "")
		}
		return nil, fmt.Errorf("failed to get teammates: %w", err)
	}
	defer resp.Body.Close()

	if c.debug {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Headers: %v\n", resp.Header)
		fmt.Printf("Response Body: %s\n", string(body))
		resp.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	var teammates []Teammate
	if err := json.NewDecoder(resp.Body).Decode(&teammates); err != nil {
		if c.debug {
			fmt.Printf("Error decoding response: %v\n", err)
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter out empty entries
	var validTeammates []Teammate
	for _, t := range teammates {
		if t.UUID != "" && t.Name != "" {
			validTeammates = append(validTeammates, t)
		}
	}

	return validTeammates, nil
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
	url := fmt.Sprintf("%s/%s", c.baseURL, path)

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

func (c *Client) GetRunner(ctx context.Context, name string) (Runner, error) {
	// Call the runners describe endpoint directly
	resp, err := c.get(ctx, fmt.Sprintf("/runners/%s/describe", name))
	if err != nil {
		return Runner{}, fmt.Errorf("failed to get runner details: %w", err)
	}
	defer resp.Body.Close()

	var runner Runner
	if err := json.NewDecoder(resp.Body).Decode(&runner); err != nil {
		return Runner{}, fmt.Errorf("failed to decode runner response: %w", err)
	}

	// Convert version if it's a number
	if runner.Version == "0" {
		runner.Version = "v1"
	} else if runner.Version == "2" {
		runner.Version = "v2"
	}

	return runner, nil
}

// Add this method to list all runners
func (c *Client) ListRunners(ctx context.Context) ([]Runner, error) {
	resp, err := c.get(ctx, "/runners")
	if err != nil {
		return nil, fmt.Errorf("failed to list runners: %w", err)
	}
	defer resp.Body.Close()

	var runners []Runner
	if err := json.NewDecoder(resp.Body).Decode(&runners); err != nil {
		return nil, fmt.Errorf("failed to decode runners response: %w", err)
	}

	// Convert version numbers to strings for consistency
	for i := range runners {
		if runners[i].Version == "0" {
			runners[i].Version = "v1"
		} else if runners[i].Version == "2" {
			runners[i].Version = "v2"
		}
	}

	return runners, nil
}

// Add this method to get runner manifest
func (c *Client) GetRunnerManifest(ctx context.Context, name string) (RunnerManifest, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/runners/%s/manifest", name))
	if err != nil {
		return RunnerManifest{}, fmt.Errorf("failed to get runner manifest: %w", err)
	}
	defer resp.Body.Close()

	var manifest RunnerManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return RunnerManifest{}, fmt.Errorf("failed to decode manifest response: %w", err)
	}

	return manifest, nil
}

// Add this method to download manifest content
func (c *Client) DownloadManifest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest content: %w", err)
	}

	return content, nil
}

func (c *Client) CreateTeammate(ctx context.Context, teammate Teammate) (*Teammate, error) {
	resp, err := c.post(ctx, "/agents", teammate)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var created Teammate
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, err
	}
	return &created, nil
}

func (c *Client) UpdateTeammate(ctx context.Context, uuid string, teammate Teammate) (*Teammate, error) {
	resp, err := c.put(ctx, fmt.Sprintf("/agents/%s", uuid), teammate)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var updated Teammate
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (c *Client) DeleteTeammate(ctx context.Context, uuid string) error {
	resp, err := c.delete(ctx, fmt.Sprintf("/agents/%s", uuid))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) BindSourceToTeammate(ctx context.Context, sourceUUID, teammateUUID string) error {
	path := fmt.Sprintf("sources/%s/teammates/%s", sourceUUID, teammateUUID)
	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to bind source to teammate: %s", resp.Status)
	}

	return nil
}

func (c *Client) GetSourceTeammates(ctx context.Context, sourceUUID string) ([]Teammate, error) {
	// Get all teammates first
	teammates, err := c.GetTeammates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list teammates: %w", err)
	}

	// Get source details for better error handling
	source, err := c.GetSourceMetadata(ctx, sourceUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source details: %w", err)
	}

	// Filter teammates that have this source
	var connectedTeammates []Teammate
	for _, teammate := range teammates {
		// Check if teammate has access to this source
		hasSource := false
		for _, teammateSrc := range teammate.Sources {
			if teammateSrc == sourceUUID {
				hasSource = true
				break
			}
		}
		if hasSource {
			// Add source details to teammate for context
			teammate.Sources = append(teammate.Sources, *&source.UUID)
			connectedTeammates = append(connectedTeammates, teammate)
		}
	}

	if c.debug {
		fmt.Printf("Found %d teammates connected to source %s\n", len(connectedTeammates), sourceUUID)
		for _, t := range connectedTeammates {
			fmt.Printf("- %s (UUID: %s)\n", t.Name, t.UUID)
		}
	}

	return connectedTeammates, nil
}

// Add a helper method to check if a teammate exists
func (c *Client) TeammateExists(ctx context.Context, nameOrID string) (*Teammate, error) {
	teammates, err := c.GetTeammates(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range teammates {
		if t.UUID == nameOrID || t.Name == nameOrID {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("teammate not found: %s", nameOrID)
}
