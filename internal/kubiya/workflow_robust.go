package kubiya

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RobustExecutionOptions configures robust workflow execution behavior
type RobustExecutionOptions struct {
	NoRetry     bool          // Disable retry on connection failures
	MaxRetries  int           // Maximum retry attempts
	RetryDelay  time.Duration // Initial retry delay (exponential backoff)
	Verbose     bool          // Enable verbose logging
	ResumeOnly  bool          // Only resume existing execution, don't create new
}

// RobustWorkflowClient provides reliable workflow execution with connection recovery
type RobustWorkflowClient struct {
	client      *WorkflowClient
	stateManager *WorkflowStateManager
	debug       bool
}

// NewRobustWorkflowClient creates a new robust workflow client
func NewRobustWorkflowClient(client *WorkflowClient, debug bool) (*RobustWorkflowClient, error) {
	stateManager, err := NewWorkflowStateManager(debug)
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	return &RobustWorkflowClient{
		client:       client,
		stateManager: stateManager,
		debug:        debug,
	}, nil
}

// ExecuteWorkflowRobust executes a workflow with robust connection handling and state management
func (rwc *RobustWorkflowClient) ExecuteWorkflowRobust(ctx context.Context, req WorkflowExecutionRequest, runner string) (<-chan RobustWorkflowEvent, error) {
	// Create execution state
	state, err := rwc.stateManager.CreateExecution(req, runner)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution state: %w", err)
	}

	// Create event channel
	events := make(chan RobustWorkflowEvent, 100) // Buffered to handle bursts

	go func() {
		defer close(events)
		defer func() {
			// Cleanup on exit if not completed
			if state.Status == "running" {
				rwc.stateManager.CompleteExecution(state, "interrupted")
			}
		}()

		// Send initial event
		events <- RobustWorkflowEvent{
			Type:        "state",
			ExecutionID: state.ExecutionID,
			State:       state,
			Message:     fmt.Sprintf("Starting workflow execution: %s", state.WorkflowName),
		}

		// Start robust execution with retries
		rwc.executeWithRetries(ctx, state, events)
	}()

	return events, nil
}

// RobustWorkflowEvent represents an event from robust workflow execution
type RobustWorkflowEvent struct {
	Type        string                   `json:"type"` // "state", "step", "data", "error", "complete", "reconnect"
	ExecutionID string                   `json:"execution_id"`
	State       *WorkflowExecutionState  `json:"state,omitempty"`
	StepName    string                   `json:"step_name,omitempty"`
	StepStatus  string                   `json:"step_status,omitempty"`
	Data        string                   `json:"data,omitempty"`
	Message     string                   `json:"message,omitempty"`
	Error       string                   `json:"error,omitempty"`
	Reconnect   bool                     `json:"reconnect,omitempty"`
}

// executeWithRetries handles the workflow execution with connection recovery
func (rwc *RobustWorkflowClient) executeWithRetries(ctx context.Context, state *WorkflowExecutionState, events chan<- RobustWorkflowEvent) {
	rwc.executeWithRetriesOptions(ctx, state, events, RobustExecutionOptions{
		MaxRetries: 10,
		RetryDelay: 2 * time.Second,
	})
}

// executeWithRetriesOptions handles workflow execution with configurable retry options
func (rwc *RobustWorkflowClient) executeWithRetriesOptions(ctx context.Context, state *WorkflowExecutionState, events chan<- RobustWorkflowEvent, options RobustExecutionOptions) {
	maxRetries := options.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}
	
	baseDelay := options.RetryDelay
	if baseDelay <= 0 {
		baseDelay = 2 * time.Second
	}
	maxDelay := 30 * time.Second
	
	// If retries are disabled, only try once
	if options.NoRetry {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			if delay > maxDelay {
				delay = maxDelay
			}

			events <- RobustWorkflowEvent{
				Type:        "reconnect",
				ExecutionID: state.ExecutionID,
				Message:     fmt.Sprintf("Attempting to reconnect (attempt %d/%d) in %v...", attempt, maxRetries, delay),
				Reconnect:   true,
			}

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}

			rwc.stateManager.MarkConnectionLost(state)
		}

		// Try to execute/reconnect
		connectionSuccessful := rwc.attemptConnection(ctx, state, events, attempt > 0)
		
		if connectionSuccessful {
			// Check if workflow is actually complete
			if state.Status == "completed" || state.Status == "failed" {
				return
			}
			
			// If we successfully connected but didn't get completion, continue monitoring
			continue
		}

		// Connection failed
		if attempt == maxRetries {
			events <- RobustWorkflowEvent{
				Type:        "error",
				ExecutionID: state.ExecutionID,
				Error:       fmt.Sprintf("Failed to reconnect after %d attempts", maxRetries),
			}
			rwc.stateManager.CompleteExecution(state, "failed")
			return
		}
	}
}

