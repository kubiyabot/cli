package kubiya

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// KnowledgeClient handles knowledge-specific operations
type KnowledgeClient struct {
	client *Client
}

// NewKnowledgeClient creates a new knowledge client
func NewKnowledgeClient(client *Client) *KnowledgeClient {
	return &KnowledgeClient{client: client}
}

// Knowledge returns the knowledge client
func (c *Client) Knowledge() *KnowledgeClient {
	return NewKnowledgeClient(c)
}

// KnowledgeQueryRequest represents a request to query the knowledge base
type KnowledgeQueryRequest struct {
	Query          string `json:"query"`
	UserID         string `json:"userID,omitempty"`
	OrgID          string `json:"orgID,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

// KnowledgeQueryResponse represents the JSON response from the query endpoint
type KnowledgeQueryResponse struct {
	SessionID    string                   `json:"session_id"`
	Query        string                   `json:"query"`
	TotalResults int                      `json:"total_results"`
	Results      []map[string]interface{} `json:"results"`
	Timestamp    string                   `json:"timestamp"`
	OrgID        *string                  `json:"org_id"`
}

// KnowledgeSSEEvent represents a knowledge query SSE event
type KnowledgeSSEEvent struct {
	Type string
	Data string
}

// Query queries the central knowledge base using the orchestration API
func (kc *KnowledgeClient) Query(ctx context.Context, req KnowledgeQueryRequest) (<-chan KnowledgeSSEEvent, error) {
	// Use orchestrator API endpoint for knowledge queries
	orchestratorURL := os.Getenv("KUBIYA_ORCHESTRATOR_URL")
	if orchestratorURL == "" {
		// Check if we should use the same base URL as the main API
		if os.Getenv("KUBIYA_USE_SAME_API") == "true" {
			orchestratorURL = strings.TrimSuffix(kc.client.baseURL, "/api/v1") + "/api/query"
		} else {
			// Default to the orchestrator service URL
			orchestratorURL = "https://orchestrator.kubiya.ai/api/query"
		}
	}

	// Set default response format if not specified
	if req.ResponseFormat == "" {
		req.ResponseFormat = "vercel"
	}

	// Create request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, orchestratorURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	// Set the org_id header if provided (required by orchestrator API)
	if req.OrgID != "" {
		httpReq.Header.Set("x-org-id", req.OrgID)
	}

	// Always use UserKey format for API key authentication
	// The orchestrator expects UserKey format for Kubiya API keys (even if they are JWTs)

	// Execute request with a custom client with longer timeout
	// Use the authenticated transport from the main client
	queryClient := &http.Client{
		Timeout:   5 * time.Minute,        // Longer timeout for knowledge queries
		Transport: kc.client.client.Transport, // Reuse authenticated transport
	}
	resp, err := queryClient.Do(httpReq)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, fmt.Errorf("knowledge base query timeout. The service might be unavailable or slow. Please try again later")
		}
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("knowledge query failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Check content type to determine response format
	contentType := resp.Header.Get("Content-Type")

	// Create channel for streaming events
	events := make(chan KnowledgeSSEEvent)

	if strings.Contains(contentType, "application/json") {
		// Handle JSON response
		go func() {
			defer close(events)
			defer resp.Body.Close()

			var queryResp KnowledgeQueryResponse
			if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
				events <- KnowledgeSSEEvent{Type: "error", Data: fmt.Sprintf("Failed to decode response: %v", err)}
				return
			}

			// Convert JSON response to events with clean formatting
			if queryResp.TotalResults == 0 {
				events <- KnowledgeSSEEvent{Type: "data", Data: "\n❌ No results found in the knowledge base.\n"}
			} else {
				events <- KnowledgeSSEEvent{Type: "data", Data: fmt.Sprintf("✅ Found %d results\n\n", queryResp.TotalResults)}

				// Format results in a clean, readable way
				for i, result := range queryResp.Results {
					var output strings.Builder
					output.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"))
					output.WriteString(fmt.Sprintf("📄 Result %d\n", i+1))
					output.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"))

					// Extract and display key information
					if data, ok := result["data"].(map[string]interface{}); ok {
						// Content
						if content, ok := data["chunk_content"].(string); ok && content != "" {
							output.WriteString(fmt.Sprintf("📝 Content:\n   %s\n\n", content))
						}

						// Relevance score
						if score, ok := result["score"].(float64); ok {
							percentage := score * 100
							output.WriteString(fmt.Sprintf("⭐ Relevance: %.1f%%\n\n", percentage))
						}

						// Source metadata (if available)
						if metadataStr, ok := data["metadata"].(string); ok {
							var metadata map[string]interface{}
							if err := json.Unmarshal([]byte(metadataStr), &metadata); err == nil {
								// Channel info
								if channelName, ok := metadata["channel_name"].(string); ok {
									output.WriteString(fmt.Sprintf("📍 Source: #%s", channelName))

									// User info
									if user, ok := metadata["user"].(string); ok {
										output.WriteString(fmt.Sprintf(" (by %s)", user))
									}
									output.WriteString("\n")
								}

								// Timestamp
								if ts, ok := metadata["ts"].(string); ok {
									output.WriteString(fmt.Sprintf("🕐 Timestamp: %s\n", ts))
								}
							}
						}

						// Database info
						if db, ok := result["database"].(string); ok {
							output.WriteString(fmt.Sprintf("💾 Database: %s\n", db))
						}
					}

					output.WriteString("\n")
					events <- KnowledgeSSEEvent{Type: "data", Data: output.String()}
				}
			}

			events <- KnowledgeSSEEvent{Type: "done", Data: ""}
		}()
	} else {
		// Handle SSE streaming response
		go func() {
			defer close(events)
			defer resp.Body.Close()

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()

				// Debug logging
				if kc.client.debug {
					fmt.Printf("[DEBUG] SSE Line: %s\n", line)
				}

				// Skip empty lines and retry messages
				if line == "" || strings.HasPrefix(line, "retry:") {
					continue
				}

				// Parse SSE format - the actual format is "0:data" or "2:{json}" or "d:{json}"
				if len(line) > 2 && line[1] == ':' {
					eventType := string(line[0])
					data := line[2:]

					switch eventType {
					case "0": // Text data
						events <- KnowledgeSSEEvent{Type: "data", Data: data}
					case "2": // JSON data event
						events <- KnowledgeSSEEvent{Type: "data", Data: data}
					case "3": // Error event with details
						events <- KnowledgeSSEEvent{Type: "data", Data: data}
					case "d": // Done event
						events <- KnowledgeSSEEvent{Type: "done", Data: data}
					case "e": // Error event
						events <- KnowledgeSSEEvent{Type: "error", Data: data}
					case "f": // Message ID or metadata
						// Skip metadata events for now
						continue
					default:
						// Unknown event type, treat as data
						events <- KnowledgeSSEEvent{Type: "data", Data: data}
					}
				} else if strings.HasPrefix(line, "data: ") {
					// Standard SSE format
					data := strings.TrimPrefix(line, "data: ")
					events <- KnowledgeSSEEvent{Type: "data", Data: data}
				} else if strings.HasPrefix(line, "event: ") {
					// Standard SSE event type
					eventType := strings.TrimPrefix(line, "event: ")
					events <- KnowledgeSSEEvent{Type: eventType, Data: ""}
				}
			}

			if err := scanner.Err(); err != nil {
				if kc.client.debug {
					fmt.Printf("[DEBUG] Scanner error: %v\n", err)
				}
				events <- KnowledgeSSEEvent{Type: "error", Data: err.Error()}
			}
		}()
	}

	return events, nil
}

// Legacy methods below for backward compatibility

// Knowledge-related client methods
func (c *Client) ListKnowledge(ctx context.Context, query string, limit int) ([]Knowledge, error) {
	url := fmt.Sprintf("%s/knowledge", c.cfg.BaseURL)

	// Add query parameters
	if query != "" || limit > 0 {
		params := make([]string, 0)
		if query != "" {
			params = append(params, fmt.Sprintf("query=%s", query))
		}
		if limit > 0 {
			params = append(params, fmt.Sprintf("limit=%d", limit))
		}
		if len(params) > 0 {
			url += "?" + fmt.Sprintf("%s", params[0])
			for i := 1; i < len(params); i++ {
				url += "&" + params[i]
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var items []Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return items, nil
}

func (c *Client) SearchKnowledge(ctx context.Context, query string, limit int) ([]Knowledge, error) {
	url := fmt.Sprintf("%s/knowledge/search?query=%s", c.cfg.BaseURL, query)
	if limit > 0 {
		url += fmt.Sprintf("&limit=%d", limit)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var items []Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return items, nil
}

func (c *Client) GetKnowledge(ctx context.Context, uuid string) (*Knowledge, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/knowledge/%s", c.cfg.BaseURL, uuid), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var item Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	return &item, nil
}

func (c *Client) CreateKnowledge(ctx context.Context, item Knowledge) (*Knowledge, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/knowledge", c.cfg.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	//req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var created Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, err
	}

	return &created, nil
}

func (c *Client) UpdateKnowledge(ctx context.Context, uuid string, item Knowledge) (*Knowledge, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/knowledge/%s", c.cfg.BaseURL, uuid), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	//req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var updated Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, err
	}

	return &updated, nil
}

func (c *Client) DeleteKnowledge(ctx context.Context, uuid string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/knowledge/%s", c.cfg.BaseURL, uuid), nil)
	if err != nil {
		return err
	}

	//req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
