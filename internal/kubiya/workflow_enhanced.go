package kubiya

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// EnhancedWorkflowClient provides robust workflow operations with proper error handling
type EnhancedWorkflowClient struct {
	client     *Client
	daguClient *DAGUClient
}

// DAGUClient handles direct integration with DAGU API
type DAGUClient struct {
	baseURL string
	client  *http.Client
	apiKey  string
}

// WorkflowStatusResponse represents the full status response from the API
type WorkflowStatusResponse struct {
	RequestID  string                 `json:"requestId"`
	Success    bool                   `json:"success"`
	Timestamp  string                 `json:"timestamp"`
	Status     *DetailedWorkflowStatus `json:"status"`
	IsActive   bool                   `json:"isActive"`
	WorkflowID string                 `json:"workflowId"`
	Error      *APIError              `json:"error,omitempty"`
}

// DetailedWorkflowStatus provides comprehensive workflow status information
type DetailedWorkflowStatus struct {
	Name        string                        `json:"name"`
	Status      int                          `json:"status"`
	StatusText  string                       `json:"statusText"`
	StartedAt   string                       `json:"startedAt"`
	FinishedAt  string                       `json:"finishedAt,omitempty"`
	Duration    string                       `json:"duration,omitempty"`
	Nodes       map[string]*DetailedStepStatus `json:"nodes"`
	Variables   map[string]interface{}        `json:"variables,omitempty"`
	Error       string                       `json:"error,omitempty"`
	RetryCount  int                          `json:"retryCount,omitempty"`
	CanRetry    bool                         `json:"canRetry,omitempty"`
	LogURL      string                       `json:"logUrl,omitempty"`
	TraceURL    string                       `json:"traceUrl,omitempty"`
}

// DetailedStepStatus provides comprehensive step status information
type DetailedStepStatus struct {
	Name       string                 `json:"name"`
	Status     string                 `json:"status"`
	ExitCode   int                    `json:"exitCode"`
	StartedAt  string                 `json:"startedAt"`
	FinishedAt string                 `json:"finishedAt,omitempty"`
	Duration   string                 `json:"duration,omitempty"`
	Output     string                 `json:"output,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Logs       []string               `json:"logs,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	RetryCount int                    `json:"retryCount,omitempty"`
	CanRetry   bool                   `json:"canRetry,omitempty"`
}

// WorkflowListResponse represents the list workflows response
type WorkflowListResponse struct {
	RequestID  string              `json:"requestId"`
	Success    bool                `json:"success"`
	Timestamp  string              `json:"timestamp"`
	Workflows  []WorkflowSummary   `json:"workflows"`
	Pagination *PaginationInfo     `json:"pagination,omitempty"`
	Errors     []string            `json:"errors,omitempty"`
}

// WorkflowSummary provides summary information about a workflow
type WorkflowSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Location    string    `json:"location"`
	UpdatedAt   string    `json:"updatedAt"`
	Tags        []string  `json:"tags,omitempty"`
	Group       string    `json:"group,omitempty"`
	StepCount   int       `json:"stepCount"`
	Runner      string    `json:"runner,omitempty"`
}

// PaginationInfo provides pagination details
type PaginationInfo struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Page   int `json:"page"`
}

