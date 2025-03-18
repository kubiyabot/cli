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
	"strings"
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
}

// NewClient creates a new Kubiya API client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:     cfg,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: cfg.BaseURL,
		debug:   cfg.Debug,
		cache:   NewCache(5 * time.Minute), // If you have a cache defined elsewhere
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

	// Filter out empty entries
	var validTeammates []Teammate
	for _, t := range teammates {
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
	resp, err := c.get(ctx, "/runners")
	if err != nil {
		return nil, fmt.Errorf("failed to list runners: %w", err)
	}
	defer resp.Body.Close()

	var runners []Runner
	if err := json.NewDecoder(resp.Body).Decode(&runners); err != nil {
		// If array decode fails, try single object
		resp.Body.Close()
		resp, err = c.get(ctx, "/runners")
		if err != nil {
			return nil, fmt.Errorf("failed to list runners: %w", err)
		}
		defer resp.Body.Close()

		var runner Runner
		if err := json.NewDecoder(resp.Body).Decode(&runner); err != nil {
			return nil, fmt.Errorf("failed to decode runners response: %w", err)
		}
		runners = []Runner{runner}
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

// Example method: retrieve a runner's manifest
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

// Example method: updating a teammate
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
