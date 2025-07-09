package kubiya

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WorkflowExecutionState represents the local state of a workflow execution
type WorkflowExecutionState struct {
	ExecutionID     string                    `json:"execution_id"`
	WorkflowName    string                    `json:"workflow_name"`
	Status          string                    `json:"status"` // "running", "completed", "failed", "unknown"
	StartTime       time.Time                 `json:"start_time"`
	EndTime         *time.Time                `json:"end_time,omitempty"`
	LastUpdate      time.Time                 `json:"last_update"`
	TotalSteps      int                       `json:"total_steps"`
	CompletedSteps  int                       `json:"completed_steps"`
	CurrentStep     string                    `json:"current_step,omitempty"`
	StepHistory     []WorkflowStepState       `json:"step_history"`
	LastKnownEvent  *WorkflowSSEEvent         `json:"last_known_event,omitempty"`
	ConnectionLost  bool                      `json:"connection_lost"`
	RetryCount      int                       `json:"retry_count"`
	Runner          string                    `json:"runner"`
	FilePath        string                    `json:"-"` // Not serialized, set when loading
	Request         *WorkflowExecutionRequest `json:"request,omitempty"`
}

// WorkflowStepState represents the state of a single workflow step
type WorkflowStepState struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"` // "pending", "running", "completed", "failed"
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
	Description string     `json:"description,omitempty"`
}

// WorkflowStateManager manages local workflow execution state
type WorkflowStateManager struct {
	stateDir string
	debug    bool
}

// NewWorkflowStateManager creates a new workflow state manager
func NewWorkflowStateManager(debug bool) (*WorkflowStateManager, error) {
	// Create state directory in user's home/.kubiya/workflows
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	stateDir := filepath.Join(homeDir, ".kubiya", "workflows")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &WorkflowStateManager{
		stateDir: stateDir,
		debug:    debug,
	}, nil
}

// CreateExecution creates a new workflow execution state
func (wsm *WorkflowStateManager) CreateExecution(req WorkflowExecutionRequest, runner string) (*WorkflowExecutionState, error) {
	executionID := generateExecutionID()
	now := time.Now()

	state := &WorkflowExecutionState{
		ExecutionID:    executionID,
		WorkflowName:   req.Name,
		Status:         "running",
		StartTime:      now,
		LastUpdate:     now,
		TotalSteps:     len(req.Steps),
		CompletedSteps: 0,
		StepHistory:    make([]WorkflowStepState, 0),
		ConnectionLost: false,
		RetryCount:     0,
		Runner:         runner,
		Request:        &req,
	}

	// Initialize step history with pending steps
	for _, step := range req.Steps {
		stepState := WorkflowStepState{
			Status: "pending",
		}
		
		// Extract step information from interface{}
		if stepMap, ok := step.(map[string]interface{}); ok {
			if name, ok := stepMap["name"].(string); ok {
				stepState.Name = name
			}
			if desc, ok := stepMap["description"].(string); ok {
				stepState.Description = desc
			}
		}
		
		state.StepHistory = append(state.StepHistory, stepState)
	}

	if err := wsm.SaveExecution(state); err != nil {
		return nil, fmt.Errorf("failed to save initial execution state: %w", err)
	}

	if wsm.debug {
		fmt.Printf("[DEBUG] Created execution state: %s\n", executionID)
	}

	return state, nil
}

// SaveExecution saves the workflow execution state to disk
func (wsm *WorkflowStateManager) SaveExecution(state *WorkflowExecutionState) error {
	state.LastUpdate = time.Now()

	filename := fmt.Sprintf("%s.json", state.ExecutionID)
	filePath := filepath.Join(wsm.stateDir, filename)
	state.FilePath = filePath

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal execution state: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write execution state file: %w", err)
	}

	if wsm.debug {
		fmt.Printf("[DEBUG] Saved execution state to: %s\n", filePath)
	}

	return nil
}

// LoadExecution loads a workflow execution state from disk
func (wsm *WorkflowStateManager) LoadExecution(executionID string) (*WorkflowExecutionState, error) {
	filename := fmt.Sprintf("%s.json", executionID)
	filePath := filepath.Join(wsm.stateDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read execution state file: %w", err)
	}

	var state WorkflowExecutionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal execution state: %w", err)
	}

	state.FilePath = filePath

	if wsm.debug {
		fmt.Printf("[DEBUG] Loaded execution state from: %s\n", filePath)
	}

	return &state, nil
}

