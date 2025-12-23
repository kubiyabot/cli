package webui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ExecutionSession represents an active execution
type ExecutionSession struct {
	ID         string
	Cmd        *exec.Cmd
	Cancel     context.CancelFunc
	Events     chan ExecutionEvent
	Done       chan struct{}
	StartTime  time.Time
	Config     ExecutionConfig
	mu         sync.Mutex
	subscribed bool
}

// ExecutionConfig is the config for starting an execution
type ExecutionConfig struct {
	Prompt       string `json:"prompt"`
	Mode         string `json:"mode"`         // auto, agent, team
	EntityID     string `json:"entityId"`     // agent or team ID
	Local        bool   `json:"local"`        // run locally
	Environment  string `json:"environment"`  // environment ID
	WorkingDir   string `json:"workingDir"`   // working directory
	StreamFormat string `json:"streamFormat"` // text, json
	Verbose      bool   `json:"verbose"`
}

// ExecutionEvent represents a streaming event
type ExecutionEvent struct {
	Type       string `json:"type"` // text, tool_call, tool_result, reasoning, error, status, plan, done
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolInput  string `json:"tool_input,omitempty"`
	ToolOutput string `json:"tool_output,omitempty"`
	Status     string `json:"status,omitempty"`
}

// ExecutionManager manages active executions
type ExecutionManager struct {
	sessions map[string]*ExecutionSession
	mu       sync.RWMutex
}

var execManager = &ExecutionManager{
	sessions: make(map[string]*ExecutionSession),
}

// handleExecAgents handles GET /api/exec/agents - list available agents
func (s *Server) handleExecAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	agents, err := s.fetchAgentsFromControlPlane()
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"agents": []interface{}{},
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

// handleExecTeams handles GET /api/exec/teams - list available teams
func (s *Server) handleExecTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	teams, err := s.fetchTeamsFromControlPlane()
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"teams": []interface{}{},
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"teams": teams,
		"count": len(teams),
	})
}

// handleExecEnvironments handles GET /api/exec/environments - list available environments
func (s *Server) handleExecEnvironments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	envs, err := s.fetchEnvironmentsFromControlPlane()
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"environments": []interface{}{},
			"error":        err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"environments": envs,
		"count":        len(envs),
	})
}

// handleExecStart handles POST /api/exec/start - start an execution
func (s *Server) handleExecStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

	// Generate execution ID
	execID := fmt.Sprintf("exec-%d", time.Now().UnixNano())

	// Create execution session
	ctx, cancel := context.WithCancel(context.Background())
	session := &ExecutionSession{
		ID:        execID,
		Cancel:    cancel,
		Events:    make(chan ExecutionEvent, 100),
		Done:      make(chan struct{}),
		StartTime: time.Now(),
		Config:    config,
	}

	execManager.mu.Lock()
	execManager.sessions[execID] = session
	execManager.mu.Unlock()

	// Start execution in background
	go s.runExecution(ctx, session)

	writeJSON(w, map[string]interface{}{
		"success":      true,
		"execution_id": execID,
		"message":      "Execution started",
	})
}

// handleExecStream handles GET /api/exec/stream/{id} - SSE stream for execution events
func (s *Server) handleExecStream(w http.ResponseWriter, r *http.Request) {
	// Extract execution ID from path
	path := r.URL.Path
	execID := strings.TrimPrefix(path, "/api/exec/stream/")

	if execID == "" {
		writeError(w, http.StatusBadRequest, "execution_id required")
		return
	}

	execManager.mu.RLock()
	session, exists := execManager.sessions[execID]
	execManager.mu.RUnlock()

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

	// Mark as subscribed
	session.mu.Lock()
	session.subscribed = true
	session.mu.Unlock()

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

// handleExecStop handles POST /api/exec/stop/{id} - stop an execution
func (s *Server) handleExecStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract execution ID from path
	path := r.URL.Path
	execID := strings.TrimPrefix(path, "/api/exec/stop/")

	if execID == "" {
		writeError(w, http.StatusBadRequest, "execution_id required")
		return
	}

	execManager.mu.RLock()
	session, exists := execManager.sessions[execID]
	execManager.mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}

	// Cancel the execution
	session.Cancel()

	// Kill the process if running
	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Execution stopped",
	})
}