// attemptConnection attempts to connect to the workflow stream
func (rwc *RobustWorkflowClient) attemptConnection(ctx context.Context, state *WorkflowExecutionState, events chan<- RobustWorkflowEvent, isReconnect bool) bool {
	if rwc.debug {
		if isReconnect {
			fmt.Printf("[DEBUG] Attempting to reconnect to workflow %s\n", state.ExecutionID)
		} else {
			fmt.Printf("[DEBUG] Starting workflow execution %s\n", state.ExecutionID)
		}
	}

	// NO timeout for workflow execution - let it run indefinitely with heartbeat monitoring
	// Only use timeout for initial connection (not execution)
	connCtx := ctx

	// Execute the workflow - for reconnections, we should check if this is a new execution
	// or resuming an existing one. For now, treat reconnections as monitoring existing execution
	var workflowEvents <-chan WorkflowSSEEvent
	var err error
	
	if isReconnect {
		// For reconnections, we should ideally have a way to reconnect to existing execution
		// For now, we'll try to execute again but this needs a better solution
		if rwc.debug {
			fmt.Printf("[DEBUG] Reconnection attempt - this may create duplicate execution\n")
		}
		workflowEvents, err = rwc.client.ExecuteWorkflow(connCtx, *state.Request, state.Runner)
	} else {
		// Fresh execution
		workflowEvents, err = rwc.client.ExecuteWorkflow(connCtx, *state.Request, state.Runner)
	}
	
	if err != nil {
		events <- RobustWorkflowEvent{
			Type:        "error",
			ExecutionID: state.ExecutionID,
			Error:       fmt.Sprintf("Failed to start workflow execution: %v", err),
		}
		return false
	}

	if isReconnect {
		rwc.stateManager.MarkConnectionRestored(state)
		events <- RobustWorkflowEvent{
			Type:        "reconnect",
			ExecutionID: state.ExecutionID,
			Message:     "Connection restored, resuming workflow monitoring",
			Reconnect:   false,
		}
	}

	// Process events with heartbeat monitoring (much longer timeout)
	lastEventTime := time.Now()
	connectionTimeout := 5 * time.Minute // No events for 5 minutes = connection lost (much longer for workflows)

	// Start a goroutine to monitor for connection timeouts
	timeoutCtx, timeoutCancel := context.WithCancel(ctx)
	defer timeoutCancel()
	
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-timeoutCtx.Done():
				return
			case <-ticker.C:
				if time.Since(lastEventTime) > connectionTimeout {
					if rwc.debug {
						fmt.Printf("[DEBUG] Connection timeout detected for execution %s\n", state.ExecutionID)
					}
					timeoutCancel()
					return
				}
			}
		}
	}()

	// Process workflow events
	for {
		select {
		case <-timeoutCtx.Done():
			// Connection timeout or context cancelled
			return false
			
		case event, ok := <-workflowEvents:
			if !ok {
				// Channel closed, check if workflow completed
				if rwc.debug {
					fmt.Printf("[DEBUG] Workflow event channel closed for execution %s\n", state.ExecutionID)
				}
				return state.Status == "completed" || state.Status == "failed"
			}

			lastEventTime = time.Now()
			
			// Process the event and update state
			completed := rwc.processWorkflowEvent(event, state, events)
			if completed {
				return true
			}
		}
	}
}

