package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (s *SourceBrowser) executeTool() tea.Cmd {
	return func() tea.Msg {
		// If not prepared, run validations first
		if !s.execution.prepared {
			validation := s.validateToolExecution()
			if len(validation) > 0 {
				var errMsg strings.Builder
				errMsg.WriteString("❌ Validation errors:\n")
				for _, err := range validation {
					errMsg.WriteString(fmt.Sprintf("• %s\n", err))
				}
				return toolExecutedMsg{err: fmt.Errorf(errMsg.String())}
			}

			s.execution.prepared = true
		}

		// If prepared but not confirmed, show confirmation
		if !s.execution.confirmed {
			s.state = stateExecuteConfirm
			s.execution.output = s.renderExecutionSummary()
			return nil
		}

		// Start actual execution
		s.state = stateExecuting
		s.execution.executing = true
		return s.startExecution()
	}
}

func (s *SourceBrowser) startExecution() tea.Cmd {
	return func() tea.Msg {
		// Check if port forward is ready
		if !s.portForward.ready {
			s.execution.output = "⚡ Setting up connection to tool manager...\n"
			msg := s.setupPortForward()()
			if pfMsg, ok := msg.(portForwardMsg); ok {
				if pfMsg.err != nil {
					return toolExecutedMsg{err: pfMsg.err}
				}
				if !pfMsg.ready {
					return toolExecutedMsg{err: fmt.Errorf("Failed to establish connection to tool manager")}
				}
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
			return toolExecutedMsg{err: fmt.Errorf("Failed to prepare request:\n%w", err)}
		}

		// Make the HTTP request
		httpClient := &http.Client{Timeout: 30 * time.Second}
		resp, err := httpClient.Post("http://localhost:5001/tool/execute", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return toolExecutedMsg{err: fmt.Errorf("Failed to execute tool:\n%w\n\nPlease check:\n"+
				"1. The tool manager is running\n"+
				"2. The port forward is working\n"+
				"3. Your network connection", err)}
		}
		defer resp.Body.Close()

		// Parse the response
		var execResp struct {
			Status      string `json:"status"`
			ExecutionID string `json:"execution_id,omitempty"`
			Output      string `json:"output,omitempty"`
			Error       string `json:"error,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
			return toolExecutedMsg{err: fmt.Errorf("Failed to decode response:\n%w", err)}
		}

		// Handle execution response
		if execResp.Error != "" {
			return toolExecutedMsg{err: fmt.Errorf("Tool execution failed:\n%s", execResp.Error)}
		}

		s.execution.output = execResp.Output
		if execResp.Status == "completed" {
			s.execution.executing = false
			return toolExecutedMsg{output: execResp.Output}
		}

		// Handle async execution
		if execResp.Status == "async" {
			go func() {
				err := followAsyncExecution(httpClient, execResp.ExecutionID, &s.execution)
				if err != nil {
					s.execution.error = err
				}
				s.execution.executing = false
			}()
		}

		return toolExecutedMsg{output: s.execution.output}
	}
}

func followAsyncExecution(client *http.Client, execID string, state *executionState) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		resp, err := client.Get(fmt.Sprintf("http://localhost:5001/tool/status/%s", execID))
		if err != nil {
			return err
		}

		var status struct {
			Status string `json:"status"`
			Output string `json:"output"`
			Error  string `json:"error"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			return err
		}
		resp.Body.Close()

		state.output = status.Output

		if status.Status == "completed" || status.Status == "failed" {
			if status.Error != "" {
				return fmt.Errorf(status.Error)
			}
			return nil
		}
	}

	return nil
}
