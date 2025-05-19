package kubiya

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kubiyabot/cli/internal/config"
)

// fileState tracks the accumulation of one file's content during SSE streaming.
type fileState struct {
	buffer *strings.Builder
	size   int
}

// Client represents a Kubiya API client
type Client struct {
	cfg     *config.Config
	client  *http.Client
	baseURL string
	debug   bool
	cache   *Cache
	audit   *AuditClient
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
	client.audit = NewAuditClient(client)
	return client
}

// Audit returns the audit client
func (c *Client) Audit() *AuditClient {
	return c.audit
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

// do performs an HTTP request and decodes the response into v
func (c *Client) doRaw(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	c.client.Timeout = 5 * time.Minute // entended timeout for this request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
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

// get is a helper for sending an HTTP GET request
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

// post is a helper for sending an HTTP POST request
func (c *Client) post(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	// Handle paths that may already include query parameters
	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	url := baseURL + strings.TrimPrefix(path, "/")

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

	if c.debug {
		fmt.Printf("Making POST request to: %s\n", url)
		fmt.Printf("Payload: %s\n", body.String())
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if c.debug && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(body))
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Body: %s\n", string(body))
	}

	return resp, nil
}

// put is a helper for sending an HTTP PUT request
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

// delete is not explicitly shown in your snippet but is referenced, so define it:
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

	return resp, nil
}

// Source-related methods have been moved to sources.go for better organization
// Please use the methods defined there instead of these deprecated ones.

// Example method: retrieving teammates
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

	// Process each teammate to ensure UUID is correctly set
	var validTeammates []Teammate
	for _, t := range teammates {
		// If UUID is empty but ID is set, use ID as UUID
		if t.UUID == "" && t.ID != "" {
			t.UUID = t.ID
			if c.debug {
				fmt.Printf("Warning: UUID was empty for teammate %s, using ID field: %s\n", t.Name, t.ID)
			}
		}

		// Filter out empty entries
		if t.UUID != "" && t.Name != "" {
			validTeammates = append(validTeammates, t)
		}
	}

	return validTeammates, nil
}

