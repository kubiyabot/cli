package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (s *SourceBrowser) executeTool() tea.Cmd {
	return func() tea.Msg {
		// Validate before execution
		if err := s.validateExecution(); err != nil {
			return toolExecutedMsg{err: err}
		}

		// Ensure port forward is ready
		if !s.portForward.ready {
			if err := s.setupPortForward()(); err != nil {
				return toolExecutedMsg{err: fmt.Errorf("failed to setup port forward: received %v", err)}
			}
		}

		// Prepare execution request
		execReq := struct {
			ToolName  string            `json:"tool_name"`
			SourceURL string            `json:"source_url"`
			ArgMap    map[string]string `json:"arg_map"`
			EnvVars   map[string]string `json:"env_vars"`
			Async     bool              `json:"async"`
		}{
			ToolName:  s.currentTool.Name,
			SourceURL: s.currentSource.URL,
			ArgMap:    s.execution.args,
			EnvVars:   s.getEnvVarsMap(),
			Async:     false,
		}

		// Execute the tool
		jsonData, err := json.Marshal(execReq)
		if err != nil {
			return toolExecutedMsg{err: fmt.Errorf("failed to prepare request: %w", err)}
		}

		// Make HTTP request with proper error handling
		httpClient := &http.Client{Timeout: 30 * time.Second}
		resp, err := httpClient.Post("http://localhost:5001/tool/execute",
			"application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return toolExecutedMsg{err: fmt.Errorf("failed to execute tool: %w", err)}
		}
		defer resp.Body.Close()

		// Handle response
		var execResp struct {
			Status      string `json:"status"`
			ExecutionID string `json:"execution_id,omitempty"`
			Output      string `json:"output,omitempty"`
			Error       string `json:"error,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
			return toolExecutedMsg{err: fmt.Errorf("failed to decode response: %w", err)}
		}

		if execResp.Error != "" {
			return toolExecutedMsg{err: fmt.Errorf(execResp.Error)}
		}

		// Handle execution status
		switch execResp.Status {
		case "completed":
			s.execution.executing = false
			return toolExecutedMsg{output: execResp.Output}

		case "async":
			s.execution.executing = true
			go s.followAsyncExecution(execResp.ExecutionID)
			return toolExecutedMsg{output: "Execution started..."}

		default:
			return toolExecutedMsg{err: fmt.Errorf("unknown execution status: %s", execResp.Status)}
		}
	}
}

// Add method to follow async execution
func (s *SourceBrowser) followAsyncExecution(execID string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	client := &http.Client{Timeout: 5 * time.Second}

	for range ticker.C {
		resp, err := client.Get(fmt.Sprintf("http://localhost:5001/tool/status/%s", execID))
		if err != nil {
			s.execution.error = err
			s.execution.executing = false
			return
		}

		var status struct {
			Status string `json:"status"`
			Output string `json:"output"`
			Error  string `json:"error"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			s.execution.error = err
			s.execution.executing = false
			return
		}
		resp.Body.Close()

		s.execution.output = status.Output

		if status.Status == "completed" || status.Status == "failed" {
			s.execution.executing = false
			if status.Error != "" {
				s.execution.error = fmt.Errorf(status.Error)
			}
			return
		}
	}
}
