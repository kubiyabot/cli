package controlplane

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

// StreamExecutionOutput streams the execution output via SSE
// Returns a channel that emits streaming events and an error channel
func (c *Client) StreamExecutionOutput(ctx context.Context, executionID string) (<-chan entities.StreamEvent, <-chan error) {
	eventChan := make(chan entities.StreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		sseURL := fmt.Sprintf("%s/api/v1/executions/%s/stream", c.BaseURL, executionID)

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

		// Use a client with no timeout for SSE streaming
		streamClient := &http.Client{
			Timeout: 0, // No timeout for SSE streams
		}
		resp, err := streamClient.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to connect to stream: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			errChan <- fmt.Errorf("stream returned status %d", resp.StatusCode)
			return
		}

		// Read SSE stream - parse proper SSE format with id/event/data fields
		reader := bufio.NewReader(resp.Body)
		var currentEventType string
		var currentEventID string

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

				line = strings.TrimRight(line, "\r\n")

				// Parse SSE fields
				if strings.HasPrefix(line, "id: ") {
					currentEventID = line[4:]
				} else if strings.HasPrefix(line, "event: ") {
					currentEventType = line[7:]
				} else if strings.HasPrefix(line, "data: ") {
					data := line[6:]

					// Parse the event using the event type from "event:" field
					event := parseSSEEvent(currentEventType, currentEventID, data)

					select {
					case eventChan <- event:
					case <-ctx.Done():
						return
					}

					// Stop streaming on terminal events
					if event.IsTerminalEvent() {
						return
					}

					// Reset for next event
					currentEventType = ""
					currentEventID = ""
				} else if line == "" {
					// Empty line marks end of event - reset state
					currentEventType = ""
					currentEventID = ""
				} else if strings.HasPrefix(line, ":") {
					// SSE comment (e.g., ": keepalive") - ignore
					continue
				}
			}
		}
	}()

	return eventChan, errChan
}

// parseSSEEvent parses an SSE event with proper handling of nested data structures
// The backend sends events in the format:
//
//	id: exec-123_1_1702938457123456
//	event: tool_started
//	data: {"data": {"tool_name": "...", "tool_execution_id": "..."}, "timestamp": "..."}
func parseSSEEvent(eventType, eventID, data string) entities.StreamEvent {
	event := entities.StreamEvent{
		Type: eventType,
	}

	// Parse the JSON data
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &rawData); err != nil {
		// If parsing fails, treat as content
		event.Content = data
		return event
	}

	// Set timestamp if present at top level
	if ts, ok := rawData["timestamp"].(string); ok {
		t, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			event.Timestamp = &entities.CustomTime{Time: t}
		}
	}

	// Handle different event types based on the "event:" field
	switch eventType {
	case "connected":
		// Connected event: {"execution_id": "...", "organization_id": "...", "status": "...", "connected_at": ...}
		event.Type = entities.StreamEventTypeConnected
		if execID, ok := rawData["execution_id"].(string); ok {
			if event.Metadata == nil {
				event.Metadata = make(map[string]interface{})
			}
			event.Metadata["execution_id"] = execID
		}

	case "message":
		// Message event: {role, content, timestamp, message_id, ...}
		event.Type = entities.StreamEventTypeMessage
		if role, ok := rawData["role"].(string); ok {
			event.Role = role
		}
		if content, ok := rawData["content"].(string); ok {
			event.Content = content
		}

	case "message_chunk":
		// Message chunk: {"data": {"content": "...", "message_id": "..."}, "timestamp": "..."}
		// OR: {"message": {"role": "...", "content": "...", "chunk": true}, ...}
		event.Type = entities.StreamEventTypeMessageChunk

		// Try nested "data" structure first (backend format)
		if nestedData, ok := rawData["data"].(map[string]interface{}); ok {
			if content, ok := nestedData["content"].(string); ok {
				event.Content = content
			}
		}
		// Try "message" structure (alternate format)
		if msg, ok := rawData["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].(string); ok {
				event.Content = content
			}
			if role, ok := msg["role"].(string); ok {
				event.Role = role
			}
			if chunk, ok := msg["chunk"].(bool); ok {
				event.Chunk = &chunk
			}
		}
		// Fallback to top-level content
		if event.Content == "" {
			if content, ok := rawData["content"].(string); ok {
				event.Content = content
			}
		}

	case "tool_started":
		// Tool started: {"data": {"tool_name": "...", "tool_execution_id": "...", "tool_input": {...}}, "timestamp": "..."}
		event.Type = entities.StreamEventTypeToolStarted
		if nestedData, ok := rawData["data"].(map[string]interface{}); ok {
			if name, ok := nestedData["tool_name"].(string); ok {
				event.ToolName = name
			}
			if inputs, ok := nestedData["tool_input"].(map[string]interface{}); ok {
				event.ToolInputs = inputs
			}
			if inputs, ok := nestedData["tool_arguments"].(map[string]interface{}); ok {
				event.ToolInputs = inputs
			}
		}

	case "tool_completed":
		// Tool completed: {"data": {"tool_name": "...", "tool_execution_id": "...", "tool_output": {...}, "tool_status": "..."}, "timestamp": "..."}
		event.Type = entities.StreamEventTypeToolCompleted
		if nestedData, ok := rawData["data"].(map[string]interface{}); ok {
			if name, ok := nestedData["tool_name"].(string); ok {
				event.ToolName = name
			}
			if outputs, ok := nestedData["tool_output"].(map[string]interface{}); ok {
				event.ToolOutputs = outputs
			}
			if status, ok := nestedData["tool_status"].(string); ok {
				success := status == "completed" || status == "success"
				event.Success = &success
			}
			if status, ok := nestedData["status"].(string); ok {
				success := status == "completed" || status == "success"
				event.Success = &success
			}
		}

	case "status":
		// Status event: {"status": "running", "execution_id": "..."}
		event.Type = entities.StreamEventTypeStatus
		if status, ok := rawData["status"].(string); ok {
			s := entities.AgentExecutionStatus(status)
			event.Status = &s
		}

	case "done":
		event.Type = entities.StreamEventTypeDone

	case "error":
		event.Type = entities.StreamEventTypeError
		if errMsg, ok := rawData["error"].(string); ok {
			event.Content = errMsg
		}

	case "history_complete":
		// History complete event - treat as a status update
		event.Type = entities.StreamEventTypeStatus

	case "keepalive":
		// Keepalive - ignore (handled as SSE comment)
		event.Type = "keepalive"

	default:
		// Unknown event type - preserve the type and try to extract content
		if event.Type == "" {
			// If no event type was set via "event:" line, try to get it from data
			if t, ok := rawData["type"].(string); ok {
				event.Type = t
			}
		}
		// Try to extract content from common fields
		if content, ok := rawData["content"].(string); ok {
			event.Content = content
		}
		if msg, ok := rawData["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].(string); ok {
				event.Content = content
			}
		}
	}

	// Store event ID in metadata for deduplication support
	if eventID != "" {
		if event.Metadata == nil {
			event.Metadata = make(map[string]interface{})
		}
		event.Metadata["event_id"] = eventID
	}

	return event
}
