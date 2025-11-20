package controlplane

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// ExecuteAgentV2 creates a new agent execution (V2 API)
func (c *Client) ExecuteAgentV2(agentID string, req *entities.ExecuteAgentRequest) (*entities.AgentExecution, error) {
	var execution entities.AgentExecution
	if err := c.post(fmt.Sprintf("/api/v1/agents/%s/execute", agentID), req, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// ExecuteTeamV2 creates a new team execution (V2 API)
func (c *Client) ExecuteTeamV2(teamID string, req *entities.ExecuteTeamRequest) (*entities.AgentExecution, error) {
	var execution entities.AgentExecution
	if err := c.post(fmt.Sprintf("/api/v1/teams/%s/execute", teamID), req, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetExecution retrieves an execution by ID
// Note: The API returns an array with a single element
func (c *Client) GetExecution(id string) (*entities.AgentExecution, error) {
	var executions []*entities.AgentExecution
	if err := c.get(fmt.Sprintf("/api/v1/executions/%s", id), &executions); err != nil {
		return nil, err
	}
	if len(executions) == 0 {
		return nil, fmt.Errorf("execution not found: %s", id)
	}
	return executions[0], nil
}

// ListExecutions lists all executions with optional filters
func (c *Client) ListExecutions(filters map[string]string) ([]*entities.AgentExecution, error) {
	var executions []*entities.AgentExecution
	endpoint := "/api/v1/executions"

	// Add query parameters if filters provided
	if len(filters) > 0 {
		endpoint += "?"
		first := true
		for k, v := range filters {
			if !first {
				endpoint += "&"
			}
			endpoint += fmt.Sprintf("%s=%s", k, v)
			first = false
		}
	}

	if err := c.get(endpoint, &executions); err != nil {
		return nil, err
	}
	return executions, nil
}

// StreamExecutionOutput streams the execution output via WebSocket
// Returns a channel that emits streaming events and an error channel
func (c *Client) StreamExecutionOutput(ctx context.Context, executionID string) (<-chan entities.StreamEvent, <-chan error) {
	eventChan := make(chan entities.StreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		// Try WebSocket endpoint first
		wsURL := fmt.Sprintf("%s/api/v1/executions/%s/stream", c.BaseURL, executionID)
		wsURL = "wss" + wsURL[4:] // Replace http/https with ws/wss

		// For now, fall back to SSE polling endpoint
		// TODO: Implement WebSocket support
		sseURL := fmt.Sprintf("%s/api/v1/executions/%s/events", c.BaseURL, executionID)

		req, err := http.NewRequest("GET", sseURL, nil)
		if err != nil {
			errChan <- fmt.Errorf("failed to create stream request: %w", err)
			return
		}

		// Set headers for SSE
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Authorization", "Bearer "+c.APIKey)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to connect to stream: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			errChan <- fmt.Errorf("stream returned status %d", resp.StatusCode)
			return
		}

		// Read SSE stream
		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						errChan <- fmt.Errorf("error reading stream: %w", err)
					}
					return
				}

				// Parse SSE format
				if len(line) > 6 && line[:6] == "data: " {
					data := line[6 : len(line)-1] // Remove "data: " prefix and trailing newline

					var event entities.StreamEvent
					if err := json.Unmarshal([]byte(data), &event); err != nil {
						// Skip malformed events
						continue
					}

					select {
					case eventChan <- event:
					case <-ctx.Done():
						return
					}

					// Stop streaming on complete or error
					if event.Type == "complete" || event.Type == "error" {
						return
					}
				}
			}
		}
	}()

	return eventChan, errChan
}
