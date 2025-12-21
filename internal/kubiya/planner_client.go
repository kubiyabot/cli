package kubiya

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

// PlannerClient wraps the Kubiya client to provide planning capabilities
type PlannerClient struct {
	client *Client
}

// NewPlannerClient creates a new planner client
func NewPlannerClient(client *Client) *PlannerClient {
	return &PlannerClient{
		client: client,
	}
}

// CreatePlan creates a task plan (non-streaming)
// NOTE: Planning can take 60-120 seconds due to AI processing, so we use an extended timeout
func (pc *PlannerClient) CreatePlan(ctx context.Context, req *PlanRequest) (*PlanResponse, error) {
	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request to non-streaming endpoint
	planURL := fmt.Sprintf("%s/api/v1/tasks/plan", pc.client.GetBaseURL())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, planURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if pc.client.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("UserKey %s", pc.client.cfg.APIKey))
	}

	// Use extended timeout client for planning (3 minutes)
	// Default 30s timeout is not enough for AI planning which takes 40-120 seconds
	httpClient := &http.Client{
		Timeout: 3 * time.Minute,
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("plan creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Plan PlanResponse `json:"plan"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode plan response: %w", err)
	}

	return &result.Plan, nil
}

// StreamPlanProgress streams plan generation with SSE events
func (pc *PlannerClient) StreamPlanProgress(ctx context.Context, req *PlanRequest) (<-chan PlanStreamEvent, <-chan error) {
	eventChan := make(chan PlanStreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		// Marshal request
		reqBody, err := json.Marshal(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		// Create SSE request
		sseURL := fmt.Sprintf("%s/api/v1/tasks/plan/stream", pc.client.GetBaseURL())
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, sseURL, bytes.NewBuffer(reqBody))
		if err != nil {
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		// Set SSE headers
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Cache-Control", "no-cache")
		httpReq.Header.Set("Connection", "keep-alive")

		// IMPORTANT: Add Authorization header (required for backend)
		// The backend expects: "Authorization: Bearer <token>" or "Authorization: UserKey <token>"
		if pc.client.cfg.APIKey != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("UserKey %s", pc.client.cfg.APIKey))
		}

		// Execute request with no timeout for streaming
		// Note: We can't use pc.client.client here because it has a 30s timeout
		// SSE streams need no timeout, but we need the auth header added above
		httpClient := &http.Client{
			Timeout: 0, // No timeout for SSE streams
		}
		resp, err := httpClient.Do(httpReq)
		if err != nil {
			errChan <- fmt.Errorf("failed to execute request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
			return
		}

		// Parse SSE stream
		reader := bufio.NewReader(resp.Body)
		var currentEvent PlanStreamEvent

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return // Stream completed
				}
				errChan <- fmt.Errorf("stream read error: %w", err)
				return
			}

			line = strings.TrimSpace(line)

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse SSE format: "event: <type>" and "data: <json>"
			if strings.HasPrefix(line, "event: ") {
				// Parse event type
				eventType := strings.TrimSpace(line[7:])
				currentEvent.Type = eventType

			} else if strings.HasPrefix(line, "data: ") {
				// Parse data
				data := strings.TrimSpace(line[6:])

				// Check for stream end markers
				if data == "[DONE]" || data == "end of stream" {
					return
				}

				// Parse JSON data into map
				var dataMap map[string]interface{}
				if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
					// If not valid JSON, store as raw string
					currentEvent.Data = map[string]interface{}{
						"content": data,
					}
				} else {
					currentEvent.Data = dataMap
				}

				// Send event
				select {
				case eventChan <- currentEvent:
					// Event sent successfully
				case <-ctx.Done():
					return
				}

				// Handle complete event - continue reading until EOF
				if currentEvent.Type == "complete" {
					// Give receiver time to process the complete event before closing channel
					// Sleep briefly to ensure the event is consumed from the channel
					time.Sleep(100 * time.Millisecond)
				}

				// Reset for next event
				currentEvent = PlanStreamEvent{}
			}
		}
	}()

	return eventChan, errChan
}

// GetPlan retrieves an existing plan by ID
func (pc *PlannerClient) GetPlan(ctx context.Context, planID string) (*PlanResponse, error) {
	path := fmt.Sprintf("/api/v1/tasks/plan/%s", planID)
	resp, err := pc.client.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get plan failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Plan PlanResponse `json:"plan"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode plan response: %w", err)
	}

	return &result.Plan, nil
}

// CheckPlannerHealth checks if the planner service is available
func (pc *PlannerClient) CheckPlannerHealth(ctx context.Context) error {
	resp, err := pc.client.get(ctx, "/api/v1/tasks/plan/health")
	if err != nil {
		return fmt.Errorf("planner health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("planner health check returned status %d", resp.StatusCode)
	}

	return nil
}

// WaitForPlanCompletion polls for plan completion (for non-streaming mode)
func (pc *PlannerClient) WaitForPlanCompletion(ctx context.Context, planID string, timeout time.Duration) (*PlanResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("timeout waiting for plan completion")
			}
			return nil, ctx.Err()

		case <-ticker.C:
			plan, err := pc.GetPlan(ctx, planID)
			if err != nil {
				continue // Retry on error
			}

			// Check if plan is complete (you may need to add a Status field to PlanResponse)
			// For now, return the plan as soon as we can retrieve it
			return plan, nil
		}
	}
}
