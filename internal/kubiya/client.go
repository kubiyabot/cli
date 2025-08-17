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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	sentryutil "github.com/kubiyabot/cli/internal/sentry"
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

// logAPICall logs all API calls to /tmp/klog.txt
func logAPICall(method, url string, headers map[string]string, body []byte, responseStatus int, responseBody []byte) {
	if os.Getenv("KUBIYA_DEBUG") != "true" {
		return // Only log if KUBIYA_DEBUG is set to true
	}
	f, err := os.OpenFile("/tmp/klog.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return // Silently fail if we can't log
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	logEntry := fmt.Sprintf("\n=== API Call %s ===\n", timestamp)
	logEntry += fmt.Sprintf("Method: %s\n", method)
	logEntry += fmt.Sprintf("URL: %s\n", url)
	logEntry += fmt.Sprintf("Headers: %v\n", headers)
	if body != nil {
		logEntry += fmt.Sprintf("Request Body: %s\n", string(body))
	}
	logEntry += fmt.Sprintf("Response Status: %d\n", responseStatus)
	if responseBody != nil {
		logEntry += fmt.Sprintf("Response Body: %s\n", string(responseBody))
	}
	logEntry += "=======================================\n"

	f.WriteString(logEntry)
}

// NewClient creates a new Kubiya API client
func NewClient(cfg *config.Config) *Client {
	client := &Client{
		cfg:     cfg,
		baseURL: cfg.BaseURL,
		debug:   cfg.Debug,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: NewAuthRoundTripper(),
		},
		cache: NewCache(5 * time.Minute),
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

	return req, nil
}

// get is a helper for sending an HTTP GET request
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		// Log the failed API call
		headers := map[string]string{
			"Authorization": "UserKey [REDACTED]",
			"Content-Type":  "application/json",
		}
		logAPICall("GET", url, headers, nil, 0, []byte(fmt.Sprintf("Request failed: %v", err)))
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Capture response body for logging
	responseBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

	// Log the API call
	headers := map[string]string{
		"Authorization": "UserKey [REDACTED]",
		"Content-Type":  "application/json",
	}
	logAPICall("GET", url, headers, nil, resp.StatusCode, responseBody)

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
		if c.debug {
			payloadBytes, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Printf("POST method serializing payload:\n%s\n", string(payloadBytes))
		}
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, fmt.Errorf("failed to encode payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

// PostRaw sends a POST request with raw JSON data
func (c *Client) PostRaw(ctx context.Context, path string, data []byte) (*http.Response, error) {
	// Handle paths that may already include query parameters
	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	url := baseURL + strings.TrimPrefix(path, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if c.debug {
		fmt.Printf("Making POST request to: %s\n", url)
		fmt.Printf("Payload: %s\n", string(data))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Capture response body for logging
	responseBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

	// Log the API call
	headers := map[string]string{
		"Authorization": "UserKey [REDACTED]",
		"Content-Type":  "application/json",
	}
	logAPICall("POST", url, headers, data, resp.StatusCode, responseBody)

	if c.debug && resp.StatusCode != http.StatusOK {
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Body: %s\n", string(responseBody))
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

	req.Header.Set("Content-Type", "application/json")

	// Capture request body for logging
	requestBody := body.Bytes()

	resp, err := c.client.Do(req)
	if err != nil {
		// Log the failed API call
		headers := map[string]string{
			"Authorization": "UserKey [REDACTED]",
			"Content-Type":  "application/json",
		}
		logAPICall("PUT", url, headers, requestBody, 0, []byte(fmt.Sprintf("Request failed: %v", err)))
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Capture response body for logging
	responseBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

	// Log the API call
	headers := map[string]string{
		"Authorization": "UserKey [REDACTED]",
		"Content-Type":  "application/json",
	}
	logAPICall("PUT", url, headers, requestBody, resp.StatusCode, responseBody)

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(responseBody))
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
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		// Log the failed API call
		headers := map[string]string{
			"Authorization": "UserKey [REDACTED]",
			"Content-Type":  "application/json",
		}
		logAPICall("DELETE", url, headers, nil, 0, []byte(fmt.Sprintf("Request failed: %v", err)))
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	// Capture response body for logging
	responseBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

	// Log the API call
	headers := map[string]string{
		"Authorization": "UserKey [REDACTED]",
		"Content-Type":  "application/json",
	}
	logAPICall("DELETE", url, headers, nil, resp.StatusCode, responseBody)

	return resp, nil
}

// Source-related methods have been moved to sources.go for better organization
// Please use the methods defined there instead of these deprecated ones.

// Example method: retrieving agents
func (c *Client) GetAgents(ctx context.Context) ([]Agent, error) {
	resp, err := c.get(ctx, "/agents?mode=all")
	if err != nil {
		if c.debug {
			fmt.Printf("Error getting agents: %v\n", err)
			fmt.Printf("BaseURL: %s\n", c.baseURL)
			fmt.Printf("Full URL: %s/agents?mode=all\n", c.baseURL)
			fmt.Printf("API Key present: %v\n", c.cfg.APIKey != "")
		}
		return nil, fmt.Errorf("failed to get agents: %w", err)
	}
	defer resp.Body.Close()

	if c.debug {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Headers: %v\n", resp.Header)
		fmt.Printf("Response Body: %s\n", string(body))
		resp.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	var agents []Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		if c.debug {
			fmt.Printf("Error decoding response: %v\n", err)
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Process each agent to ensure UUID is correctly set
	var validAgents []Agent
	for _, t := range agents {
		// If UUID is empty but ID is set, use ID as UUID
		if t.UUID == "" && t.ID != "" {
			t.UUID = t.ID
			if c.debug {
				fmt.Printf("Warning: UUID was empty for agent %s, using ID field: %s\n", t.Name, t.ID)
			}
		}

		// Filter out empty entries
		if t.UUID != "" && t.Name != "" {
			validAgents = append(validAgents, t)
		}
	}

	return validAgents, nil
}

// Example method: retrieving a agent's environment variable
func (c *Client) GetAgentEnvVar(ctx context.Context, agentID, varName string) (string, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/v1/agents/%s", agentID))
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
	return "", fmt.Errorf("environment variable %s not found for agent", varName)
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

// GetRunner retrieves a specific runner's information including health status
func (c *Client) GetRunner(ctx context.Context, name string) (Runner, error) {
	// First, try to get the runner from the list of all runners (v3 API)
	runners, err := c.ListRunners(ctx)
	if err != nil {
		return Runner{}, fmt.Errorf("failed to list runners: %w", err)
	}

	// Find the specific runner
	for _, runner := range runners {
		if runner.Name == name {
			return runner, nil
		}
	}

	// If not found in the list, try the v1 API as fallback
	resp, err := c.get(ctx, fmt.Sprintf("/runners/%s/describe", name))
	if err != nil {
		return Runner{}, fmt.Errorf("runner '%s' not found", name)
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

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list runners: %w", err)
	}
	defer resp.Body.Close()

	// Read and debug the response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if c.debug {
		fmt.Printf("DEBUG: Raw API response: %s\n", string(bodyBytes))
	}

	// First try to decode as an array of raw JSON objects to handle version conversion
	var rawRunners []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rawRunners); err != nil {
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

// ListUsers retrieves all users in the organization
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	// The baseURL is like https://api.kubiya.ai/api/v1, we need https://api.kubiya.ai/api/v2/users
	baseURL := strings.Replace(c.baseURL, "/v1", "/v2", 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/users?limit=100&page=1", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var response struct {
		Items []User `json:"items"`
	}
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return response.Items, nil
}

// ListGroups retrieves all groups in the organization
func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/manage/groups", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var groups []Group
	if err := c.do(req, &groups); err != nil {
		return nil, err
	}

	return groups, nil
}

// Example method: creating a agent
func (c *Client) CreateAgent(ctx context.Context, agent Agent) (*Agent, error) {
	fmt.Printf("TRACE: CreateAgent called with agent name: %s\n", agent.Name)

	// Create a minimal API-compliant payload based on API documentation
	// This completely bypasses the problematic Agent struct serialization
	payload := map[string]interface{}{
		"name":             agent.Name,
		"description":      agent.Description,
		"instruction_type": agent.InstructionType,
		"llm_model":        agent.LLMModel,
	}

	// Only include optional fields if they have meaningful values
	if len(agent.Sources) > 0 {
		payload["sources"] = agent.Sources
	}
	if len(agent.Tools) > 0 {
		payload["tools"] = agent.Tools
	}
	if agent.AIInstructions != "" {
		payload["ai_instructions"] = agent.AIInstructions
	}
	if len(agent.Secrets) > 0 {
		payload["secrets"] = agent.Secrets
	}
	if len(agent.Integrations) > 0 {
		payload["integrations"] = agent.Integrations
	}
	if agent.Environment != nil && len(agent.Environment) > 0 {
		payload["environment_variables"] = agent.Environment
	}

	// Debug output for the request
	if c.debug {
		fmt.Printf("DEBUG: payload type: %T\n", payload)
		payloadBytes, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Printf("CreateAgent request payload:\n%s\n", string(payloadBytes))
	}

	// Use PostRaw to send exactly what we want, bypassing any struct serialization
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := c.PostRaw(ctx, "/agents", payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug output for the response
	if c.debug {
		fmt.Printf("CreateAgent response:\n%s\n", string(bodyBytes))
	}

	// Check for error status codes
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("agent creation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Unmarshal into created agent
	var created Agent
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		// Try parsing as a different response format
		var altResponse struct {
			ID string `json:"id"`
		}
		if altErr := json.Unmarshal(bodyBytes, &altResponse); altErr == nil && altResponse.ID != "" {
			// We got an alternative format with just an ID
			created.ID = altResponse.ID
			created.UUID = altResponse.ID // Also set UUID to match
			created.Name = agent.Name     // Copy over the name from the request
			if c.debug {
				fmt.Printf("Parsed agent ID from alternative response format: %s\n", created.ID)
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

	// Verify we have a valid agent
	if created.UUID == "" {
		if c.debug {
			fmt.Printf("Warning: Created agent has empty UUID after processing\n")
		}
	}

	return &created, nil
}

// Example method: updating a agent
func (c *Client) UpdateAgent(ctx context.Context, uuid string, agent Agent) (*Agent, error) {
	// Create a minimal API-compliant payload similar to CreateAgent
	payload := map[string]interface{}{
		"name":             agent.Name,
		"description":      agent.Description,
		"instruction_type": agent.InstructionType,
		"llm_model":        agent.LLMModel,
	}

	// Only include optional fields if they have meaningful values
	if len(agent.Sources) > 0 {
		payload["sources"] = agent.Sources
	}
	if len(agent.Tools) > 0 {
		payload["tools"] = agent.Tools
	}
	if agent.AIInstructions != "" {
		payload["ai_instructions"] = agent.AIInstructions
	}
	if len(agent.Secrets) > 0 {
		payload["secrets"] = agent.Secrets
	}
	if len(agent.Integrations) > 0 {
		payload["integrations"] = agent.Integrations
	}
	if agent.Environment != nil && len(agent.Environment) > 0 {
		payload["environment_variables"] = agent.Environment
	}

	// Debug output
	if c.debug {
		payloadBytes, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Printf("UpdateAgent request payload:\n%s\n", string(payloadBytes))
	}

	// Use the raw update method to avoid struct serialization issues
	return c.UpdateAgentRaw(ctx, uuid, payload)
}

// UpdateAgentRaw updates an agent using raw JSON data to avoid struct field issues
func (c *Client) UpdateAgentRaw(ctx context.Context, uuid string, data map[string]interface{}) (*Agent, error) {
	// Debug output
	if c.debug {
		payload, _ := json.MarshalIndent(data, "", "  ")
		fmt.Printf("UpdateAgentRaw request payload:\n%s\n", string(payload))
	}

	resp, err := c.put(ctx, fmt.Sprintf("/agents/%s", uuid), data)
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
		fmt.Printf("UpdateAgentRaw response:\n%s\n", string(bodyBytes))
	}

	var updated Agent
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

// Example method: deleting a agent
func (c *Client) DeleteAgent(ctx context.Context, uuid string) error {
	resp, err := c.delete(ctx, fmt.Sprintf("/v1/agents/%s", uuid))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Example method: binding a source to a agent
func (c *Client) BindSourceToAgent(ctx context.Context, sourceUUID, agentUUID string) error {
	path := fmt.Sprintf("sources/%s/agents/%s", sourceUUID, agentUUID)
	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to bind source to agent: %s", resp.Status)
	}
	return nil
}

// Example method: listing agents for a particular source
func (c *Client) GetSourceAgents(ctx context.Context, sourceUUID string) ([]Agent, error) {
	// Get all agents first
	agents, err := c.GetAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Get source details
	source, err := c.GetSourceMetadata(ctx, sourceUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source details: %w", err)
	}

	// Filter agents that have this source
	var connectedAgents []Agent
	for _, agent := range agents {
		hasSource := false
		for _, agentSrc := range agent.Sources {
			if agentSrc == sourceUUID {
				hasSource = true
				break
			}
		}
		if hasSource {
			// Add source details to agent for context
			agent.Sources = append(agent.Sources, source.UUID)
			connectedAgents = append(connectedAgents, agent)
		}
	}

	if c.debug {
		fmt.Printf("Found %d agents connected to source %s\n", len(connectedAgents), sourceUUID)
		for _, t := range connectedAgents {
			fmt.Printf("- %s (UUID: %s)\n", t.Name, t.UUID)
		}
	}

	return connectedAgents, nil
}

// Example helper method to check if a agent exists
func (c *Client) AgentExists(ctx context.Context, nameOrID string) (*Agent, error) {
	agents, err := c.GetAgents(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range agents {
		if t.UUID == nameOrID || t.Name == nameOrID {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("agent not found: %s", nameOrID)
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
		req.Header.Set("x-vercel-ai-data-stream", "v1") // protocol flag
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		// Execute request with extended timeout for long-running tools
		httpClient := &http.Client{
			Timeout: 0, // No timeout for streaming connections
		}
		resp, err := httpClient.Do(req)
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

// isRunnerHealthy checks if a runner is healthy based on various status fields
func (c *Client) isRunnerHealthy(runner Runner) bool {
	// Check various status fields that indicate health
	status := strings.ToLower(runner.RunnerHealth.Status)
	health := strings.ToLower(runner.RunnerHealth.Health)

	// Accept various indicators of health
	return status == "healthy" || status == "ok" || status == "ready" ||
		health == "healthy" || health == "true" || health == "ok" ||
		(status == "" && health == "") // Sometimes no status means it's running fine
}

// findHealthyRunnerQuickly attempts to quickly find a healthy runner with short timeouts
func (c *Client) findHealthyRunnerQuickly(ctx context.Context) (string, error) {
	// Create a context with a short timeout for the entire operation
	quickCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Get list of runners quickly
	runners, err := c.ListRunners(quickCtx)
	if err != nil {
		return "", fmt.Errorf("failed to list runners quickly: %w", err)
	}

	// Define priority order for runners
	priorityRunners := []string{"kubiya-hosted", "kubiya-hosted-1", "kubiya-cloud"}

	// First, check priority runners
	for _, runnerName := range priorityRunners {
		for _, runner := range runners {
			if runner.Name == runnerName && c.isRunnerHealthy(runner) {
				return runner.Name, nil
			}
		}
	}

	// Then check any other healthy runner
	for _, runner := range runners {
		if c.isRunnerHealthy(runner) {
			return runner.Name, nil
		}
	}

	return "", fmt.Errorf("no healthy runners found")
}

// ExecuteToolWithTimeout executes a tool directly using the tool execution API with a configurable timeout
func (c *Client) ExecuteToolWithTimeout(ctx context.Context, toolName string, toolDef map[string]interface{}, runner string, timeout time.Duration, args map[string]any) (<-chan WorkflowSSEEvent, error) {
	// Use comprehensive tool execution tracing
	var eventChan <-chan WorkflowSSEEvent
	var execErr error

	err := sentryutil.WithToolExecution(ctx, toolName, runner, func(ctx context.Context) error {
		// Create Sentry span for tracing (keeping existing pattern)
		span, ctx := sentryutil.StartSpan(ctx, "execute_tool")
		if span != nil {
			span.SetTag("tool.name", toolName)
			span.SetTag("runner", runner)
			span.SetTag("timeout", timeout.String())
			defer span.Finish()
		}

		eventChan, execErr = c.executeToolWithTimeoutInternal(ctx, toolName, toolDef, runner, timeout, args)
		return execErr
	})

	if err != nil {
		return nil, err
	}

	return eventChan, nil
}

// executeToolWithTimeoutInternal contains the actual implementation
func (c *Client) executeToolWithTimeoutInternal(ctx context.Context, toolName string, toolDef map[string]interface{}, runner string, timeout time.Duration, args map[string]any) (<-chan WorkflowSSEEvent, error) {

	// Handle "auto" runner selection with fast failover
	selectedRunner := runner
	if runner == "auto" {
		// Try to quickly find a healthy runner
		healthyRunner, err := c.findHealthyRunnerQuickly(ctx)
		if err != nil {
			// If we can't find a healthy runner quickly, try kubiya-hosted first
			selectedRunner = "kubiya-hosted"
			if c.debug {
				fmt.Printf("[DEBUG] No healthy runner found quickly, trying kubiya-hosted\n")
			}
			sentryutil.AddBreadcrumb("runner_selection", "No healthy runner found, using default", map[string]interface{}{
				"default_runner": selectedRunner,
			})
		} else {
			selectedRunner = healthyRunner
			if c.debug {
				fmt.Printf("[DEBUG] Selected healthy runner: %s\n", selectedRunner)
			}
			sentryutil.AddBreadcrumb("runner_selection", "Selected healthy runner", map[string]interface{}{
				"runner": selectedRunner,
			})
		}
	}

	// Build URL with query parameters
	params := url.Values{}
	params.Set("runner", selectedRunner)
	execURL := fmt.Sprintf("%s/tools/exec?%s", c.baseURL, params.Encode())

	// Create request body
	body := map[string]interface{}{
		"tool_name": toolName,
		"tool_def":  toolDef,
		"args":      args,
	}

	// If auto was selected and we're using a fallback runner, prepare a list of runners to try
	var runnersToTry []string
	if runner == "auto" {
		// If we already selected a healthy runner, just use it
		runnersToTry = []string{selectedRunner}
		// But also prepare fallbacks in case it fails
		if selectedRunner == "kubiya-hosted" {
			runnersToTry = append(runnersToTry, "kubiya-hosted-1", "kubiya-cloud")
		}
	} else {
		// For specific runner, only try that one
		runnersToTry = []string{selectedRunner}
	}

	var lastErr error
	var resultEvents <-chan WorkflowSSEEvent

	// Implement simple retry logic with exponential backoff
	maxAttempts := len(runnersToTry)
	retryDelay := 1 * time.Second

	for attemptIndex := 0; attemptIndex < maxAttempts; attemptIndex++ {
		tryRunner := runnersToTry[attemptIndex]

		if attemptIndex > 0 {
			// Exponential backoff
			time.Sleep(retryDelay)
			retryDelay = retryDelay * 2
			if retryDelay > 10*time.Second {
				retryDelay = 10 * time.Second
			}

			// Log retry attempt
			if c.debug {
				fmt.Printf("[DEBUG] Retrying with runner: %s (attempt %d/%d)\n", tryRunner, attemptIndex+1, len(runnersToTry))
			}
			sentryutil.AddBreadcrumb("retry", fmt.Sprintf("Retrying with runner %s", tryRunner), map[string]interface{}{
				"attempt":      attemptIndex + 1,
				"total":        len(runnersToTry),
				"previous_err": lastErr.Error(),
			})
		}

		// Update the URL with the current runner
		params.Set("runner", tryRunner)
		execURL = fmt.Sprintf("%s/tools/exec?%s", c.baseURL, params.Encode())

		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, execURL, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers for SSE
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-vercel-ai-data-stream", "v1") // protocol flag
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		// Use a shorter timeout for initial connection to fail fast
		connectTimeout := 10 * time.Second
		if timeout > 0 && timeout < connectTimeout {
			connectTimeout = timeout
		}

		// Execute request with shorter timeout for connection
		httpClient := &http.Client{
			Timeout: connectTimeout,
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("runner %s: failed to connect: %w", tryRunner, err)
			// Log the failed API call
			headers := map[string]string{
				"Authorization": "UserKey [REDACTED]",
				"Content-Type":  "application/json",
				"Accept":        "text/event-stream",
			}
			logAPICall("POST", execURL, headers, jsonBody, 0, []byte(fmt.Sprintf("Connection failed: %v", err)))
			// Check if it's retryable
			if attemptIndex < maxAttempts-1 && runner == "auto" {
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// Log the failed API call
			headers := map[string]string{
				"Authorization": "UserKey [REDACTED]",
				"Content-Type":  "application/json",
				"Accept":        "text/event-stream",
			}
			logAPICall("POST", execURL, headers, jsonBody, resp.StatusCode, body)
			lastErr = fmt.Errorf("runner %s: execution failed with status %d: %s", tryRunner, resp.StatusCode, string(body))

			// Don't retry on client errors (4xx)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, lastErr
			}

			// Retry on server errors if we have more runners
			if attemptIndex < maxAttempts-1 && runner == "auto" && resp.StatusCode >= 500 {
				continue
			}

			return nil, lastErr
		}

		// Log successful API call
		headers := map[string]string{
			"Authorization": "UserKey [REDACTED]",
			"Content-Type":  "application/json",
			"Accept":        "text/event-stream",
		}
		logAPICall("POST", execURL, headers, jsonBody, resp.StatusCode, []byte("SSE stream established"))

		// Success! Create streaming channel
		events, streamErr := c.streamToolExecution(resp, timeout, tryRunner)
		if streamErr != nil {
			lastErr = streamErr
			if attemptIndex < maxAttempts-1 && runner == "auto" {
				continue
			}
			return nil, streamErr
		}

		// Success
		resultEvents = events
		break
	}

	if resultEvents == nil {
		sentryutil.CaptureError(lastErr, map[string]string{
			"tool":   toolName,
			"runner": runner,
		}, map[string]interface{}{
			"attempts":      maxAttempts,
			"runners_tried": runnersToTry,
		})
		return nil, fmt.Errorf("all runners failed, last error: %w", lastErr)
	}

	return resultEvents, nil
}

// streamToolExecution handles the SSE streaming from successful connection
func (c *Client) streamToolExecution(resp *http.Response, timeout time.Duration, runnerName string) (<-chan WorkflowSSEEvent, error) {
	// Create channel for streaming events
	events := make(chan WorkflowSSEEvent)
	lastEventTime := time.Now()

	// Send initial event about runner selection
	go func() {
		events <- WorkflowSSEEvent{
			Type: "data",
			Data: fmt.Sprintf(`{"type":"runner","runner":"%s"}`, runnerName),
		}
	}()

	go func() {
		defer close(events)
		defer resp.Body.Close()

		// Create a reader that will keep the connection alive
		reader := bufio.NewReader(resp.Body)

		// Start a goroutine to monitor for timeouts only if timeout > 0
		// Use a much more generous timeout for SSE streams to allow long-running silent operations
		if timeout > 0 {
			// For tool executions, use a longer inactivity timeout (3x the original timeout or minimum 1 hour)
			inactivityTimeout := timeout * 3
			if inactivityTimeout < time.Hour {
				inactivityTimeout = time.Hour // Minimum 1 hour for long-running tools
			}

			ticker := time.NewTicker(30 * time.Second) // Check less frequently
			defer ticker.Stop()

			go func() {
				for range ticker.C {
					timeSinceLastEvent := time.Since(lastEventTime)
					if timeSinceLastEvent > inactivityTimeout {
						if c.debug {
							fmt.Printf("[DEBUG] SSE stream timeout after %v of inactivity (configured: %v)\n", timeSinceLastEvent, inactivityTimeout)
						}
						// Add Sentry tracking for stream timeouts
						sentryutil.CaptureError(fmt.Errorf("SSE stream inactivity timeout"), map[string]string{
							"timeout_type":       "sse_inactivity",
							"configured_timeout": inactivityTimeout.String(),
							"actual_inactivity":  timeSinceLastEvent.String(),
						}, map[string]interface{}{
							"runner":             runnerName,
							"inactivity_minutes": timeSinceLastEvent.Minutes(),
						})
						resp.Body.Close() // Force close the connection
						return
					}
				}
			}()
		}

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					if c.debug {
						fmt.Printf("[DEBUG] Read error: %v\n", err)
					}
					events <- WorkflowSSEEvent{Type: "error", Data: err.Error()}
				}
				return
			}

			// Update last event time - we received data
			lastEventTime = time.Now()

			line = strings.TrimSpace(line)

			// Debug logging
			if c.debug {
				fmt.Printf("[DEBUG] SSE Line: %s\n", line)
			}

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse SSE format
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				// Check if it's the end marker
				if data == "[DONE]" || data == "end of stream" {
					events <- WorkflowSSEEvent{Type: "done", Data: ""}
					return
				}

				// Send data event
				events <- WorkflowSSEEvent{Type: "data", Data: data}
			} else if strings.HasPrefix(line, "event: ") {
				eventType := strings.TrimPrefix(line, "event: ")

				// Handle close event specially
				if eventType == "close" {
					events <- WorkflowSSEEvent{Type: "done", Data: ""}
					return
				}

				// Next line should contain the data
				if dataLine, err := reader.ReadString('\n'); err == nil {
					dataLine = strings.TrimSpace(dataLine)
					if strings.HasPrefix(dataLine, "data: ") {
						data := strings.TrimPrefix(dataLine, "data: ")
						events <- WorkflowSSEEvent{Type: eventType, Data: data}
						lastEventTime = time.Now() // Update on data received
					}
				}
			}
		}
	}()

	return events, nil
}

// GitHubIntegration represents a GitHub integration response
type GitHubIntegration struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// GetGitHubToken retrieves GitHub access token from integrations API
func (c *Client) GetGitHubToken(ctx context.Context) (string, error) {
	resp, err := c.get(ctx, "/api/v2/integrations/github_app")
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub integration: %w", err)
	}
	defer resp.Body.Close()

	var integrations []GitHubIntegration
	if err := json.NewDecoder(resp.Body).Decode(&integrations); err != nil {
		return "", fmt.Errorf("failed to decode GitHub integration response: %w", err)
	}

	// Find the first active GitHub integration
	for _, integration := range integrations {
		if integration.Type == "github" && integration.Status == "active" && integration.AccessToken != "" {
			return integration.AccessToken, nil
		}
	}

	return "", fmt.Errorf("no active GitHub integration found")
}
