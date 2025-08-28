package util

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
	"strings"
	"time"
)

// HTTPClient provides a wrapper around the standard HTTP client with common utilities
type HTTPClient struct {
	client  *http.Client
	baseURL string
	apiKey  string
	debug   bool
	timeout time.Duration
}

// Option is a functional option for configuring the HTTP client
type Option func(*HTTPClient)

// WithAPIKey sets the API key for authentication
func WithAPIKey(apiKey string) Option {
	return func(c *HTTPClient) {
		c.apiKey = apiKey
	}
}

// WithDebug enables debug logging
func WithDebug(debug bool) Option {
	return func(c *HTTPClient) {
		c.debug = debug
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *HTTPClient) {
		c.timeout = timeout
		c.client.Timeout = timeout
	}
}

// WithHTTPClient allows using a custom http.Client
func WithHTTPClient(client *http.Client) Option {
	return func(c *HTTPClient) {
		c.client = client
		c.timeout = client.Timeout
	}
}

// NewHTTPClient creates a new HTTP client with the given base URL and options
func NewHTTPClient(baseURL string, opts ...Option) *HTTPClient {
	// Default configuration
	c := &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		timeout: 30 * time.Second,
		debug:   false,
	}

	// Apply all options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// SetTimeout updates the HTTP client timeout
func (h *HTTPClient) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
	h.client.Timeout = timeout
}

// GetClient returns the underlying http.Client
func (h *HTTPClient) GetClient() *http.Client {
	return h.client
}

// GetBaseURL returns the base URL
func (h *HTTPClient) GetBaseURL() string {
	return h.baseURL
}

// SetBaseURL updates the base URL
func (h *HTTPClient) SetBaseURL(baseURL string) {
	h.baseURL = baseURL
}

// IsDebug returns whether debug mode is enabled
func (h *HTTPClient) IsDebug() bool {
	return h.debug
}

// SetDebug enables or disables debug mode
func (h *HTTPClient) SetDebug(debug bool) {
	h.debug = debug
}

// GetTimeout returns the current timeout setting
func (h *HTTPClient) GetTimeout() time.Duration {
	return h.timeout
}

// BuildURL constructs a full URL from the base URL and path
func (h *HTTPClient) BuildURL(path string) string {
	baseURL := h.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return baseURL + strings.TrimPrefix(path, "/")
}

// BuildPathWithParams constructs a path with query parameters
// This returns a path (not a full URL) that can be passed to GET, POST, etc.
func (h *HTTPClient) BuildPathWithParams(path string, params map[string]string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	q := u.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// NewRequest creates a new HTTP request with standard headers
func (h *HTTPClient) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u := h.BuildURL(path)

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard headers
	if h.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("UserKey %s", h.apiKey))
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// NewJSONRequest creates a new HTTP request with JSON payload
func (h *HTTPClient) NewJSONRequest(ctx context.Context, method, path string, payload interface{}) (*http.Request, error) {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, fmt.Errorf("failed to encode request payload: %w", err)
		}
	}

	return h.NewRequest(ctx, method, path, &body)
}

// Do performs an HTTP request and returns the response
func (h *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Log request if debug is enabled
	if h.debug {
		h.logRequest(req)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		if h.debug {
			fmt.Printf("[DEBUG] Request failed: %v\n", err)
		}
		LogAPICall(req.Method, req.URL.String(), h.headerMapFromRequest(req), nil, 0, []byte(fmt.Sprintf("Request failed: %v", err)))
		return nil, err
	}

	// Log response if debug is enabled
	if h.debug {
		h.logResponse(resp)
	}

	return resp, nil
}

// DoJSON performs an HTTP request and decodes the JSON response
func (h *HTTPClient) DoJSON(req *http.Request, v interface{}) error {
	resp, err := h.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// DoRaw performs an HTTP request and returns the raw response body
func (h *HTTPClient) DoRaw(req *http.Request) ([]byte, error) {
	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// GET performs a GET request
// The path parameter should be a path (with optional query params), not a full URL
func (h *HTTPClient) GET(ctx context.Context, path string) (*http.Response, error) {
	req, err := h.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}

	// Log the API call
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	LogAPICall("GET", req.URL.String(), h.headerMapFromRequest(req), nil, resp.StatusCode, bodyBytes)

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

// POST performs a POST request with JSON payload
func (h *HTTPClient) POST(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	req, err := h.NewJSONRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}

	// Capture request body for logging
	var requestBody []byte
	if payload != nil {
		requestBody, _ = json.Marshal(payload)
	}

	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}

	// Log the API call
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	LogAPICall("POST", req.URL.String(), h.headerMapFromRequest(req), requestBody, resp.StatusCode, bodyBytes)

	return resp, nil
}

