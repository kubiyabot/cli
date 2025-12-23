package webui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// StreamEvent represents a normalized streaming event for the WebUI
type StreamEvent struct {
	Type       string                 `json:"type"`
	Content    string                 `json:"content,omitempty"`
	Timestamp  string                 `json:"timestamp"`
	ToolName   string                 `json:"tool_name,omitempty"`
	ToolInput  string                 `json:"tool_input,omitempty"`
	ToolOutput string                 `json:"tool_output,omitempty"`
	Status     string                 `json:"status,omitempty"`
	Role       string                 `json:"role,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// CPExecution represents an execution from the control plane
type CPExecution struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	OrganizationID string `json:"organization_id"`
}

// CPExecuteRequest represents the request to execute an agent
type CPExecuteRequest struct {
	Prompt        string  `json:"prompt"`
	WorkerQueueID *string `json:"worker_queue_id,omitempty"`
	Stream        *bool   `json:"stream,omitempty"`
}

// executeAgent submits an execution request to the control plane
func (c *cpClient) executeAgent(agentID string, req *CPExecuteRequest) (*CPExecution, error) {
	url := fmt.Sprintf("%s/api/v1/agents/%s/execute", c.baseURL, agentID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var execution CPExecution
	if err := json.NewDecoder(resp.Body).Decode(&execution); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &execution, nil
}

// streamExecution streams events from the control plane for an execution
func (c *cpClient) streamExecution(ctx context.Context, executionID string) (<-chan StreamEvent, <-chan error) {
	eventChan := make(chan StreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		url := fmt.Sprintf("%s/api/v1/executions/%s/stream", c.baseURL, executionID)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			errChan <- fmt.Errorf("failed to create stream request: %w", err)
			return
		}

		// Set headers for SSE
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		// Use context for cancellation
		req = req.WithContext(ctx)

		// Use a client with no timeout for SSE streaming
		streamClient := &http.Client{Timeout: 0}
		resp, err := streamClient.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to connect to stream: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("stream returned status %d: %s", resp.StatusCode, string(body))
			return
		}

		// Read SSE stream
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
					event := parseSSEEventData(currentEventType, currentEventID, data)

					select {
					case eventChan <- event:
					case <-ctx.Done():
						return
					}

					// Stop on terminal events
					if event.Type == "done" || event.Type == "error" || event.Type == "complete" {
						return
					}

					// Reset for next event
					currentEventType = ""
					currentEventID = ""
				} else if line == "" {
					currentEventType = ""
					currentEventID = ""
				}
			}
		}
	}()

	return eventChan, errChan
}

// parseSSEEventData parses SSE data into a normalized StreamEvent
func parseSSEEventData(eventType, eventID, data string) StreamEvent {
	now := time.Now().Format(time.RFC3339)
	event := StreamEvent{
		Type:      eventType,
		Timestamp: now,
	}

	// Parse JSON data
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &rawData); err != nil {
		event.Content = data
		return event
	}

	// Extract timestamp if present
	if ts, ok := rawData["timestamp"].(string); ok {
		event.Timestamp = ts
	}

	// Handle different event types
	switch eventType {
	case "connected":
		event.Type = "status"
		event.Status = "connected"
		if execID, ok := rawData["execution_id"].(string); ok {
			event.Content = fmt.Sprintf("Connected to execution: %s", execID)
		}

	case "message":
		event.Type = "output"
		if role, ok := rawData["role"].(string); ok {
			event.Role = role
		}
		if content, ok := rawData["content"].(string); ok {
			event.Content = content
		}

	case "message_chunk":
		event.Type = "output"
		// Try nested "data" structure
		if nestedData, ok := rawData["data"].(map[string]interface{}); ok {
			if content, ok := nestedData["content"].(string); ok {
				event.Content = content
			}
		}
		// Try "message" structure
		if msg, ok := rawData["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].(string); ok {
				event.Content = content
			}
			if role, ok := msg["role"].(string); ok {
				event.Role = role
			}
		}
		// Fallback to top-level content
		if event.Content == "" {
			if content, ok := rawData["content"].(string); ok {
				event.Content = content
			}
		}

	case "tool_started":
		event.Type = "tool_call"
		if nestedData, ok := rawData["data"].(map[string]interface{}); ok {
			if name, ok := nestedData["tool_name"].(string); ok {
				event.ToolName = name
			}
			if inputs, ok := nestedData["tool_input"].(map[string]interface{}); ok {
				inputBytes, _ := json.Marshal(inputs)
				event.ToolInput = string(inputBytes)
			}
			if inputs, ok := nestedData["tool_arguments"].(map[string]interface{}); ok {
				inputBytes, _ := json.Marshal(inputs)
				event.ToolInput = string(inputBytes)
			}
		}
		if event.ToolName == "" {
			event.ToolName = "tool"
		}
		event.Content = fmt.Sprintf("Calling %s", event.ToolName)

	case "tool_completed":
		event.Type = "tool_result"
		if nestedData, ok := rawData["data"].(map[string]interface{}); ok {
			if name, ok := nestedData["tool_name"].(string); ok {
				event.ToolName = name
			}
			if outputs, ok := nestedData["tool_output"].(map[string]interface{}); ok {
				outputBytes, _ := json.Marshal(outputs)
				event.ToolOutput = string(outputBytes)
			}
			// Check for string output
			if output, ok := nestedData["tool_output"].(string); ok {
				event.ToolOutput = output
			}
			// Check status
			if status, ok := nestedData["tool_status"].(string); ok {
				event.Status = status
			}
		}

	case "status":
		event.Type = "status"
		if status, ok := rawData["status"].(string); ok {
			event.Status = status
			event.Content = fmt.Sprintf("Status: %s", status)
		}
		// Handle nested status object
		if statusObj, ok := rawData["status"].(map[string]interface{}); ok {
			if state, ok := statusObj["state"].(string); ok {
				event.Status = state
				event.Content = fmt.Sprintf("Status: %s", state)
			}
		}

	case "error":
		event.Type = "error"
		if errMsg, ok := rawData["error"].(string); ok {
			event.Content = errMsg
		}
		if content, ok := rawData["content"].(string); ok {
			event.Content = content
		}

	case "done", "complete":
		event.Type = "done"
		if content, ok := rawData["content"].(string); ok {
			event.Content = content
		}

	case "history_complete":
		event.Type = "status"
		event.Status = "history_loaded"

	case "thinking", "reasoning":
		event.Type = "reasoning"
		if content, ok := rawData["content"].(string); ok {
			event.Content = content
		}

	default:
		// Unknown type - try to extract content
		if content, ok := rawData["content"].(string); ok {
			event.Content = content
		}
		if msg, ok := rawData["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].(string); ok {
				event.Content = content
			}
		}
	}

	// Store event ID in metadata
	if eventID != "" {
		event.Metadata = map[string]interface{}{"event_id": eventID}
	}

	return event
}

// DirectExecutionSession represents a direct execution via control plane
type DirectExecutionSession struct {
	ID          string
	AgentID     string
	Cancel      context.CancelFunc
	Events      chan StreamEvent
	Done        chan struct{}
	StartTime   time.Time
	Config      ExecutionConfig
	mu          sync.Mutex
	subscribers []chan StreamEvent
}

// directExecManager manages direct executions
var directExecManager = struct {
	sessions map[string]*DirectExecutionSession
	mu       sync.RWMutex
}{
	sessions: make(map[string]*DirectExecutionSession),
}

// handleDirectExecStart handles POST /api/exec/direct/start - start direct execution via control plane
func (s *Server) handleDirectExecStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.cpClient == nil {
		writeError(w, http.StatusServiceUnavailable, "control plane client not configured")
		return
	}

	var config ExecutionConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if config.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt required")
		return
	}

	if config.EntityID == "" {
		writeError(w, http.StatusBadRequest, "agent ID (entityId) required for direct execution")
		return
	}

	// Submit execution to control plane
	streamFlag := true
	queueID := s.config.QueueID
	execReq := &CPExecuteRequest{
		Prompt:        config.Prompt,
		Stream:        &streamFlag,
		WorkerQueueID: &queueID,
	}

	execution, err := s.cpClient.executeAgent(config.EntityID, execReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start execution: %v", err))
		return
	}

	// Create session
	ctx, cancel := context.WithCancel(context.Background())
	session := &DirectExecutionSession{
		ID:        execution.ID,
		AgentID:   config.EntityID,
		Cancel:    cancel,
		Events:    make(chan StreamEvent, 100),
		Done:      make(chan struct{}),
		StartTime: time.Now(),
		Config:    config,
	}

	directExecManager.mu.Lock()
	directExecManager.sessions[execution.ID] = session
	directExecManager.mu.Unlock()

	// Start streaming in background
	go s.streamDirectExecution(ctx, session)

	writeJSON(w, map[string]interface{}{
		"success":      true,
		"execution_id": execution.ID,
		"message":      "Execution started",
	})
}

// streamDirectExecution streams events from control plane to the session
func (s *Server) streamDirectExecution(ctx context.Context, session *DirectExecutionSession) {
	defer func() {
		close(session.Done)
		// Clean up after delay
		go func() {
			time.Sleep(5 * time.Minute)
			directExecManager.mu.Lock()
			delete(directExecManager.sessions, session.ID)
			directExecManager.mu.Unlock()
		}()
	}()

	// Send initial connected event
	session.Events <- StreamEvent{
		Type:      "status",
		Content:   fmt.Sprintf("Connected to execution: %s", session.ID),
		Status:    "connected",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Stream from control plane
	eventChan, errChan := s.cpClient.streamExecution(ctx, session.ID)

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed - send done event
				session.Events <- StreamEvent{
					Type:      "done",
					Content:   fmt.Sprintf("Completed in %v", time.Since(session.StartTime).Round(time.Millisecond)),
					Timestamp: time.Now().Format(time.RFC3339),
				}
				return
			}
			session.Events <- event

		case err, ok := <-errChan:
			if ok && err != nil {
				session.Events <- StreamEvent{
					Type:      "error",
					Content:   err.Error(),
					Timestamp: time.Now().Format(time.RFC3339),
				}
			}
			return

		case <-ctx.Done():
			return
		}
	}
}

// handleDirectExecStream handles GET /api/exec/direct/stream/{id} - SSE stream for direct execution
func (s *Server) handleDirectExecStream(w http.ResponseWriter, r *http.Request) {
	// Extract execution ID from path
	path := r.URL.Path
	execID := strings.TrimPrefix(path, "/api/exec/direct/stream/")

	if execID == "" {
		writeError(w, http.StatusBadRequest, "execution_id required")
		return
	}

	directExecManager.mu.RLock()
	session, exists := directExecManager.sessions[execID]
	directExecManager.mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Stream events
	for {
		select {
		case event, ok := <-session.Events:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if event.Type == "done" || event.Type == "error" {
				return
			}

		case <-session.Done:
			return

		case <-r.Context().Done():
			return
		}
	}
}

// handleDirectExecStop handles POST /api/exec/direct/stop/{id} - stop a direct execution
func (s *Server) handleDirectExecStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := r.URL.Path
	execID := strings.TrimPrefix(path, "/api/exec/direct/stop/")

	if execID == "" {
		writeError(w, http.StatusBadRequest, "execution_id required")
		return
	}

	directExecManager.mu.RLock()
	session, exists := directExecManager.sessions[execID]
	directExecManager.mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}

	session.Cancel()

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Execution stopped",
	})
}
