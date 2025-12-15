package contextgraph

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a direct client for the Context Graph API
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new context graph client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute, // Long timeout for streaming
		},
	}
}

// IntelligentSearchRequest represents a search request
type IntelligentSearchRequest struct {
	Keywords    string  `json:"keywords"`
	MaxTurns    int     `json:"max_turns,omitempty"`
	SessionID   string  `json:"session_id,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}

// StreamSearch executes a streaming intelligent search
func (c *Client) StreamSearch(ctx context.Context, req IntelligentSearchRequest) (<-chan StreamEvent, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := c.BaseURL + "/api/v1/graph/intelligent-search/stream"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	// Execute request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Create event channel
	events := make(chan StreamEvent, 10)

	// Start SSE parser goroutine
	go func() {
		defer close(events)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		var currentEvent string

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					events <- StreamEvent{
						Event: "error",
						Data:  map[string]interface{}{"message": err.Error()},
					}
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Parse SSE format
			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

				var rawData map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &rawData); err != nil {
					continue // Skip malformed data
				}

				// Check if the data contains nested event/data structure
				// Backend sends: {"event":"progress","data":{"message":"...","progress":5}}
				var eventType string
				var eventData map[string]interface{}

				if nestedEvent, ok := rawData["event"].(string); ok {
					// Nested format: use the inner event and data
					eventType = nestedEvent
					if nestedData, ok := rawData["data"].(map[string]interface{}); ok {
						eventData = nestedData
					} else {
						// No nested data, use rawData
						eventData = rawData
					}
				} else {
					// Simple format: use the SSE event line and rawData as-is
					eventType = currentEvent
					eventData = rawData
				}

				select {
				case events <- StreamEvent{Event: eventType, Data: eventData}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return events, nil
}