// processWorkflowEvent processes a single workflow event and updates state
func (rwc *RobustWorkflowClient) processWorkflowEvent(event WorkflowSSEEvent, state *WorkflowExecutionState, events chan<- RobustWorkflowEvent) bool {
	switch event.Type {
	case "data":
		// Try to parse as JSON for structured output
		var jsonData map[string]interface{}
		if err := json.Unmarshal([]byte(event.Data), &jsonData); err == nil {
			// Handle different event types from the API
			if eventType, ok := jsonData["type"].(string); ok {
				switch eventType {
				case "step_running":
					if step, ok := jsonData["step"].(map[string]interface{}); ok {
						if stepName, ok := step["name"].(string); ok {
							rwc.stateManager.UpdateStepStatus(state, stepName, "running", "", "")
							
							events <- RobustWorkflowEvent{
								Type:        "step",
								ExecutionID: state.ExecutionID,
								StepName:    stepName,
								StepStatus:  "running",
								State:       state,
								Message:     fmt.Sprintf("Step started: %s", stepName),
							}
						}
					}

				case "step_complete":
					if step, ok := jsonData["step"].(map[string]interface{}); ok {
						var stepName, output, status string
						
						if name, ok := step["name"].(string); ok {
							stepName = name
						}
						if out, ok := step["output"].(string); ok {
							output = out
						}
						if stat, ok := step["status"].(string); ok {
							status = stat
						}

						// Update state
						if status == "finished" {
							rwc.stateManager.UpdateStepStatus(state, stepName, "completed", output, "")
						} else if status == "failed" {
							rwc.stateManager.UpdateStepStatus(state, stepName, "failed", output, "step execution failed")
						}

						events <- RobustWorkflowEvent{
							Type:        "step",
							ExecutionID: state.ExecutionID,
							StepName:    stepName,
							StepStatus:  status,
							Data:        output,
							State:       state,
							Message:     fmt.Sprintf("Step completed: %s", stepName),
						}
					}

				case "workflow_complete":
					// Extract execution ID if available
					if requestId, ok := jsonData["requestId"].(string); ok {
						state.ExecutionID = requestId
					}

					// Determine final status
					finalStatus := "failed"
					if status, ok := jsonData["status"].(string); ok {
						if status == "finished" && jsonData["success"] == true {
							finalStatus = "completed"
						}
					}

					// Complete the execution
					rwc.stateManager.CompleteExecution(state, finalStatus)

					events <- RobustWorkflowEvent{
						Type:        "complete",
						ExecutionID: state.ExecutionID,
						State:       state,
						Message:     fmt.Sprintf("Workflow %s: %s", finalStatus, state.WorkflowName),
					}

					return true // Workflow completed
				}
			}
		} else {
			// Plain text output
			events <- RobustWorkflowEvent{
				Type:        "data",
				ExecutionID: state.ExecutionID,
				Data:        event.Data,
				Message:     "Raw workflow output",
			}
		}

	case "error":
		// Extract more detailed error information
		errorDetails := rwc.analyzeError(event.Data)
		events <- RobustWorkflowEvent{
			Type:        "error",
			ExecutionID: state.ExecutionID,
			Error:       event.Data,
			Message:     fmt.Sprintf("Workflow execution error: %s", errorDetails),
		}
		// Mark workflow as failed for context deadline exceeded errors
		if strings.Contains(event.Data, "context deadline exceeded") {
			rwc.stateManager.CompleteExecution(state, "failed")
			return true
		}
		// For other errors, don't return true - let retry logic handle it

	case "done":
		// Check if workflow actually completed
		if state.Status == "completed" || state.Status == "failed" {
			return true
		}
		// If not completed but stream ended, we'll need to reconnect
		return false
	}

	// Update last known event
	state.LastKnownEvent = &event
	rwc.stateManager.SaveExecution(state)

	return false
}

// ResumeExecution resumes a previously interrupted execution
func (rwc *RobustWorkflowClient) ResumeExecution(ctx context.Context, executionID string) (<-chan RobustWorkflowEvent, error) {
	// Load execution state
	state, err := rwc.stateManager.LoadExecution(executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load execution state: %w", err)
	}

	if state.Status == "completed" || state.Status == "failed" {
		return nil, fmt.Errorf("execution %s is already %s", executionID, state.Status)
	}

	// Create event channel
	events := make(chan RobustWorkflowEvent, 100)

	go func() {
		defer close(events)

		// Send resume event
		events <- RobustWorkflowEvent{
			Type:        "state",
			ExecutionID: state.ExecutionID,
			State:       state,
			Message:     fmt.Sprintf("Resuming workflow execution: %s", state.WorkflowName),
		}

		// Continue execution with retries
		rwc.executeWithRetries(ctx, state, events)
	}()

	return events, nil
}

// ListActiveExecutions returns all active executions
func (rwc *RobustWorkflowClient) ListActiveExecutions() ([]*WorkflowExecutionState, error) {
	return rwc.stateManager.ListActiveExecutions()
}

// CleanupOldExecutions cleans up old execution state files
func (rwc *RobustWorkflowClient) CleanupOldExecutions(maxAge time.Duration) error {
	return rwc.stateManager.CleanupOldExecutions(maxAge)
}

// GetExecutionState returns the current state of an execution
func (rwc *RobustWorkflowClient) GetExecutionState(executionID string) (*WorkflowExecutionState, error) {
	return rwc.stateManager.LoadExecution(executionID)
}