// runExecution runs the kubiya exec command
func (s *Server) runExecution(ctx context.Context, session *ExecutionSession) {
	defer func() {
		close(session.Done)
		// Clean up after a delay
		go func() {
			time.Sleep(5 * time.Minute)
			execManager.mu.Lock()
			delete(execManager.sessions, session.ID)
			execManager.mu.Unlock()
		}()
	}()

	config := session.Config

	// Build command args
	args := []string{"exec"}

	// Add mode-specific args
	if config.Mode == "agent" && config.EntityID != "" {
		args = append(args, "agent", config.EntityID)
	} else if config.Mode == "team" && config.EntityID != "" {
		args = append(args, "team", config.EntityID)
	}

	// Add prompt
	args = append(args, config.Prompt)

	// Add flags
	args = append(args, "--yes") // Auto-confirm

	// Use stream format from config (default to text for human-readable output)
	streamFormat := config.StreamFormat
	if streamFormat == "" {
		streamFormat = "text"
	}
	args = append(args, fmt.Sprintf("--stream-format=%s", streamFormat))

	if config.Local {
		args = append(args, "--local")
	}

	if config.Environment != "" {
		args = append(args, fmt.Sprintf("--environment=%s", config.Environment))
	}

	if config.Verbose {
		args = append(args, "-v")
	}

	// Find kubiya CLI binary
	kubiyaBin := findKubiyaBinary()

	session.Events <- ExecutionEvent{
		Type:      "status",
		Content:   fmt.Sprintf("Starting: %s %s", kubiyaBin, strings.Join(args, " ")),
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    "starting",
	}

	// Create command
	cmd := exec.CommandContext(ctx, kubiyaBin, args...)
	session.Cmd = cmd

	// Set working directory
	if config.WorkingDir != "" {
		cmd.Dir = config.WorkingDir
	} else if s.config.WorkerDir != "" {
		cmd.Dir = s.config.WorkerDir
	}

	// Inherit environment
	cmd.Env = os.Environ()

	// Create pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		session.Events <- ExecutionEvent{
			Type:      "error",
			Content:   fmt.Sprintf("Failed to create stdout pipe: %v", err),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		session.Events <- ExecutionEvent{
			Type:      "error",
			Content:   fmt.Sprintf("Failed to create stderr pipe: %v", err),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		session.Events <- ExecutionEvent{
			Type:      "error",
			Content:   fmt.Sprintf("Failed to start execution: %v", err),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		return
	}

	// Read stdout - planning/TUI output goes here
	// Note: Actual streaming events go to stderr, this is mostly CLI decoration
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// Strip ANSI codes
			line = stripANSIExec(line)
			if line == "" {
				continue
			}

			// Check for JSON (might get some streaming events on stdout too)
			if strings.HasPrefix(strings.TrimSpace(line), "{") {
				event := parseControlPlaneEvent(line)
				if event.Type != "" && (event.Content != "" || event.ToolName != "") {
					session.Events <- event
					continue
				}
			}

			// For text format, show planning output too (user wants to see the plan)
			if streamFormat == "text" {
				// Skip excessive noise but allow planning output
				if shouldSkipLineExec(line) {
					continue
				}
				session.Events <- ExecutionEvent{
					Type:      "text",
					Content:   line,
					Timestamp: time.Now().Format(time.RFC3339),
				}
				continue
			}

			// For JSON format, skip non-JSON noise
			if shouldSkipLineExec(line) {
				continue
			}

			// Try to parse as JSON
			event := parseControlPlaneEvent(line)
			if event.Type == "status" && event.Content == "" {
				continue
			}
			session.Events <- event
		}
	}()

	// Read stderr - THIS IS WHERE STREAMING EVENTS GO!
	// The CLI writes streaming output to stderr, not stdout
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large outputs

		for scanner.Scan() {
			line := scanner.Text()
			line = stripANSIExec(line)

			if line == "" {
				continue
			}

			// Try to parse as JSON streaming event first
			if strings.HasPrefix(strings.TrimSpace(line), "{") {
				event := parseControlPlaneEvent(line)
				if event.Type != "" && (event.Content != "" || event.ToolName != "") {
					session.Events <- event
					continue
				}
			}

			// For text format streaming, send non-noise lines as text events
			if streamFormat == "text" {
				// Skip planning/worker noise
				if shouldSkipLineExec(line) {
					continue
				}
				session.Events <- ExecutionEvent{
					Type:      "text",
					Content:   line,
					Timestamp: time.Now().Format(time.RFC3339),
				}
				continue
			}

			// For JSON format, only process JSON lines (already handled above)
			// Skip non-JSON noise
			if shouldSkipLineExec(line) {
				continue
			}

			// Show actual errors
			if strings.Contains(strings.ToLower(line), "error") {
				session.Events <- ExecutionEvent{
					Type:      "error",
					Content:   line,
					Timestamp: time.Now().Format(time.RFC3339),
				}
			}
		}
	}()

	// Wait for completion
	err = cmd.Wait()

	if ctx.Err() == context.Canceled {
		session.Events <- ExecutionEvent{
			Type:      "status",
			Content:   "Execution cancelled",
			Timestamp: time.Now().Format(time.RFC3339),
			Status:    "cancelled",
		}
		return
	}

	if err != nil {
		session.Events <- ExecutionEvent{
			Type:      "error",
			Content:   fmt.Sprintf("Execution failed: %v", err),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		return
	}

	session.Events <- ExecutionEvent{
		Type:      "done",
		Content:   fmt.Sprintf("Completed in %v", time.Since(session.StartTime).Round(time.Millisecond)),
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// fetchAgentsFromControlPlane fetches agents from control plane
func (s *Server) fetchAgentsFromControlPlane() ([]map[string]interface{}, error) {
	if s.config.ControlPlaneURL == "" || s.config.APIKey == "" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", s.config.ControlPlaneURL+"/api/v1/agents", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "UserKey "+s.config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control plane returned %d", resp.StatusCode)
	}

	var agents []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}

	// Simplify the response
	result := make([]map[string]interface{}, 0, len(agents))
	for _, agent := range agents {
		result = append(result, map[string]interface{}{
			"id":          agent["id"],
			"name":        agent["name"],
			"description": agent["description"],
			"model":       agent["model_id"],
		})
	}

	return result, nil
}

