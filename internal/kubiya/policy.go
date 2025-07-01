package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Policy represents an OPA policy
type Policy struct {
	Name   string   `json:"name"`
	Env    []string `json:"env"`
	Policy string   `json:"policy"`
}

// PolicyValidationRequest represents a policy validation request
type PolicyValidationRequest struct {
	Name   string   `json:"name"`
	Env    []string `json:"env"`
	Policy string   `json:"policy"`
}

// PolicyValidationResponse represents a policy validation response
type PolicyValidationResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

// PolicyEvaluationRequest represents a policy evaluation request
type PolicyEvaluationRequest struct {
	Input  map[string]interface{} `json:"input"`
	Policy string                 `json:"policy"`
	Data   map[string]interface{} `json:"data"`
	Query  string                 `json:"query"`
}

// PolicyEvaluationResponse represents a policy evaluation response
type PolicyEvaluationResponse struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error"`
}

// PolicyDeleteResponse represents a policy deletion response
type PolicyDeleteResponse struct {
	Status string `json:"status"`
}

// CreatePolicy creates a new OPA policy
func (c *Client) CreatePolicy(ctx context.Context, policy Policy) (*Policy, error) {
	body, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal policy: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/opa/policies", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var result Policy
	if err := c.do(req, &result); err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}

	return &result, nil
}

// ValidatePolicy validates an OPA policy
func (c *Client) ValidatePolicy(ctx context.Context, validation PolicyValidationRequest) (*PolicyValidationResponse, error) {
	body, err := json.Marshal(validation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/opa/policies/validate", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var result PolicyValidationResponse
	if err := c.do(req, &result); err != nil {
		return nil, fmt.Errorf("failed to validate policy: %w", err)
	}

	return &result, nil
}

// UpdatePolicy updates an existing OPA policy
func (c *Client) UpdatePolicy(ctx context.Context, nameOrID string, policy Policy) (*Policy, error) {
	body, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal policy: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/opa/policies/%s", c.baseURL, nameOrID), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var result Policy
	if err := c.do(req, &result); err != nil {
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}

	return &result, nil
}

// ListPolicies lists all OPA policies
func (c *Client) ListPolicies(ctx context.Context) ([]Policy, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/opa/policies", c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var policies []Policy
	if err := c.do(req, &policies); err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}

	return policies, nil
}

// GetPolicy retrieves a specific OPA policy
func (c *Client) GetPolicy(ctx context.Context, nameOrID string) (*Policy, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/opa/policies/%s", c.baseURL, nameOrID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var policy Policy
	if err := c.do(req, &policy); err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	return &policy, nil
}

// DeletePolicy deletes an OPA policy
func (c *Client) DeletePolicy(ctx context.Context, nameOrID string) (*PolicyDeleteResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/opa/policies/%s", c.baseURL, nameOrID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var result PolicyDeleteResponse
	if err := c.do(req, &result); err != nil {
		return nil, fmt.Errorf("failed to delete policy: %w", err)
	}

	return &result, nil
}