// Example method: retrieving a teammate's environment variable
func (c *Client) GetTeammateEnvVar(ctx context.Context, teammateID, varName string) (string, error) {
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

// Example secrets methods
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

// Example method: listing tools from a source
func (c *Client) ListTools(ctx context.Context, sourceURL string) ([]Tool, error) {
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

// Example method: describing a runner
func (c *Client) GetRunner(ctx context.Context, name string) (Runner, error) {
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

// Example method: listing all runners
func (c *Client) ListRunners(ctx context.Context) ([]Runner, error) {
	baseURL := strings.TrimSuffix(c.baseURL, "/api/v1")
	url := fmt.Sprintf("%s/api/v3/runners", baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list runners: %w", err)
	}
	defer resp.Body.Close()

	// First try to decode as an array of raw JSON objects to handle version conversion
	var rawRunners []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawRunners); err != nil {
		return nil, fmt.Errorf("failed to decode runners response: %w", err)
	}

	// Convert the raw runners to proper Runner structs
	runners := make([]Runner, len(rawRunners))

	// Use a wait group to fetch health status concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 5) // Limit concurrent requests

	for i, raw := range rawRunners {
		// Convert version from number to string
		if version, ok := raw["version"].(float64); ok {
			if version == 0 {
				raw["version"] = "v1"
			} else if version == 2 {
				raw["version"] = "v2"
			}
		}

		// Marshal back to JSON and decode into Runner struct
		runnerJSON, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal runner: %w", err)
		}

		if err := json.Unmarshal(runnerJSON, &runners[i]); err != nil {
			return nil, fmt.Errorf("failed to decode runner: %w", err)
		}

		// Start a goroutine to fetch health status
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			healthURL := fmt.Sprintf("%s/api/v3/runners/%s/health", baseURL, runners[index].Name)
			healthReq, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
			if err != nil {
				return
			}

			healthReq.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
			healthReq.Header.Set("Content-Type", "application/json")

			healthResp, err := c.client.Do(healthReq)
			if err != nil {
				return
			}
			defer healthResp.Body.Close()

			if healthResp.StatusCode == http.StatusOK {
				var healthData struct {
					Checks []struct {
						Error    string `json:"error"`
						Metadata struct {
							GitSHA  string `json:"git_sha"`
							Release string `json:"release"`
						} `json:"metadata"`
						Name    string `json:"name"`
						Status  string `json:"status"`
						Version string `json:"version"`
					} `json:"checks"`
					Health  string `json:"health"`
					Status  string `json:"status"`
					Time    string `json:"time"`
					Version string `json:"version"`
				}

				if err := json.NewDecoder(healthResp.Body).Decode(&healthData); err != nil {
					return
				}

				// Use mutex to safely update runner data
				mu.Lock()
				// Update runner health information
				runners[index].RunnerHealth.Status = healthData.Status
				runners[index].RunnerHealth.Health = healthData.Health
				runners[index].RunnerHealth.Version = healthData.Version

				// Update tool manager health
				for _, check := range healthData.Checks {
					if check.Name == "tool-manager" {
						runners[index].ToolManagerHealth.Status = check.Status
						runners[index].ToolManagerHealth.Version = check.Version
						if check.Error != "" {
							runners[index].ToolManagerHealth.Error = check.Error
						}
					} else if check.Name == "agent-manager" {
						runners[index].AgentManagerHealth.Status = check.Status
						runners[index].AgentManagerHealth.Version = check.Version
						if check.Error != "" {
							runners[index].AgentManagerHealth.Error = check.Error
						}
					}
				}
				mu.Unlock()
			}
		}(i)

		// Handle empty health status fields
		if runners[i].RunnerHealth.Status == "" {
			runners[i].RunnerHealth.Status = "unknown"
		}
		if runners[i].RunnerHealth.Health == "" {
			runners[i].RunnerHealth.Health = "unknown"
		}

		// Handle empty tool manager health
		if runners[i].ToolManagerHealth.Status == "" {
			runners[i].ToolManagerHealth.Status = "unknown"
		}
		if runners[i].ToolManagerHealth.Health == "" {
			runners[i].ToolManagerHealth.Health = "unknown"
		}

		// Handle empty agent manager health
		if runners[i].AgentManagerHealth.Status == "" {
			runners[i].AgentManagerHealth.Status = "unknown"
		}
		if runners[i].AgentManagerHealth.Health == "" {
			runners[i].AgentManagerHealth.Health = "unknown"
		}

		// Use kubernetes_namespace if namespace is empty
		if runners[i].Namespace == "" && runners[i].KubernetesNamespace != "" {
			runners[i].Namespace = runners[i].KubernetesNamespace
		}
	}

	// Wait for all health checks to complete
	wg.Wait()

	return runners, nil
}