// RetryWorkflowRequest represents a workflow retry request
type RetryWorkflowRequest struct {
	WorkflowID string                 `json:"workflowId"`
	FromStep   string                 `json:"fromStep,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	Force      bool                   `json:"force,omitempty"`
}

// CancelWorkflowRequest represents a workflow cancellation request
type CancelWorkflowRequest struct {
	WorkflowID string `json:"workflowId"`
	Reason     string `json:"reason,omitempty"`
}

// APIError represents structured error information from the API
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
	Retry      bool   `json:"retry,omitempty"`
	RetryAfter int    `json:"retryAfter,omitempty"`
}

// WorkflowError represents workflow-specific errors
type WorkflowError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Step    string `json:"step,omitempty"`
	Retry   bool   `json:"retry,omitempty"`
}

// WorkflowStepEvent represents step-level events
type WorkflowStepEvent struct {
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	Output      string                 `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ExitCode    int                    `json:"exitCode,omitempty"`
	StartedAt   string                 `json:"startedAt,omitempty"`
	FinishedAt  string                 `json:"finishedAt,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Logs        []string               `json:"logs,omitempty"`
	RetryCount  int                    `json:"retryCount,omitempty"`
	CanRetry    bool                   `json:"canRetry,omitempty"`
}

// WorkflowProgress represents workflow execution progress
type WorkflowProgress struct {
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
	CurrentStep string `json:"currentStep"`
	Percentage  int    `json:"percentage"`
}

// NewEnhancedWorkflowClient creates a new enhanced workflow client
func NewEnhancedWorkflowClient(client *Client) *EnhancedWorkflowClient {
	daguClient := &DAGUClient{
		baseURL: client.baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: client.cfg.APIKey,
	}

	return &EnhancedWorkflowClient{
		client:     client,
		daguClient: daguClient,
	}
}

// WorkflowEnhanced returns the enhanced workflow client
func (c *Client) WorkflowEnhanced() *EnhancedWorkflowClient {
	return NewEnhancedWorkflowClient(c)
}

// ExecuteWorkflowEnhanced executes a workflow with enhanced error handling and reconnection
func (ewc *EnhancedWorkflowClient) ExecuteWorkflowEnhanced(ctx context.Context, req WorkflowExecutionRequest, runner string) (<-chan EnhancedWorkflowEvent, error) {
	// Add enhanced error tracking
	if req.Env == nil {
		req.Env = make(map[string]interface{})
	}
	req.Env["KUBIYA_ENHANCED_ERRORS"] = "true"
	req.Env["KUBIYA_STREAM_LOGS"] = "true"
	req.Env["KUBIYA_TRACE_ENABLED"] = "true"

	events := make(chan EnhancedWorkflowEvent, 100)
	
	go func() {
		defer close(events)
		
		// Implement retry logic for connection failures
		maxRetries := 3
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				select {
				case events <- EnhancedWorkflowEvent{
					Type: "reconnecting",
					Data: map[string]interface{}{
						"attempt": attempt + 1,
						"max":     maxRetries,
					},
				}:
				case <-ctx.Done():
					return
				}
				
				// Exponential backoff
				backoff := time.Duration(attempt) * time.Second
				timer := time.NewTimer(backoff)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return
				}
			}

			err := ewc.executeWithRetry(ctx, req, runner, events)
			if err == nil {
				return
			}

			// Check if we should retry
			if !shouldRetryError(err) || attempt == maxRetries-1 {
				select {
				case events <- EnhancedWorkflowEvent{
					Type: "error",
					Error: &WorkflowError{
						Code:    "execution_failed",
						Message: err.Error(),
						Retry:   attempt < maxRetries-1,
					},
				}:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	return events, nil
}

// executeWithRetry performs a single execution attempt
func (ewc *EnhancedWorkflowClient) executeWithRetry(ctx context.Context, req WorkflowExecutionRequest, runner string, events chan<- EnhancedWorkflowEvent) error {
	// Build URL with query parameters
	params := url.Values{}
	params.Set("runner", runner)
	params.Set("operation", "execute_workflow")

	baseURL := strings.TrimSuffix(ewc.client.baseURL, "/")
	if strings.HasSuffix(baseURL, "/api/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/api/v1")
	}
	executeURL := fmt.Sprintf("%s/api/v1/workflow?%s", baseURL, params.Encode())

	// Create request body
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, executeURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for enhanced streaming
	httpReq.Header.Set("Authorization", "UserKey "+ewc.client.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-vercel-ai-data-stream", "v1")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	httpReq.Header.Set("X-Request-Enhanced", "true")

	// Execute request
	client := &http.Client{Timeout: 0} // No timeout for streaming
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("workflow execution failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Process streaming response
	return ewc.processEnhancedStream(ctx, resp.Body, events)
}

// processEnhancedStream processes the enhanced SSE stream
func (ewc *EnhancedWorkflowClient) processEnhancedStream(ctx context.Context, body io.Reader, events chan<- EnhancedWorkflowEvent) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "retry:") {
			continue
		}

		// Parse enhanced SSE format
		if len(line) > 2 && line[1] == ':' {
			eventType := string(line[0])
			data := line[2:]

			var event EnhancedWorkflowEvent
			switch eventType {
			case "2": // Data event
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					// Fallback to basic event processing
					event = EnhancedWorkflowEvent{
						Type: "data",
						Data: map[string]interface{}{"raw": data},
					}
				}
			case "3": // Error event with details
				event = EnhancedWorkflowEvent{
					Type: "error",
					Error: &WorkflowError{
						Code:    "stream_error",
						Message: data,
					},
				}
			case "d": // Done event
				event = EnhancedWorkflowEvent{
					Type: "done",
					Data: map[string]interface{}{"reason": data},
				}
			case "e": // Error event
				event = EnhancedWorkflowEvent{
					Type: "error",
					Error: &WorkflowError{
						Code:    "execution_error",
						Message: data,
					},
				}
			default:
				continue
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return scanner.Err()
}

// GetDetailedStatus gets comprehensive workflow status
func (ewc *EnhancedWorkflowClient) GetDetailedStatus(ctx context.Context, workflowID, runner string) (*WorkflowStatusResponse, error) {
	req := map[string]interface{}{
		"workflowId": workflowID,
	}

	return ewc.callWorkflowAPI(ctx, "get_status", runner, req)
}

// StreamWorkflowLogs connects to an existing workflow and streams logs
func (ewc *EnhancedWorkflowClient) StreamWorkflowLogs(ctx context.Context, workflowID, runner string) (<-chan EnhancedWorkflowEvent, error) {
	events := make(chan EnhancedWorkflowEvent, 100)
	
	go func() {
		defer close(events)
		
		// First get current status
		statusResp, err := ewc.GetDetailedStatus(ctx, workflowID, runner)
		if err != nil {
			select {
			case events <- EnhancedWorkflowEvent{
				Type: "error",
				Error: &WorkflowError{
					Code:    "status_error",
					Message: fmt.Sprintf("Failed to get workflow status: %v", err),
				},
			}:
			case <-ctx.Done():
			}
			return
		}

		// Send initial status
		select {
		case events <- EnhancedWorkflowEvent{
			Type:       "status",
			WorkflowID: workflowID,
			Data: map[string]interface{}{
				"status":    statusResp.Status,
				"isActive":  statusResp.IsActive,
				"timestamp": statusResp.Timestamp,
			},
		}:
		case <-ctx.Done():
			return
		}

		// If workflow is not active, return historical data and exit
		if !statusResp.IsActive {
			for stepName, stepStatus := range statusResp.Status.Nodes {
				select {
				case events <- EnhancedWorkflowEvent{
					Type:       "step_history",
					WorkflowID: workflowID,
					Step: &WorkflowStepEvent{
						Name:       stepName,
						Status:     stepStatus.Status,
						Output:     stepStatus.Output,
						Error:      stepStatus.Error,
						ExitCode:   stepStatus.ExitCode,
						StartedAt:  stepStatus.StartedAt,
						FinishedAt: stepStatus.FinishedAt,
						Duration:   stepStatus.Duration,
						Variables:  stepStatus.Variables,
						Logs:       stepStatus.Logs,
					},
				}:
				case <-ctx.Done():
					return
				}
			}
			return
		}

		// For active workflows, implement streaming
		ewc.streamActiveWorkflow(ctx, workflowID, runner, events)
	}()

	return events, nil
}

// streamActiveWorkflow streams logs from an active workflow
func (ewc *EnhancedWorkflowClient) streamActiveWorkflow(ctx context.Context, workflowID, runner string, events chan<- EnhancedWorkflowEvent) {
	// Implementation would depend on the actual streaming endpoint
	// This could be polling or a dedicated streaming endpoint
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastStatus *WorkflowStatusResponse
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, err := ewc.GetDetailedStatus(ctx, workflowID, runner)
			if err != nil {
				select {
				case events <- EnhancedWorkflowEvent{
					Type: "error",
					Error: &WorkflowError{
						Code:    "polling_error",
						Message: fmt.Sprintf("Failed to poll status: %v", err),
						Retry:   true,
					},
				}:
				case <-ctx.Done():
				}
				continue
			}

			// Compare with last status and send diff events
			if lastStatus == nil || status.Status.StatusText != lastStatus.Status.StatusText {
				select {
				case events <- EnhancedWorkflowEvent{
					Type:       "status_change",
					WorkflowID: workflowID,
					Data: map[string]interface{}{
						"status":    status.Status,
						"timestamp": status.Timestamp,
					},
				}:
				case <-ctx.Done():
					return
				}
			}

			// Send step updates
			if lastStatus != nil {
				for stepName, stepStatus := range status.Status.Nodes {
					if lastStatus.Status.Nodes[stepName] == nil || 
					   lastStatus.Status.Nodes[stepName].Status != stepStatus.Status {
						select {
						case events <- EnhancedWorkflowEvent{
							Type:       "step_update",
							WorkflowID: workflowID,
							Step: &WorkflowStepEvent{
								Name:       stepName,
								Status:     stepStatus.Status,
								Output:     stepStatus.Output,
								Error:      stepStatus.Error,
								ExitCode:   stepStatus.ExitCode,
								StartedAt:  stepStatus.StartedAt,
								FinishedAt: stepStatus.FinishedAt,
								Duration:   stepStatus.Duration,
								Variables:  stepStatus.Variables,
								Logs:       stepStatus.Logs,
							},
						}:
						case <-ctx.Done():
							return
						}
					}
				}
			}

			lastStatus = status

			// Stop streaming if workflow is no longer active
			if !status.IsActive {
				select {
				case events <- EnhancedWorkflowEvent{
					Type:       "done",
					WorkflowID: workflowID,
					Data: map[string]interface{}{
						"final_status": status.Status.StatusText,
						"timestamp":    status.Timestamp,
					},
				}:
				case <-ctx.Done():
				}
				return
			}
		}
	}
}

// RetryWorkflow retries a workflow from a specific step
func (ewc *EnhancedWorkflowClient) RetryWorkflow(ctx context.Context, req RetryWorkflowRequest, runner string) (<-chan EnhancedWorkflowEvent, error) {
	// Call the retry API
	apiReq := map[string]interface{}{
		"workflowId": req.WorkflowID,
		"fromStep":   req.FromStep,
		"variables":  req.Variables,
		"force":      req.Force,
	}

	_, err := ewc.callWorkflowAPI(ctx, "retry_workflow", runner, apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to retry workflow: %w", err)
	}

	// Stream the retried workflow execution
	return ewc.StreamWorkflowLogs(ctx, req.WorkflowID, runner)
}

// CancelWorkflow cancels a running workflow
func (ewc *EnhancedWorkflowClient) CancelWorkflow(ctx context.Context, req CancelWorkflowRequest, runner string) error {
	apiReq := map[string]interface{}{
		"workflowId": req.WorkflowID,
		"reason":     req.Reason,
	}

	_, err := ewc.callWorkflowAPI(ctx, "cancel_workflow", runner, apiReq)
	return err
}

// ListWorkflows lists workflows with filtering
func (ewc *EnhancedWorkflowClient) ListWorkflows(ctx context.Context, filter, runner string, limit, offset int) (*WorkflowListResponse, error) {
	req := map[string]interface{}{
		"filter": filter,
		"limit":  limit,
		"offset": offset,
	}

	// For cross-runner queries, this would need to aggregate results
	return ewc.callWorkflowListAPI(ctx, "list_workflows", runner, req)
}

// callWorkflowAPI makes a call to the workflow API
func (ewc *EnhancedWorkflowClient) callWorkflowAPI(ctx context.Context, operation, runner string, req map[string]interface{}) (*WorkflowStatusResponse, error) {
	params := url.Values{}
	params.Set("runner", runner)
	params.Set("operation", operation)

	baseURL := strings.TrimSuffix(ewc.client.baseURL, "/")
	if strings.HasSuffix(baseURL, "/api/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/api/v1")
	}
	apiURL := fmt.Sprintf("%s/api/v1/workflow?%s", baseURL, params.Encode())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "UserKey "+ewc.client.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := ewc.daguClient.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	var result WorkflowStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		if result.Error != nil {
			return nil, fmt.Errorf("API error: %s", result.Error.Message)
		}
		return nil, fmt.Errorf("API call failed")
	}

	return &result, nil
}

// callWorkflowListAPI makes a call to the workflow list API
func (ewc *EnhancedWorkflowClient) callWorkflowListAPI(ctx context.Context, operation, runner string, req map[string]interface{}) (*WorkflowListResponse, error) {
	params := url.Values{}
	params.Set("runner", runner)
	params.Set("operation", operation)

	baseURL := strings.TrimSuffix(ewc.client.baseURL, "/")
	if strings.HasSuffix(baseURL, "/api/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/api/v1")
	}
	apiURL := fmt.Sprintf("%s/api/v1/workflow?%s", baseURL, params.Encode())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "UserKey "+ewc.client.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := ewc.daguClient.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	var result WorkflowListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API call failed")
	}

	return &result, nil
}

// EnhancedWorkflowEvent represents enhanced workflow events
type EnhancedWorkflowEvent struct {
	Type       string                 `json:"type"`
	RequestID  string                 `json:"requestId,omitempty"`
	WorkflowID string                 `json:"workflowId,omitempty"`
	Timestamp  string                 `json:"timestamp,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Error      *WorkflowError         `json:"error,omitempty"`
	Step       *WorkflowStepEvent     `json:"step,omitempty"`
	Progress   *WorkflowProgress      `json:"progress,omitempty"`
}

// shouldRetryError determines if an error should trigger a retry
func shouldRetryError(err error) bool {
	errStr := strings.ToLower(err.Error())
	retryableErrors := []string{
		"connection reset",
		"timeout",
		"temporary failure",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
	}
	
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	
	return false
}