// EvaluatePolicy evaluates a policy with given input
func (c *Client) EvaluatePolicy(ctx context.Context, evaluation PolicyEvaluationRequest) (*PolicyEvaluationResponse, error) {
	body, err := json.Marshal(evaluation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal evaluation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/opa/policies/evaluate", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var result PolicyEvaluationResponse
	if err := c.do(req, &result); err != nil {
		return nil, fmt.Errorf("failed to evaluate policy: %w", err)
	}

	return &result, nil
}

// ValidateToolExecution validates if a user can execute a specific tool with given parameters
func (c *Client) ValidateToolExecution(ctx context.Context, toolName string, args map[string]interface{}, runner string) (bool, string, error) {
	input := map[string]interface{}{
		"action":    "tool_execution",
		"tool_name": toolName,
		"args":      args,
		"runner":    runner,
		"user":      map[string]interface{}{
			// Add user context if available
		},
	}

	// Create a generic policy evaluation for tool execution
	evaluation := PolicyEvaluationRequest{
		Input: input,
		Query: "data.tools.allow", // Standard query for tool permissions
		Data:  map[string]interface{}{},
	}

	result, err := c.EvaluatePolicy(ctx, evaluation)
	if err != nil {
		return false, fmt.Sprintf("Policy evaluation failed: %v", err), err
	}

	if result.Error != "" {
		return false, result.Error, nil
	}

	// Check if result indicates permission granted
	if allowed, ok := result.Result.(bool); ok {
		return allowed, "", nil
	}

	// If result is complex, try to extract permission
	if resultMap, ok := result.Result.(map[string]interface{}); ok {
		if allowed, exists := resultMap["allow"]; exists {
			if allowedBool, ok := allowed.(bool); ok {
				message := ""
				if msg, exists := resultMap["message"]; exists {
					if msgStr, ok := msg.(string); ok {
						message = msgStr
					}
				}
				return allowedBool, message, nil
			}
		}
	}

	return false, "Policy evaluation result format not recognized", nil
}

// ValidateWorkflowExecution validates if a user can execute a workflow end-to-end
func (c *Client) ValidateWorkflowExecution(ctx context.Context, workflowDef map[string]interface{}, params map[string]interface{}, runner string) (bool, []string, error) {
	var issues []string
	
	// Extract workflow steps
	steps, ok := workflowDef["steps"].([]interface{})
	if !ok {
		return false, []string{"Invalid workflow definition: steps not found or not an array"}, nil
	}

	// Validate each step in the workflow
	for i, stepInterface := range steps {
		step, ok := stepInterface.(map[string]interface{})
		if !ok {
			issues = append(issues, fmt.Sprintf("Step %d: Invalid step format", i+1))
			continue
		}

		stepName, _ := step["name"].(string)
		if stepName == "" {
			stepName = fmt.Sprintf("step_%d", i+1)
		}

		// Check if step has tool execution
		if executor, ok := step["executor"].(map[string]interface{}); ok {
			if executorType, ok := executor["type"].(string); ok && executorType == "tool" {
				// Extract tool definition
				if config, ok := executor["config"].(map[string]interface{}); ok {
					if toolDef, ok := config["tool_def"].(map[string]interface{}); ok {
						toolName, _ := toolDef["name"].(string)
						if toolName == "" {
							issues = append(issues, fmt.Sprintf("Step '%s': Tool name missing", stepName))
							continue
						}

						// Extract tool args
						toolArgs := make(map[string]interface{})
						if args, ok := config["args"].(map[string]interface{}); ok {
							toolArgs = args
						}

						// Validate tool execution permission
						allowed, message, err := c.ValidateToolExecution(ctx, toolName, toolArgs, runner)
						if err != nil {
							issues = append(issues, fmt.Sprintf("Step '%s': Permission check failed for tool '%s': %v", stepName, toolName, err))
							continue
						}

						if !allowed {
							errorMsg := fmt.Sprintf("Step '%s': No permission to execute tool '%s'", stepName, toolName)
							if message != "" {
								errorMsg += fmt.Sprintf(" - %s", message)
							}
							issues = append(issues, errorMsg)
						}
					}
				}
			}
		}
	}

	// Overall workflow validation
	input := map[string]interface{}{
		"action":       "workflow_execution",
		"workflow_def": workflowDef,
		"params":       params,
		"runner":       runner,
		"user":         map[string]interface{}{
			// Add user context if available
		},
	}

	evaluation := PolicyEvaluationRequest{
		Input: input,
		Query: "data.workflows.allow", // Standard query for workflow permissions
		Data:  map[string]interface{}{},
	}

	result, err := c.EvaluatePolicy(ctx, evaluation)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Workflow policy evaluation failed: %v", err))
		return false, issues, nil
	}

	if result.Error != "" {
		issues = append(issues, fmt.Sprintf("Workflow policy error: %s", result.Error))
		return false, issues, nil
	}

	// If we have step-level issues but overall policy allows, return with warnings
	if len(issues) > 0 {
		return false, issues, nil
	}

	// Check overall workflow permission
	if allowed, ok := result.Result.(bool); ok {
		return allowed, issues, nil
	}

	if resultMap, ok := result.Result.(map[string]interface{}); ok {
		if allowed, exists := resultMap["allow"]; exists {
			if allowedBool, ok := allowed.(bool); ok {
				if !allowedBool {
					if msg, exists := resultMap["message"]; exists {
						if msgStr, ok := msg.(string); ok {
							issues = append(issues, msgStr)
						}
					}
				}
				return allowedBool, issues, nil
			}
		}
	}

	return true, issues, nil // Default to allow if policy format not recognized
}