// GetRunnerManifest retrieves a runner's manifest
func (c *Client) GetRunnerManifest(ctx context.Context, name string) (*RunnerManifest, error) {
	req, err := c.newJSONRequest(ctx, "GET", fmt.Sprintf("/runners/%s/manifest", name), nil)
	if err != nil {
		return nil, err
	}

	var manifest RunnerManifest
	if err := c.do(req, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// GetRunnerHelmChart retrieves the Helm chart configuration for a runner
func (c *Client) GetRunnerHelmChart(ctx context.Context, name string) (*RunnerHelmChart, error) {
	// The endpoint is a different path than the standard API
	url := fmt.Sprintf("%s/api/v3/runners/helmchart/%s", strings.TrimSuffix(c.baseURL, "/api/v1"), name)

	if c.debug {
		fmt.Printf("Getting Helm chart configuration for %s at URL: %s\n", name, url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get Helm chart configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var helmChart RunnerHelmChart
	if err := json.NewDecoder(resp.Body).Decode(&helmChart); err != nil {
		return nil, fmt.Errorf("failed to decode Helm chart response: %w", err)
	}

	return &helmChart, nil
}

// CreateRunnerManifest requests a new runner manifest from the API
func (c *Client) CreateRunnerManifest(ctx context.Context, name string) (RunnerManifest, error) {
	// The endpoint is a different path than the standard API
	url := fmt.Sprintf("%s/api/v3/runners/%s", strings.TrimSuffix(c.baseURL, "/api/v1"), name)

	if c.debug {
		fmt.Printf("Creating runner manifest for %s at URL: %s\n", name, url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return RunnerManifest{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return RunnerManifest{}, fmt.Errorf("failed to create runner manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return RunnerManifest{}, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var manifest RunnerManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return RunnerManifest{}, fmt.Errorf("failed to decode manifest response: %w", err)
	}

	return manifest, nil
}

// Example method: downloading a manifest from an external URL
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

// Example method: creating a teammate
func (c *Client) CreateTeammate(ctx context.Context, teammate Teammate) (*Teammate, error) {
	// Debug output for the request
	if c.debug {
		payload, _ := json.MarshalIndent(teammate, "", "  ")
		fmt.Printf("CreateTeammate request payload:\n%s\n", string(payload))
	}

	resp, err := c.post(ctx, "/agents", teammate)
	if err != nil {
		return nil, fmt.Errorf("failed to create teammate: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug output for the response
	if c.debug {
		fmt.Printf("CreateTeammate response:\n%s\n", string(bodyBytes))
	}

	// Unmarshal into created teammate
	var created Teammate
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		// Try parsing as a different response format
		var altResponse struct {
			ID string `json:"id"`
		}
		if altErr := json.Unmarshal(bodyBytes, &altResponse); altErr == nil && altResponse.ID != "" {
			// We got an alternative format with just an ID
			created.ID = altResponse.ID
			created.UUID = altResponse.ID // Also set UUID to match
			created.Name = teammate.Name  // Copy over the name from the request
			if c.debug {
				fmt.Printf("Parsed teammate ID from alternative response format: %s\n", created.ID)
			}
		} else {
			return nil, fmt.Errorf("failed to unmarshal response: %w\nResponse body: %s", err, string(bodyBytes))
		}
	}

	// Ensure the UUID is set correctly
	if created.UUID == "" && created.ID != "" {
		created.UUID = created.ID
		if c.debug {
			fmt.Printf("Warning: UUID was empty, using ID field: %s\n", created.ID)
		}
	}

	// Verify we have a valid teammate
	if created.UUID == "" {
		if c.debug {
			fmt.Printf("Warning: Created teammate has empty UUID after processing\n")
		}
	}

	return &created, nil
}

// Example method: updating a teammate
func (c *Client) UpdateTeammate(ctx context.Context, uuid string, teammate Teammate) (*Teammate, error) {
	// Debug output
	if c.debug {
		payload, _ := json.MarshalIndent(teammate, "", "  ")
		fmt.Printf("UpdateTeammate request payload:\n%s\n", string(payload))
	}

	resp, err := c.put(ctx, fmt.Sprintf("/agents/%s", uuid), teammate)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug output
	if c.debug {
		fmt.Printf("UpdateTeammate response:\n%s\n", string(bodyBytes))
	}

	var updated Teammate
	if err := json.Unmarshal(bodyBytes, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w\nResponse body: %s", err, string(bodyBytes))
	}

	// Ensure the UUID is set correctly
	if updated.UUID == "" && updated.ID != "" {
		updated.UUID = updated.ID
		if c.debug {
			fmt.Printf("Warning: UUID was empty, using ID field: %s\n", updated.ID)
		}
	}

	return &updated, nil
}

// Example method: deleting a teammate
func (c *Client) DeleteTeammate(ctx context.Context, uuid string) error {
	resp, err := c.delete(ctx, fmt.Sprintf("/agents/%s", uuid))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Example method: binding a source to a teammate
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

// Example method: listing teammates for a particular source
func (c *Client) GetSourceTeammates(ctx context.Context, sourceUUID string) ([]Teammate, error) {
	// Get all teammates first
	teammates, err := c.GetTeammates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list teammates: %w", err)
	}

	// Get source details
	source, err := c.GetSourceMetadata(ctx, sourceUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source details: %w", err)
	}

	// Filter teammates that have this source
	var connectedTeammates []Teammate
	for _, teammate := range teammates {
		hasSource := false
		for _, teammateSrc := range teammate.Sources {
			if teammateSrc == sourceUUID {
				hasSource = true
				break
			}
		}
		if hasSource {
			// Add source details to teammate for context
			teammate.Sources = append(teammate.Sources, source.UUID)
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

// Example helper method to check if a teammate exists
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

// Example method to discover a source
func (c *Client) DiscoverSource(ctx context.Context, sourceURL string, cfg map[string]interface{}, runnerName string, inlineTools []Tool) (*SourceDiscoveryResponse, error) {
	body := struct {
		DynamicConfig map[string]interface{} `json:"dynamic_config"`
		InlineTools   []Tool                 `json:"inline_tools,omitempty"`
	}{
		DynamicConfig: cfg,
		InlineTools:   inlineTools,
	}

	endpoint := "/sources/load"
	if sourceURL != "" {
		endpoint = fmt.Sprintf("%s?url=%s", endpoint, url.QueryEscape(sourceURL))
	}

	if runnerName != "" {
		if strings.Contains(endpoint, "?") {
			endpoint += fmt.Sprintf("&runner=%s", runnerName)
		} else {
			endpoint += fmt.Sprintf("?runner=%s", runnerName)
		}
	}

	resp, err := c.post(ctx, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to discover source: %w", err)
	}
	defer resp.Body.Close()

	var discovery SourceDiscoveryResponse
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If the backend returned errors in the response, we can return them
	if len(discovery.Errors) > 0 {
		return &discovery, &discovery
	}

	return &discovery, nil
}

// Example method to sync a source
func (c *Client) SyncSource(ctx context.Context, sourceID string, opts SyncOptions, runnerName string) (*Source, error) {
	body := struct {
		Mode       string `json:"mode,omitempty"`
		Branch     string `json:"branch,omitempty"`
		Force      bool   `json:"force,omitempty"`
		AutoCommit bool   `json:"auto_commit,omitempty"`
		NoDiff     bool   `json:"no_diff,omitempty"`
	}{
		Mode:       opts.Mode,
		Branch:     opts.Branch,
		Force:      opts.Force,
		AutoCommit: opts.AutoCommit,
		NoDiff:     opts.NoDiff,
	}

	url := fmt.Sprintf("/sources/%s/sync", sourceID)
	if runnerName != "" {
		url += fmt.Sprintf("?runner=%s", runnerName)
	}

	resp, err := c.post(ctx, url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("source not found: %s", sourceID)
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sync failed: %s", string(bodyBytes))
	}

	var source Source
	if err := json.NewDecoder(resp.Body).Decode(&source); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &source, nil
}

// GetBaseURL returns the client's base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// GenerateTool starts a tool generation process (SSE-based) and returns
// a channel of ChatMessage items. This is the updated SSE logic that accumulates
// partial code blocks by filename and emits them as "generated_tool".
func (c *Client) GenerateTool(ctx context.Context, description, sessionID string) (<-chan ToolGenerationChatMessage, error) {
	// Create a channel to receive chat messages
	messages := make(chan ToolGenerationChatMessage)

	// Start a goroutine to read SSE messages
	go func() {
		defer close(messages)

		// Create the SSE request
		reqURL := fmt.Sprintf("%s/http-bridge/v1/generate-tool", strings.TrimSuffix(c.baseURL, "/"))
		jsonData, err := json.Marshal(struct {
			Message   string `json:"message"`
			SessionID string `json:"session_id"`
		}{
			Message:   description,
			SessionID: sessionID,
		})
		if err != nil {
			messages <- ToolGenerationChatMessage{Type: "error", GeneratedToolContent: []GeneratedToolContent{{Content: fmt.Sprintf("failed to marshal payload: %v", err)}}}
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonData))
		if err != nil {
			messages <- ToolGenerationChatMessage{Type: "error", GeneratedToolContent: []GeneratedToolContent{{Content: fmt.Sprintf("failed to create request: %v", err)}}}
			return
		}

		// Set headers to request SSE
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		// We'll use a client with no (or very large) timeout for SSE
		sseClient := &http.Client{Timeout: 0}

		resp, err := sseClient.Do(req)
		if err != nil {
			messages <- ToolGenerationChatMessage{Type: "error", GeneratedToolContent: []GeneratedToolContent{{Content: fmt.Sprintf("failed to execute request: %v", err)}}}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			messages <- ToolGenerationChatMessage{Type: "error", GeneratedToolContent: []GeneratedToolContent{{Content: fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))}}}
			return
		}

		// Create a new SSE reader
		sseReader := bufio.NewReader(resp.Body)

		// Read SSE messages
		for {
			line, err := sseReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				messages <- ToolGenerationChatMessage{Type: "error", GeneratedToolContent: []GeneratedToolContent{{Content: err.Error()}}}
				return
			}

			content := strings.TrimSpace(line)

			// Parse the JSON content into a ToolGenerationChatMessage struct
			var msg ToolGenerationChatMessage
			if err := json.Unmarshal([]byte(content), &msg); err != nil {
				messages <- ToolGenerationChatMessage{Type: "error", GeneratedToolContent: []GeneratedToolContent{{Content: fmt.Sprintf("failed to parse message: %v", err)}}}
				continue
			}

			// Send the parsed message to the channel
			messages <- msg
		}
	}()

	return messages, nil
}

// ListProjects retrieves all projects
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	if c.debug {
		fmt.Printf("Making request to: %s/usecases\n", c.baseURL)
	}

	resp, err := c.get(ctx, "/usecases")
	if err != nil {
		if c.debug {
			fmt.Printf("Error fetching projects: %v\n", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if c.debug {
		body, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(body))
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Body: %s\n", string(body))
	}

	var response struct {
		Usecases []Project `json:"usecases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		if c.debug {
			fmt.Printf("Error decoding response: %v\n", err)
		}
		return nil, err
	}

	if c.debug {
		fmt.Printf("Parsed %d projects\n", len(response.Usecases))
		for i, p := range response.Usecases {
			fmt.Printf("Project %d: Name=%s, UUID=%s, Status=%s\n",
				i+1, p.Name, p.UUID, p.Status)
		}
	}

	return response.Usecases, nil
}

// GetProject retrieves a project by UUID
func (c *Client) GetProject(ctx context.Context, uuid string) (*Project, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/tasks/%s", uuid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, err
	}
	return &project, nil
}

// CreateProject creates a new project
func (c *Client) CreateProject(ctx context.Context, templateID string, name string, description string, variables map[string]string) (*Project, error) {
	// Validate variables against template
	if templateID != "" {
		missingVars, extraVars, typeErrors, err := c.ValidateVariablesAgainstTemplate(ctx, templateID, variables)
		if err != nil {
			return nil, err
		}

		// Error if any required variables are missing
		if len(missingVars) > 0 {
			return nil, fmt.Errorf("missing required variables: %s", strings.Join(missingVars, ", "))
		}

		// Error if any type validation errors
		if len(typeErrors) > 0 {
			return nil, fmt.Errorf("variable type validation errors: %s", strings.Join(typeErrors, ", "))
		}

		// Log extra variables as warning
		if len(extraVars) > 0 && c.debug {
			fmt.Printf("Warning: The following variables are not defined in the template: %s\n",
				strings.Join(extraVars, ", "))
		}
	}

	// Convert variables to Variable objects
	validatedVars := make([]Variable, 0, len(variables))
	for name, value := range variables {
		validatedVars = append(validatedVars, Variable{
			Name:  name,
			Value: value,
			Type:  "string", // Default type
		})
	}

	// Create request body
	requestBody := map[string]interface{}{
		"template_id": templateID,
		"name":        name,
		"description": description,
		"variables":   validatedVars,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/usecases", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, err
	}

	return &project, nil
}

// UpdateProject updates an existing project
func (c *Client) UpdateProject(ctx context.Context, projectID string, name string, description string, variables map[string]string) (*Project, error) {
	// Validate variables
	// First get project to determine template ID
	project, err := c.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// If the project has a template ID, validate variables against it
	var validatedVars []Variable
	if templateID := project.UsecaseID; templateID != "" {
		missingVars, extraVars, typeErrors, err := c.ValidateVariablesAgainstTemplate(ctx, templateID, variables)
		if err != nil {
			return nil, err
		}

		// Error if any required variables are missing
		if len(missingVars) > 0 {
			return nil, fmt.Errorf("missing required variables: %s", strings.Join(missingVars, ", "))
		}

		// Error if any type validation errors
		if len(typeErrors) > 0 {
			return nil, fmt.Errorf("variable type validation errors: %s", strings.Join(typeErrors, ", "))
		}

		// Log extra variables as warning
		if len(extraVars) > 0 && c.debug {
			fmt.Printf("Warning: The following variables are not defined in the template: %s\n",
				strings.Join(extraVars, ", "))
		}

		// Convert variables to Variable objects
		validatedVars = make([]Variable, 0, len(variables))
		for name, value := range variables {
			validatedVars = append(validatedVars, Variable{
				Name:  name,
				Value: value,
				Type:  "string", // Default type
			})
		}
	} else {
		// Simple conversion if no template
		validatedVars = make([]Variable, 0, len(variables))
		for name, value := range variables {
			validatedVars = append(validatedVars, Variable{
				Name:  name,
				Value: value,
				Type:  "string",
			})
		}
	}

	// Create request body
	requestBody := map[string]interface{}{
		"name":        name,
		"description": description,
		"variables":   validatedVars,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/usecases/%s", c.baseURL, projectID)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var updatedProject Project
	if err := json.NewDecoder(resp.Body).Decode(&updatedProject); err != nil {
		return nil, err
	}

	return &updatedProject, nil
}

// DeleteProject deletes a project by UUID
func (c *Client) DeleteProject(ctx context.Context, uuid string) error {
	resp, err := c.delete(ctx, fmt.Sprintf("/tasks/%s", uuid))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ListProjectTemplates retrieves project templates from a repository
func (c *Client) ListProjectTemplates(ctx context.Context, repository string) ([]ProjectTemplate, error) {
	var urlString string
	// The correct API endpoint for templates is /api/v1/usecases
	urlString = fmt.Sprintf("%s/usecases", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", urlString, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if c.debug {
		body, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(body))
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Body: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Usecases []ProjectTemplate `json:"usecases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Usecases, nil
}

// GetProjectTemplate retrieves a project template by ID
func (c *Client) GetProjectTemplate(ctx context.Context, id string) (*ProjectTemplate, error) {
	// The correct API endpoint for a specific template is /api/v1/usecases/{id}
	url := fmt.Sprintf("%s/usecases/%s", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if c.debug {
		body, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(body))
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Body: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// The API returns the template directly, not wrapped in a "template" field
	var template ProjectTemplate

	if err := json.NewDecoder(resp.Body).Decode(&template); err != nil {
		return nil, err
	}

	return &template, nil
}

// CreateProjectPlan creates a new plan for a project
func (c *Client) CreateProjectPlan(ctx context.Context, projectID string) (*ProjectPlan, error) {
	payload := map[string]interface{}{
		"project_id": projectID,
	}

	resp, err := c.post(ctx, fmt.Sprintf("/tasks/plan/%s", projectID), payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var plan ProjectPlan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// GetProjectPlan retrieves a plan by ID
func (c *Client) GetProjectPlan(ctx context.Context, planID string) (*ProjectPlan, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/tasks/plan/%s", planID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var plan ProjectPlan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// ApproveProjectPlan approves a plan for execution
func (c *Client) ApproveProjectPlan(ctx context.Context, planID string) (*ProjectExecution, error) {
	// The API uses PUT with the same payload as in the example
	payload := map[string]interface{}{
		"action": "approve",
	}

	resp, err := c.put(ctx, fmt.Sprintf("/tasks/%s", planID), payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var execution ProjectExecution
	if err := json.NewDecoder(resp.Body).Decode(&execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetProjectExecution retrieves an execution by ID
func (c *Client) GetProjectExecution(ctx context.Context, executionID string) (*ProjectExecution, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/tasks/%s", executionID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var execution ProjectExecution
	if err := json.NewDecoder(resp.Body).Decode(&execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetProjectExecutionLogs retrieves logs for an execution
func (c *Client) GetProjectExecutionLogs(ctx context.Context, executionID string) ([]string, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/tasks/logs/%s", executionID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var logsResponse struct {
		Done bool `json:"done"`
		Plan struct {
			Status  string `json:"status"`
			Content string `json:"content"`
		} `json:"plan"`
		Apply struct {
			Status  string `json:"status"`
			Content string `json:"content"`
		} `json:"apply"`
		Status string   `json:"status"`
		Errors []string `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&logsResponse); err != nil {
		return nil, err
	}

	logs := []string{}
	if logsResponse.Plan.Content != "" {
		logs = append(logs, fmt.Sprintf("Plan: %s\n%s", logsResponse.Plan.Status, logsResponse.Plan.Content))
	}
	if logsResponse.Apply.Content != "" {
		logs = append(logs, fmt.Sprintf("Apply: %s\n%s", logsResponse.Apply.Status, logsResponse.Apply.Content))
	}
	for _, err := range logsResponse.Errors {
		logs = append(logs, fmt.Sprintf("Error: %s", err))
	}

	return logs, nil
}

// ValidateVariableType validates that a variable value matches the expected type
func (c *Client) ValidateVariableType(value string, expectedType string) (interface{}, error) {
	switch expectedType {
	case "string":
		return value, nil
	case "number":
		// Try to parse as int first
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal, nil
		}
		// Then try as float
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal, nil
		}
		return nil, fmt.Errorf("value '%s' is not a valid number", value)
	case "boolean", "bool":
		switch strings.ToLower(value) {
		case "true", "yes", "1", "y":
			return true, nil
		case "false", "no", "0", "n":
			return false, nil
		default:
			return nil, fmt.Errorf("value '%s' is not a valid boolean", value)
		}
	case "list", "array":
		// If it starts with [ and ends with ], try to parse as JSON
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			var list []interface{}
			if err := json.Unmarshal([]byte(value), &list); err != nil {
				return nil, fmt.Errorf("value '%s' is not a valid list: %w", value, err)
			}
			return list, nil
		}
		// Otherwise, try to parse as comma-separated values
		items := strings.Split(value, ",")
		trimmedItems := make([]string, 0, len(items))
		for _, item := range items {
			trimmedItems = append(trimmedItems, strings.TrimSpace(item))
		}
		return trimmedItems, nil
	case "map", "object":
		// If it starts with { and ends with }, try to parse as JSON
		if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(value), &obj); err != nil {
				return nil, fmt.Errorf("value '%s' is not a valid object: %w", value, err)
			}
			return obj, nil
		}
		// Otherwise, try to parse as key=value pairs
		items := strings.Split(value, ",")
		obj := make(map[string]string)
		for _, item := range items {
			parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("value '%s' is not a valid key=value pair", item)
			}
			obj[parts[0]] = parts[1]
		}
		return obj, nil
	default:
		// For unknown types, just pass the string value
		return value, nil
	}
}

// ValidateVariablesAgainstTemplate validates that all required variables are present
// and their types match what's expected by the template.
func (c *Client) ValidateVariablesAgainstTemplate(ctx context.Context, templateID string, variables map[string]string) ([]string, []string, []string, error) {
	// Get the template
	template, err := c.GetProjectTemplate(ctx, templateID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Collect all variables from template and resources
	allVariables := make(map[string]TemplateVariable)
	requiredVars := make(map[string]bool)

	// Add template variables
	for _, v := range template.Variables {
		allVariables[v.Name] = v
		if v.Required && v.Default == nil {
			requiredVars[v.Name] = true
		}
	}

	// Add resource variables
	for _, resource := range template.Resources {
		for _, v := range resource.Variables {
			allVariables[v.Name] = TemplateVariable{
				Name:        v.Name,
				Type:        v.Type,
				Default:     v.Default,
				Description: v.Description,
				Required:    true, // Consider resource variables required
			}
			requiredVars[v.Name] = true
		}
	}

	// Find missing required variables
	var missingRequired []string
	for name := range requiredVars {
		if _, ok := variables[name]; !ok {
			missingRequired = append(missingRequired, name)
		}
	}

	// Find variables not in template
	var extraVars []string
	for name := range variables {
		if _, ok := allVariables[name]; !ok {
			extraVars = append(extraVars, name)
		}
	}

	// Validate types of provided variables
	var typeErrors []string
	for name, value := range variables {
		if templateVar, ok := allVariables[name]; ok {
			valid, errMsg := validateVariableType(value, templateVar.Type)
			if !valid {
				typeErrors = append(typeErrors, fmt.Sprintf("%s: %s", name, errMsg))
			}
		}
	}

	return missingRequired, extraVars, typeErrors, nil
}

// Helper function to validate variable types
func validateVariableType(value, expectedType string) (bool, string) {
	switch expectedType {
	case "string":
		// All values can be treated as strings
		return true, ""

	case "number":
		// Try to parse as a number
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return false, fmt.Sprintf("expected number but got '%s'", value)
		}

	case "bool", "boolean":
		// Try to parse as a boolean
		lowerValue := strings.ToLower(value)
		if lowerValue != "true" && lowerValue != "false" &&
			lowerValue != "0" && lowerValue != "1" &&
			lowerValue != "yes" && lowerValue != "no" {
			return false, fmt.Sprintf("expected boolean but got '%s'", value)
		}

	case "list", "array":
		// Try to parse as a JSON array
		var list []interface{}
		if err := json.Unmarshal([]byte(value), &list); err != nil {
			return false, fmt.Sprintf("expected JSON array but got '%s'", value)
		}

	case "map", "object":
		// Try to parse as a JSON object
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(value), &m); err != nil {
			return false, fmt.Sprintf("expected JSON object but got '%s'", value)
		}
	}

	return true, ""
}