// fetchTeamsFromControlPlane fetches teams from control plane
func (s *Server) fetchTeamsFromControlPlane() ([]map[string]interface{}, error) {
	if s.config.ControlPlaneURL == "" || s.config.APIKey == "" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", s.config.ControlPlaneURL+"/api/v1/teams", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "UserKey "+s.config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control plane returned %d", resp.StatusCode)
	}

	var teams []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		return nil, err
	}

	// Simplify the response
	result := make([]map[string]interface{}, 0, len(teams))
	for _, team := range teams {
		result = append(result, map[string]interface{}{
			"id":          team["id"],
			"name":        team["name"],
			"description": team["description"],
		})
	}

	return result, nil
}

// fetchEnvironmentsFromControlPlane fetches environments from control plane
func (s *Server) fetchEnvironmentsFromControlPlane() ([]map[string]interface{}, error) {
	if s.config.ControlPlaneURL == "" || s.config.APIKey == "" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", s.config.ControlPlaneURL+"/api/v1/environments", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "UserKey "+s.config.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control plane returned %d", resp.StatusCode)
	}

	var envs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&envs); err != nil {
		return nil, err
	}

	// Simplify the response
	result := make([]map[string]interface{}, 0, len(envs))
	for _, env := range envs {
		result = append(result, map[string]interface{}{
			"id":   env["id"],
			"name": env["name"],
		})
	}

	return result, nil
}