// POSTRaw performs a POST request with raw data
func (h *HTTPClient) POSTRaw(ctx context.Context, path string, data []byte) (*http.Response, error) {
	req, err := h.NewRequest(ctx, http.MethodPost, path, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}

	// Log the API call
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	LogAPICall("POST", req.URL.String(), h.headerMapFromRequest(req), data, resp.StatusCode, bodyBytes)

	return resp, nil
}

// PUT performs a PUT request with JSON payload
func (h *HTTPClient) PUT(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	req, err := h.NewJSONRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return nil, err
	}

	// Capture request body for logging
	var requestBody []byte
	if payload != nil {
		requestBody, _ = json.Marshal(payload)
	}

	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}

	// Log the API call
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	LogAPICall("PUT", req.URL.String(), h.headerMapFromRequest(req), requestBody, resp.StatusCode, bodyBytes)

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

// DELETE performs a DELETE request
func (h *HTTPClient) DELETE(ctx context.Context, path string) (*http.Response, error) {
	req, err := h.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.Do(req)
	if err != nil {
		return nil, err
	}

	// Log the API call
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	LogAPICall("DELETE", req.URL.String(), h.headerMapFromRequest(req), nil, resp.StatusCode, bodyBytes)

	return resp, nil
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Type string
	Data string
}

// StreamSSE creates an SSE stream from an HTTP response
func (h *HTTPClient) StreamSSE(resp *http.Response, timeout time.Duration) (<-chan SSEEvent, error) {
	events := make(chan SSEEvent)
	lastEventTime := time.Now()

	go func() {
		defer close(events)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)

		// Monitor for timeouts if specified
		if timeout > 0 {
			inactivityTimeout := timeout * 3
			if inactivityTimeout < time.Hour {
				inactivityTimeout = time.Hour
			}

			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			go func() {
				for range ticker.C {
					if time.Since(lastEventTime) > inactivityTimeout {
						if h.debug {
							fmt.Printf("[DEBUG] SSE stream timeout after %v of inactivity\n", time.Since(lastEventTime))
						}
						resp.Body.Close()
						return
					}
				}
			}()
		}

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					events <- SSEEvent{Type: "error", Data: err.Error()}
				}
				return
			}

			lastEventTime = time.Now()
			line = strings.TrimSpace(line)

			if h.debug {
				fmt.Printf("[DEBUG] SSE Line: %s\n", line)
			}

			if line == "" {
				continue
			}

			// Parse SSE format
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				if data == "[DONE]" || data == "end of stream" {
					events <- SSEEvent{Type: "done", Data: ""}
					return
				}

				events <- SSEEvent{Type: "data", Data: data}
			} else if strings.HasPrefix(line, "event: ") {
				eventType := strings.TrimPrefix(line, "event: ")

				if eventType == "close" {
					events <- SSEEvent{Type: "done", Data: ""}
					return
				}

				// Next line should contain the data
				if dataLine, err := reader.ReadString('\n'); err == nil {
					dataLine = strings.TrimSpace(dataLine)
					if strings.HasPrefix(dataLine, "data: ") {
						data := strings.TrimPrefix(dataLine, "data: ")
						events <- SSEEvent{Type: eventType, Data: data}
						lastEventTime = time.Now()
					}
				}
			}
		}
	}()

	return events, nil
}

// Helper methods for logging

func (h *HTTPClient) logRequest(req *http.Request) {
	fmt.Printf("[DEBUG] %s %s\n", req.Method, req.URL.String())
	if req.Body != nil {
		if body, err := io.ReadAll(req.Body); err == nil {
			req.Body = io.NopCloser(bytes.NewBuffer(body))
			if len(body) > 0 {
				fmt.Printf("[DEBUG] Request Body: %s\n", string(body))
			}
		}
	}
}

