package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client represents a control plane API client
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Debug      bool
}

// New creates a new Control Plane API client
// If baseURL is empty, it will use the default or environment variables
func New(apiKey string, debug bool) (*Client, error) {
	return NewWithURL(apiKey, "", debug)
}

// NewWithURL creates a new Control Plane API client with a specific base URL
// If baseURL is empty, it will check environment variables and use defaults
func NewWithURL(apiKey, baseURL string, debug bool) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Use provided URL or fall back to environment/default
	if baseURL == "" {
		baseURL = getBaseURL()
	}

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	client := &Client{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: httpClient,
		Debug:      debug,
	}

	if debug {
		fmt.Printf("Created Kubiya Control Plane client (base_url=%s)\n", baseURL)
	}

	return client, nil
}

// getBaseURL returns the base URL for the Control Plane API
// Checks environment variables with priority:
// 1. KUBIYA_CONTROL_PLANE_BASE_URL (standard)
// 2. CONTROL_PLANE_GATEWAY_URL (backward compatibility)
// 3. CONTROL_PLANE_URL (backward compatibility)
// 4. Default: https://control-plane.kubiya.ai
func getBaseURL() string {
	if customURL := os.Getenv("KUBIYA_CONTROL_PLANE_BASE_URL"); customURL != "" {
		return customURL
	}
	if customURL := os.Getenv("CONTROL_PLANE_GATEWAY_URL"); customURL != "" {
		return customURL
	}
	if customURL := os.Getenv("CONTROL_PLANE_URL"); customURL != "" {
		return customURL
	}
	return "https://control-plane.kubiya.ai"
}

// DoRequest performs an HTTP request with proper headers and error handling
func (c *Client) DoRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	var jsonBody []byte

	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	// Check if path is already a full URL (starts with http:// or https://)
	fullURL := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		fullURL = c.BaseURL + path
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	if c.Debug {
		fmt.Printf("[DEBUG] %s %s\n", method, fullURL)
		if len(jsonBody) > 0 {
			fmt.Printf("[DEBUG] Request body: %s\n", string(jsonBody))
		}
	}

	startTime := time.Now()
	resp, err := c.HTTPClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		if c.Debug {
			fmt.Printf("[ERROR] Request failed: %v\n", err)
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Response status: %d (took %dms)\n", resp.StatusCode, duration.Milliseconds())
	}

	// Log errors to file
	if resp.StatusCode >= 400 {
		c.logError(method, fullURL, resp.StatusCode, duration, jsonBody, resp)
	}

	return resp, nil
}

// ParseResponse parses the HTTP response into the provided interface
func (c *Client) ParseResponse(resp *http.Response, target interface{}) error {
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Response body: %s\n", string(bodyBytes))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	if target != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, target); err != nil {
			if c.Debug {
				fmt.Printf("[ERROR] Failed to parse response: %v\n", err)
			}
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// logError logs error details to a file for debugging
func (c *Client) logError(method, url string, statusCode int, duration time.Duration, requestBody []byte, resp *http.Response) {
	logFile := os.Getenv("KUBIYA_API_LOG_FILE")
	if logFile == "" {
		logFile = "/tmp/kubiya_api_errors.log"
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "\n========== API ERROR ==========\n")
	fmt.Fprintf(f, "Time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "Method: %s\n", method)
	fmt.Fprintf(f, "URL: %s\n", url)
	fmt.Fprintf(f, "Status Code: %d\n", statusCode)
	fmt.Fprintf(f, "Duration: %dms\n", duration.Milliseconds())

	if len(requestBody) > 0 {
		fmt.Fprintf(f, "\n--- Request Body ---\n%s\n", string(requestBody))
	}

	if resp != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			fmt.Fprintf(f, "\n--- Response Body ---\n%s\n", string(bodyBytes))
			// Reset body for further reading
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	fmt.Fprintf(f, "===============================\n\n")
}

// get performs a GET request
func (c *Client) get(path string, target interface{}) error {
	resp, err := c.DoRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.ParseResponse(resp, target)
}

// post performs a POST request
func (c *Client) post(path string, body interface{}, target interface{}) error {
	resp, err := c.DoRequest(http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return c.ParseResponse(resp, target)
}

// patch performs a PATCH request
func (c *Client) patch(path string, body interface{}, target interface{}) error {
	resp, err := c.DoRequest(http.MethodPatch, path, body)
	if err != nil {
		return err
	}
	return c.ParseResponse(resp, target)
}

// put performs a PUT request
func (c *Client) put(path string, body interface{}, target interface{}) error {
	resp, err := c.DoRequest(http.MethodPut, path, body)
	if err != nil {
		return err
	}
	return c.ParseResponse(resp, target)
}

// delete performs a DELETE request
func (c *Client) delete(path string) error {
	resp, err := c.DoRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return c.ParseResponse(resp, nil)
}

// getExternal performs a GET request to an external URL (not via control plane)
// Used for fetching data from external APIs like PyPI
func (c *Client) getExternal(url string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if c.Debug {
		fmt.Printf("[DEBUG] GET (external) %s\n", url)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] External response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] External response body: %s\n", string(bodyBytes))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("external API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	if target != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, target); err != nil {
			return fmt.Errorf("failed to parse external response: %w", err)
		}
	}

	return nil
}

// ClientConfig represents configuration for the CLI client
type ClientConfig struct {
	ContextGraphAPIBase string `json:"context_graph_api_base"`
	OrganizationID      string `json:"organization_id"`
	OrganizationName    string `json:"organization_name"`
}

// GetClientConfig fetches client configuration from the control plane
// This allows CLI to discover backend service URLs and connect directly
func (c *Client) GetClientConfig() (*ClientConfig, error) {
	var config ClientConfig
	err := c.get("/api/v1/client/config", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch client config: %w", err)
	}
	return &config, nil
}