// findKubiyaBinary finds the kubiya CLI binary
func findKubiyaBinary() string {
	// Check common locations
	locations := []string{
		"kubiya",
		"./kubiya",
		"/usr/local/bin/kubiya",
		os.Getenv("HOME") + "/.kubiya/bin/kubiya",
	}

	for _, loc := range locations {
		if _, err := exec.LookPath(loc); err == nil {
			return loc
		}
	}

	// Fallback to 'kubiya' and let PATH resolve it
	return "kubiya"
}

// parseControlPlaneEvent parses control plane JSON and transforms to normalized event
func parseControlPlaneEvent(line string) ExecutionEvent {
	now := time.Now().Format(time.RFC3339)

	// Try to parse as JSON
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		// Plain text
		return ExecutionEvent{
			Type:      "text",
			Content:   line,
			Timestamp: now,
		}
	}

	// Get common fields
	eventType, _ := raw["type"].(string)
	timestamp, ok := raw["timestamp"].(string)
	if !ok || timestamp == "" {
		timestamp = now
	}

	// Store raw JSON for display
	rawJSON, _ := json.Marshal(raw)

	switch eventType {
	case "connected":
		execID, _ := raw["execution_id"].(string)
		return ExecutionEvent{
			Type:      "status",
			Content:   fmt.Sprintf("Connected to execution: %s", execID),
			Timestamp: timestamp,
			Status:    "connected",
		}

	case "message":
		// Handle message events - these contain the conversation
		if msg, ok := raw["message"].(map[string]interface{}); ok {
			role, _ := msg["role"].(string)
			content, _ := msg["content"].(string)

			if role == "assistant" {
				return ExecutionEvent{
					Type:      "text",
					Content:   content,
					Timestamp: timestamp,
				}
			} else if role == "user" {
				return ExecutionEvent{
					Type:      "status",
					Content:   fmt.Sprintf("User: %s", truncateString(content, 100)),
					Timestamp: timestamp,
					Status:    "message",
				}
			}
		}
		return ExecutionEvent{
			Type:      "text",
			Content:   string(rawJSON),
			Timestamp: timestamp,
		}

	case "status":
		// Handle status events
		if status, ok := raw["status"].(map[string]interface{}); ok {
			state, _ := status["state"].(string)
			return ExecutionEvent{
				Type:      "status",
				Content:   fmt.Sprintf("Status: %s", state),
				Timestamp: timestamp,
				Status:    state,
			}
		}
		return ExecutionEvent{
			Type:      "status",
			Content:   string(rawJSON),
			Timestamp: timestamp,
		}

	case "tool_call", "tool_use", "tool_started":
		// Handle tool calls / tool started events
		toolName := ""
		toolInput := ""

		// Check "tool" field first (CLI streaming format)
		if tool, ok := raw["tool"].(map[string]interface{}); ok {
			toolName, _ = tool["name"].(string)
			if input, ok := tool["input"].(map[string]interface{}); ok {
				inputBytes, _ := json.Marshal(input)
				toolInput = string(inputBytes)
			} else if inputStr, ok := tool["input"].(string); ok {
				toolInput = inputStr
			}
		}

		// Try "data" field (control plane streaming format)
		if toolName == "" {
			if data, ok := raw["data"].(map[string]interface{}); ok {
				toolName, _ = data["tool_name"].(string)
				if toolName == "" {
					toolName, _ = data["name"].(string)
				}
				if input, ok := data["tool_input"].(map[string]interface{}); ok {
					inputBytes, _ := json.Marshal(input)
					toolInput = string(inputBytes)
				} else if input, ok := data["input"].(map[string]interface{}); ok {
					inputBytes, _ := json.Marshal(input)
					toolInput = string(inputBytes)
				} else if inputStr, ok := data["tool_input"].(string); ok {
					toolInput = inputStr
				}
			}
		}

		// Try tool_call structure
		if toolName == "" {
			if tc, ok := raw["tool_call"].(map[string]interface{}); ok {
				toolName, _ = tc["name"].(string)
				if input, ok := tc["input"].(map[string]interface{}); ok {
					inputBytes, _ := json.Marshal(input)
					toolInput = string(inputBytes)
				} else if inputStr, ok := tc["input"].(string); ok {
					toolInput = inputStr
				}
			} else if tc, ok := raw["content"].(map[string]interface{}); ok {
				toolName, _ = tc["name"].(string)
				if input, ok := tc["input"].(map[string]interface{}); ok {
					inputBytes, _ := json.Marshal(input)
					toolInput = string(inputBytes)
				}
			}
		}

		// Also check top-level fields
		if toolName == "" {
			toolName, _ = raw["tool_name"].(string)
		}
		if toolName == "" {
			toolName, _ = raw["name"].(string)
		}
		if toolInput == "" {
			if input, ok := raw["tool_input"].(map[string]interface{}); ok {
				inputBytes, _ := json.Marshal(input)
				toolInput = string(inputBytes)
			} else if input, ok := raw["input"].(map[string]interface{}); ok {
				inputBytes, _ := json.Marshal(input)
				toolInput = string(inputBytes)
			}
		}

		if toolName == "" {
			toolName = "tool"
		}

		return ExecutionEvent{
			Type:      "tool_call",
			Content:   fmt.Sprintf("Calling %s", toolName),
			Timestamp: timestamp,
			ToolName:  toolName,
			ToolInput: toolInput,
		}

	case "tool_result", "tool_output", "tool_completed":
		// Handle tool results / tool completed events
		output := ""
		toolName := ""
		isError := false

		// Check "tool" field first (CLI streaming format)
		if tool, ok := raw["tool"].(map[string]interface{}); ok {
			toolName, _ = tool["name"].(string)
			if success, ok := tool["success"].(bool); ok && !success {
				isError = true
			}
			if outputStr, ok := tool["output"].(string); ok {
				output = outputStr
			} else if outputObj, ok := tool["output"].(map[string]interface{}); ok {
				outputBytes, _ := json.Marshal(outputObj)
				output = string(outputBytes)
			}
		}

		// Check "data" field (control plane streaming format)
		if toolName == "" {
			if data, ok := raw["data"].(map[string]interface{}); ok {
				toolName, _ = data["tool_name"].(string)
				if toolName == "" {
					toolName, _ = data["name"].(string)
				}
				if outputStr, ok := data["tool_output"].(string); ok {
					output = outputStr
				} else if outputObj, ok := data["tool_output"].(map[string]interface{}); ok {
					outputBytes, _ := json.Marshal(outputObj)
					output = string(outputBytes)
				} else if outputStr, ok := data["output"].(string); ok {
					output = outputStr
				}
				if status, ok := data["tool_status"].(string); ok && status == "error" {
					isError = true
				}
				if errStr, ok := data["error"].(string); ok && errStr != "" {
					output = errStr
					isError = true
				}
			}
		}

		// Try result structure
		if output == "" {
			if result, ok := raw["result"].(map[string]interface{}); ok {
				if outputStr, ok := result["output"].(string); ok {
					output = outputStr
				} else {
					outputBytes, _ := json.Marshal(result)
					output = string(outputBytes)
				}
				if errStr, ok := result["error"].(string); ok && errStr != "" {
					output = errStr
					isError = true
				}
			} else if content, ok := raw["content"].(string); ok {
				output = content
			}
		}

		// Check top-level fields
		if toolName == "" {
			toolName, _ = raw["tool_name"].(string)
		}
		if toolName == "" {
			toolName, _ = raw["name"].(string)
		}
		if output == "" {
			if outputStr, ok := raw["tool_output"].(string); ok {
				output = outputStr
			}
		}

		// Default success message if no output
		if output == "" && !isError {
			output = "Completed successfully"
		}

		eventType := "tool_result"
		if isError {
			output = "Error: " + output
		}

		return ExecutionEvent{
			Type:       eventType,
			Content:    output,
			Timestamp:  timestamp,
			ToolName:   toolName,
			ToolOutput: output,
		}

	case "thinking", "reasoning":
		content := ""
		if c, ok := raw["content"].(string); ok {
			content = c
		} else if thinking, ok := raw["thinking"].(string); ok {
			content = thinking
		}
		return ExecutionEvent{
			Type:      "reasoning",
			Content:   content,
			Timestamp: timestamp,
		}

	case "message_chunk":
		// Handle message_chunk - this is the actual agent response
		content := ""
		if msg, ok := raw["message"].(map[string]interface{}); ok {
			if c, ok := msg["content"].(string); ok {
				content = c
			}
		}
		// Skip empty or placeholder content
		if content == "" || content == "(no content)" {
			return ExecutionEvent{}
		}
		// Return as "output" type to distinguish from CLI noise
		return ExecutionEvent{
			Type:      "output",
			Content:   content,
			Timestamp: timestamp,
		}

	case "text", "output", "content_block_delta":
		// Handle text/content output
		content := ""
		if c, ok := raw["content"].(string); ok {
			content = c
		} else if text, ok := raw["text"].(string); ok {
			content = text
		} else if delta, ok := raw["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				content = text
			}
		}
		return ExecutionEvent{
			Type:      "output",
			Content:   content,
			Timestamp: timestamp,
		}

	case "metadata":
		// Skip metadata events
		return ExecutionEvent{
			Type:      "status",
			Timestamp: timestamp,
		}

	case "run_started":
		// Agent run started
		return ExecutionEvent{
			Type:      "status",
			Content:   "Agent started processing...",
			Timestamp: timestamp,
			Status:    "running",
		}

	case "plan":
		content := ""
		if c, ok := raw["content"].(string); ok {
			content = c
		} else if plan, ok := raw["plan"].(string); ok {
			content = plan
		}
		return ExecutionEvent{
			Type:      "plan",
			Content:   content,
			Timestamp: timestamp,
		}

	case "error":
		content := ""
		if c, ok := raw["content"].(string); ok {
			content = c
		} else if errStr, ok := raw["error"].(string); ok {
			content = errStr
		} else if msg, ok := raw["message"].(string); ok {
			content = msg
		}
		return ExecutionEvent{
			Type:      "error",
			Content:   content,
			Timestamp: timestamp,
		}

	case "done", "complete", "completed":
		content := ""
		if c, ok := raw["content"].(string); ok {
			content = c
		}
		return ExecutionEvent{
			Type:      "done",
			Content:   content,
			Timestamp: timestamp,
		}

	default:
		// Unknown event type - show as raw JSON for debugging
		return ExecutionEvent{
			Type:      "text",
			Content:   string(rawJSON),
			Timestamp: timestamp,
		}
	}
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// stripANSIExec removes ANSI escape codes from a string
func stripANSIExec(s string) string {
	result := make([]byte, 0, len(s))
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result = append(result, s[i])
	}

	return strings.TrimSpace(string(result))
}