// UpdateStepStatus updates the status of a specific step
func (wsm *WorkflowStateManager) UpdateStepStatus(state *WorkflowExecutionState, stepName, status string, output, errorMsg string) error {
	now := time.Now()

	// Find and update the step
	for i := range state.StepHistory {
		if state.StepHistory[i].Name == stepName {
			step := &state.StepHistory[i]
			
			// Set start time if step is starting
			if status == "running" && step.StartTime == nil {
				step.StartTime = &now
				state.CurrentStep = stepName
			}
			
			// Set end time if step is finishing
			if (status == "completed" || status == "failed") && step.EndTime == nil {
				step.EndTime = &now
				if status == "completed" {
					state.CompletedSteps++
					if wsm.debug {
						fmt.Printf("[DEBUG] Step '%s' completed. Progress: %d/%d\n", stepName, state.CompletedSteps, state.TotalSteps)
					}
				}
				if stepName == state.CurrentStep {
					state.CurrentStep = ""
				}
			}

			step.Status = status
			if output != "" {
				step.Output = output
			}
			if errorMsg != "" {
				step.Error = errorMsg
			}
			break
		}
	}

	return wsm.SaveExecution(state)
}

// MarkConnectionLost marks the execution as having lost connection
func (wsm *WorkflowStateManager) MarkConnectionLost(state *WorkflowExecutionState) error {
	state.ConnectionLost = true
	state.RetryCount++
	return wsm.SaveExecution(state)
}

// MarkConnectionRestored marks the execution as having restored connection
func (wsm *WorkflowStateManager) MarkConnectionRestored(state *WorkflowExecutionState) error {
	state.ConnectionLost = false
	return wsm.SaveExecution(state)
}

// CompleteExecution marks the execution as completed
func (wsm *WorkflowStateManager) CompleteExecution(state *WorkflowExecutionState, status string) error {
	now := time.Now()
	state.Status = status
	state.EndTime = &now
	state.ConnectionLost = false
	return wsm.SaveExecution(state)
}

// ListActiveExecutions returns all active (running) executions
func (wsm *WorkflowStateManager) ListActiveExecutions() ([]*WorkflowExecutionState, error) {
	files, err := os.ReadDir(wsm.stateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	var activeExecutions []*WorkflowExecutionState
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			executionID := file.Name()[:len(file.Name())-5] // Remove .json extension
			state, err := wsm.LoadExecution(executionID)
			if err != nil {
				if wsm.debug {
					fmt.Printf("[DEBUG] Failed to load execution %s: %v\n", executionID, err)
				}
				continue
			}

			if state.Status == "running" || state.ConnectionLost {
				activeExecutions = append(activeExecutions, state)
			}
		}
	}

	return activeExecutions, nil
}

// CleanupOldExecutions removes execution state files older than the specified duration
func (wsm *WorkflowStateManager) CleanupOldExecutions(maxAge time.Duration) error {
	files, err := os.ReadDir(wsm.stateDir)
	if err != nil {
		return fmt.Errorf("failed to read state directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			filePath := filepath.Join(wsm.stateDir, file.Name())
			info, err := file.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				// Load state to check if it's completed
				executionID := file.Name()[:len(file.Name())-5]
				state, err := wsm.LoadExecution(executionID)
				if err != nil {
					continue
				}

				// Only cleanup completed or failed executions
				if state.Status == "completed" || state.Status == "failed" {
					if err := os.Remove(filePath); err != nil {
						if wsm.debug {
							fmt.Printf("[DEBUG] Failed to remove old execution file %s: %v\n", filePath, err)
						}
					} else if wsm.debug {
						fmt.Printf("[DEBUG] Cleaned up old execution: %s\n", executionID)
					}
				}
			}
		}
	}

	return nil
}

// DeleteExecution removes an execution state file
func (wsm *WorkflowStateManager) DeleteExecution(executionID string) error {
	filename := fmt.Sprintf("%s.json", executionID)
	filePath := filepath.Join(wsm.stateDir, filename)
	
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete execution state: %w", err)
	}

	if wsm.debug {
		fmt.Printf("[DEBUG] Deleted execution state: %s\n", executionID)
	}

	return nil
}

// generateExecutionID generates a unique execution ID
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d_%d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
}

// GetExecutionSummary returns a human-readable summary of the execution
func (state *WorkflowExecutionState) GetExecutionSummary() string {
	duration := time.Since(state.StartTime)
	if state.EndTime != nil {
		duration = state.EndTime.Sub(state.StartTime)
	}

	status := state.Status
	if state.ConnectionLost {
		status += " (connection lost)"
	}

	return fmt.Sprintf("Execution %s: %s [%d/%d steps] - %v",
		state.ExecutionID,
		status,
		state.CompletedSteps,
		state.TotalSteps,
		duration.Round(time.Second))
}

// IsStale checks if the execution state is stale (no updates for a long time)
func (state *WorkflowExecutionState) IsStale(threshold time.Duration) bool {
	return time.Since(state.LastUpdate) > threshold
}