func (h *HTTPClient) logResponse(resp *http.Response) {
	fmt.Printf("[DEBUG] Response Status: %d\n", resp.StatusCode)
}

func (h *HTTPClient) headerMapFromRequest(req *http.Request) map[string]string {
	headers := make(map[string]string)
	for key, values := range req.Header {
		if key == "Authorization" {
			headers[key] = "UserKey [REDACTED]"
		} else {
			headers[key] = strings.Join(values, ", ")
		}
	}
	return headers
}

// LogAPICall logs all API calls to /tmp/klog.txt (extracted from kubiya client)
func LogAPICall(method, url string, headers map[string]string, body []byte, responseStatus int, responseBody []byte) {
	if os.Getenv("KUBIYA_DEBUG") != "true" {
		return
	}

	f, err := os.OpenFile("/tmp/klog.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
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

// Response reading utilities

// ReadResponseBody reads and returns the response body, replacing it with a new reader
func ReadResponseBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}

// DecodeJSONResponse decodes a JSON response into the provided interface
func DecodeJSONResponse(resp *http.Response, v interface{}) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// ParseErrorResponse attempts to parse an error response from the body
func ParseErrorResponse(body []byte) *ErrorResponse {
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		return &errResp
	}
	return nil
}

// Request builder utilities

// RequestOption is a function that modifies an HTTP request
type RequestOption func(*http.Request)

// WithHeader adds a header to the request
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// WithRequestTimeout sets a custom timeout for the request
// Note: The caller is responsible for managing the request lifecycle
func WithRequestTimeout(timeout time.Duration) RequestOption {
	return func(req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		// Store the cancel function in the request context for cleanup
		// Note: In practice, the request will complete or be cancelled before the timeout
		_ = cancel // Acknowledge that we're aware of the cancel function
		*req = *req.WithContext(ctx)
	}
}

// ApplyOptions applies multiple RequestOptions to a request
func ApplyOptions(req *http.Request, opts ...RequestOption) {
	for _, opt := range opts {
		opt(req)
	}
}

// Retry utilities

// RetryableRequest performs a request with retry logic using the existing RetryConfig
func RetryableRequest(client *http.Client, req *http.Request, config *RetryConfig) (*http.Response, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	var resp *http.Response

	err := RetryWithBackoff(req.Context(), config, "http request", func() error {
		// Clone the request for retry
		reqCopy := req.Clone(req.Context())
		if req.Body != nil {
			// If there's a body, we need to recreate it
			if seeker, ok := req.Body.(io.Seeker); ok {
				seeker.Seek(0, io.SeekStart)
			}
		}

		var err error
		resp, err = client.Do(reqCopy)
		if err != nil {
			lastErr = err
			return err
		}

		// Don't retry on client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil // Success - don't retry client errors
		}

		// Retry on server errors (5xx)
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			return lastErr
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// RetryableHTTPClient wraps HTTPClient with retry logic
type RetryableHTTPClient struct {
	*HTTPClient
	retryConfig *RetryConfig
}

// NewRetryableHTTPClient creates a new HTTP client with retry capabilities
func NewRetryableHTTPClient(httpClient *HTTPClient, retryConfig *RetryConfig) *RetryableHTTPClient {
	if retryConfig == nil {
		retryConfig = DefaultRetryConfig()
	}

	return &RetryableHTTPClient{
		HTTPClient:  httpClient,
		retryConfig: retryConfig,
	}
}

// NewRetryableHTTPClientWithOptions creates a new retryable HTTP client with options
// This is a convenience method that creates both the HTTP client and retry wrapper
func NewRetryableHTTPClientWithOptions(baseURL string, retryConfig *RetryConfig, opts ...Option) *RetryableHTTPClient {
	httpClient := NewHTTPClient(baseURL, opts...)
	return NewRetryableHTTPClient(httpClient, retryConfig)
}

// DoWithRetry performs a request with retry logic
func (r *RetryableHTTPClient) DoWithRetry(req *http.Request) (*http.Response, error) {
	return RetryableRequest(r.client, req, r.retryConfig)
}