// analyzeError provides detailed error analysis for better user feedback
func (rwc *RobustWorkflowClient) analyzeError(errorData string) string {
	if errorData == "" {
		return "Unknown error occurred"
	}
	
	// Parse common error patterns and provide helpful explanations
	if strings.Contains(errorData, "context deadline exceeded") {
		return "Connection timeout - workflow step took too long or connection was lost. This may indicate a step failure or network issue."
	}
	if strings.Contains(errorData, "RUNNER_CONFIG_ERROR") {
		return "Runner configuration error - the specified runner may not exist or be unavailable"
	}
	if strings.Contains(errorData, "connection refused") {
		return "Service unavailable - unable to connect to workflow execution service"
	}
	if strings.Contains(errorData, "unauthorized") || strings.Contains(errorData, "403") {
		return "Authentication error - check your API credentials"
	}
	if strings.Contains(errorData, "500") {
		return "Server error - temporary issue with the workflow execution service"
	}
	if strings.Contains(errorData, "failed to fetch runner") {
		return "Runner not found - ensure the specified runner exists and is available"
	}
	
	// Try to extract JSON error messages
	if strings.Contains(errorData, "\"error\":") {
		// Extract the error message from JSON
		start := strings.Index(errorData, "\"error\":\"")
		if start != -1 {
			start += 9 // Skip past "error":"
			end := strings.Index(errorData[start:], "\"")
			if end != -1 {
				return errorData[start : start+end]
			}
		}
	}
	
	// Return first 200 characters of error for debugging
	if len(errorData) > 200 {
		return errorData[:200] + "..."
	}
	return errorData
}

// ExecuteWorkflowRobustWithOptions executes a workflow with configurable retry options
func (rwc *RobustWorkflowClient) ExecuteWorkflowRobustWithOptions(ctx context.Context, req WorkflowExecutionRequest, runner string, options RobustExecutionOptions) (<-chan RobustWorkflowEvent, error) {
	// Create execution state
	state, err := rwc.stateManager.CreateExecution(req, runner)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution state: %w", err)
	}

	// Create event channel
	events := make(chan RobustWorkflowEvent, 100)

	go func() {
		defer close(events)
		defer func() {
			if state.Status == "running" {
				rwc.stateManager.CompleteExecution(state, "interrupted")
			}
		}()

		// Send initial event
		events <- RobustWorkflowEvent{
			Type:        "state",
			ExecutionID: state.ExecutionID,
			State:       state,
			Message:     fmt.Sprintf("Starting workflow execution: %s", state.WorkflowName),
		}

		// Start robust execution with configured retries
		rwc.executeWithRetriesOptions(ctx, state, events, options)
	}()

	return events, nil
}

// ExecuteWorkflowFromStepWithOptions executes a workflow from a specific step with options
func (rwc *RobustWorkflowClient) ExecuteWorkflowFromStepWithOptions(ctx context.Context, req WorkflowExecutionRequest, runner string, startFromStep string, options RobustExecutionOptions) (<-chan RobustWorkflowEvent, error) {
	// Validate that the step exists
	stepExists := false
	startStepIndex := 0
	for i, step := range req.Steps {
		if stepMap, ok := step.(map[string]interface{}); ok {
			if name, ok := stepMap["name"].(string); ok {
				if name == startFromStep {
					stepExists = true
					startStepIndex = i
					break
				}
			}
		}
	}
	
	if !stepExists {
		return nil, fmt.Errorf("step '%s' not found in workflow", startFromStep)
	}
	
	// Create a modified request with steps starting from the specified step
	modifiedReq := req
	modifiedReq.Steps = req.Steps[startStepIndex:]
	modifiedReq.Name = fmt.Sprintf("%s (from step %s)", req.Name, startFromStep)
	
	// Create execution state
	state, err := rwc.stateManager.CreateExecution(modifiedReq, runner)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution state: %w", err)
	}
	
	// Mark previous steps as skipped
	for i := 0; i < startStepIndex; i++ {
		if stepMap, ok := req.Steps[i].(map[string]interface{}); ok {
			if name, ok := stepMap["name"].(string); ok {
				rwc.stateManager.UpdateStepStatus(state, name, "skipped", "", "Step skipped - starting from later step")
			}
		}
	}
	
	// Create event channel
	events := make(chan RobustWorkflowEvent, 100)
	
	go func() {
		defer close(events)
		defer func() {
			if state.Status == "running" {
				rwc.stateManager.CompleteExecution(state, "interrupted")
			}
		}()
		
		// Send initial event
		events <- RobustWorkflowEvent{
			Type:        "state",
			ExecutionID: state.ExecutionID,
			State:       state,
			Message:     fmt.Sprintf("Starting workflow execution from step '%s': %s", startFromStep, state.WorkflowName),
		}
		
		// Start robust execution with configured retries
		rwc.executeWithRetriesOptions(ctx, state, events, options)
	}()
	
	return events, nil
}

// ExecuteWorkflowFromStep executes a workflow starting from a specific step (deprecated - use ExecuteWorkflowFromStepWithOptions)
func (rwc *RobustWorkflowClient) ExecuteWorkflowFromStep(ctx context.Context, req WorkflowExecutionRequest, runner string, startFromStep string) (<-chan RobustWorkflowEvent, error) {
	return rwc.ExecuteWorkflowFromStepWithOptions(ctx, req, runner, startFromStep, RobustExecutionOptions{
		MaxRetries: 10,
		RetryDelay: 2 * time.Second,
	})
}