// shouldSkipLineExec returns true if the line should be filtered out
func shouldSkipLineExec(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}

	// Skip CLI decorations, banners, and worker startup noise
	// IMPORTANT: Don't filter JSON lines - they start with {
	if strings.HasPrefix(trimmed, "{") {
		return false // Never skip JSON lines
	}

	// Skip lines that start with common CLI decoration characters
	// These are typically planning/TUI output, not agent responses
	firstRune := []rune(trimmed)[0]
	decorationPrefixes := []rune{
		'💻', '🤖', '✓', '⚙', '🔍', '📋', '⏳', '📦', '💰', '💾', '▶', '►', '🚀',
		'⚠', '■', '•', '›', '💕', '⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏',
	}
	for _, prefix := range decorationPrefixes {
		if firstRune == prefix {
			return true
		}
	}

	// Skip lines that are heavily indented (planner TUI output)
	if strings.HasPrefix(line, "   ") || strings.HasPrefix(line, "\t\t") {
		return true
	}

	// Skip Progress: lines
	if strings.HasPrefix(trimmed, "Progress:") {
		return true
	}

	skipPatterns := []string{
		// Box drawing characters
		"═══", "╔", "╗", "║", "╚", "╝", "╠", "╣", "───", "┌", "┐", "└", "┘", "│",
		// Update notices
		"UPDATE AVAILABLE",
		"Current version:",
		"Latest version:",
		"Run 'kubiya update'",
		// Execution headers
		"Executing task:",
		"Direct Execution",
		"Using agent:",
		// Debug/info prefixes
		"[DEBUG]",
		"[INFO]",
		"INFO:",
		"WARNING:",
		// Worker startup noise
		"Python environment",
		"Installing dependencies",
		"Dependencies installed",
		"SUCCESS:",
		"Starting worker process",
		"Worker started",
		"Worker is polling",
		"Worker ready",
		"Still waiting",
		"Press Ctrl+C",
		"Executing Task",
		"✓ Python",
		"► Installing",
		"✓ Dependencies",
		"► Starting worker",
		"✓ Worker",
		"💕 Worker",
		"🚀 Executing",
		"🚀 Starting",
		"⏳ Waiting",
		"⏳ Discovering",
		"✓ Found",
		"✓ Queue",
		"✓ Plan",
		"📦 Creating",
		"📋 Task",
		"💰 Estimated",
		"💾 Plan saved",
		"🤖 Intelligent",
		"🤖 Using agent",
		"💻 Local Execution",
		"Progress:",
		"Phase 1:",
		"Responsibilities:",
		"Estimated Time:",
		"Identified Risks",
		"Prerequisites",
		"Success Criteria",
		"Cost Estimate",
		"Task Breakdown",
		"Recommended Execution",
		"No environment specified",
		"Auto-selected",
		"Creating ephemeral",
		"Starting local worker",
		"Worker Startup",
		"Control Plane:",
		"Queue:",
		"WebUI:",
		"Setting up Python",
		"Package manager detected",
		"Fetching latest version",
		"Latest version found",
		"Worker process started",
		"Worker process exited",
		// Step indicators like [1/3], [2/3], etc.
		"[1/", "[2/", "[3/", "[4/", "[5/",
		// Task planner TUI output (CLI noise before actual execution)
		"• ", "✓ ", "⚠ ", "■ ", "› ", // Bullet points and checkmarks
		"(First run may take",
		"Status: Active",
		"Format and present",
		"Handle any permission",
		"Directory may contain",
		"Large directories",
		"Permission denied errors",
		"Access to the target",
		"Basic shell execution",
		"Directory contents are",
		"All files and subdirectories",
		"No errors or permission",
		"Output is clearly formatted",
		"may produce verbose output",
		"should not be listed",
		"is not accessible",
		// More planner TUI patterns
		"Local Execution Mode",
		"Running with ephemeral",
		"Using fast planning",
		"Discovering available resources",
		"Discovered ",
		"Generating execution plan",
		"[initializing]",
		"[analyzing]",
		"[generating]",
		"Analyzing agents",
		"Task Analyzer",
		"Resource Selector",
		"completed",
		"Creating detailed execution plan",
		"cost estimates",
		"risks, and success",
		"Plan Summary",
		"Execution Plan",
		"Selected Agent",
		"Task Description",
		"Execution Steps",
		"Risk Assessment",
		"Plan approved",
		"ephemeral worker",
		"▶ Starting",
		"Worker process",
		"Virtual environment",
		"uv is available",
		"Virtual env exists",
		"Installing package",
		"Using uv",
		"kubiya-worker-controller",
		"Agent:",
		"Task:",
		"Environment:",
		"Model:",
		"Agent execution started",
		"Execution completed",
		"Session ended",
		"Plan ID:",
		"Agent ID:",
		"Team ID:",
		// Spinner/progress indicators
		"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}

	// Skip lines that are mostly box-drawing characters or pipes
	boxChars := 0
	for _, r := range trimmed {
		if r == '═' || r == '║' || r == '╔' || r == '╗' || r == '╚' || r == '╝' || r == '─' || r == '│' || r == '|' || r == ' ' {
			boxChars++
		}
	}
	if len(trimmed) > 0 && float64(boxChars)/float64(len(trimmed)) > 0.5 {
		return true
	}

	// Skip lines that are just pipes and spaces (table borders)
	pipeOnly := true
	for _, r := range trimmed {
		if r != '|' && r != ' ' && r != '\t' {
			pipeOnly = false
			break
		}
	}
	if pipeOnly && len(trimmed) > 0 {
		return true
	}

	return false
}
