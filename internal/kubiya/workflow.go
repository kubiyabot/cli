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
)

// WorkflowClient handles workflow-specific operations
type WorkflowClient struct {
	client *Client
}

// NewWorkflowClient creates a new workflow client
func NewWorkflowClient(client *Client) *WorkflowClient {
	return &WorkflowClient{client: client}
}

// Workflow returns the workflow client
func (c *Client) Workflow() *WorkflowClient {
	return NewWorkflowClient(c)
}

// OrchestrateRequest represents a request to the orchestration API
type OrchestrateRequest struct {
	Format           string                 `json:"format,omitempty"`
	OrgID            string                 `json:"orgId,omitempty"`
	Prompt           string                 `json:"prompt"`
	UserID           string                 `json:"userID,omitempty"`
	ConversationID   string                 `json:"conversationId,omitempty"`
	EnableClaudeCode bool                   `json:"enable_claude_code,omitempty"`
	AnthropicAPIKey  string                 `json:"anthropic_api_key,omitempty"`
	MCPServers       []MCPServerConfig      `json:"mcp_servers,omitempty"`
	Variables        map[string]interface{} `json:"variables,omitempty"`
	WorkflowMode     string                 `json:"workflow_mode,omitempty"` // "act" or "plan"
	BearerToken      string                 `json:"bearer_token,omitempty"`
	IDToken          string                 `json:"idToken,omitempty"`
}

// MCPServerConfig represents MCP server configuration
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// WorkflowExecutionRequest represents a direct workflow execution request
type WorkflowExecutionRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Steps       []interface{}          `json:"steps"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Runner      string                 `json:"runner,omitempty"`
}

// WorkflowSSEEvent represents a workflow-specific SSE event
type WorkflowSSEEvent struct {
	Type     string
	Data     string
	Step     string
	Progress int
}

// GenerateWorkflow generates a workflow from a natural language prompt using the orchestration API
func (wc *WorkflowClient) GenerateWorkflow(ctx context.Context, prompt string, options OrchestrateRequest) (<-chan WorkflowSSEEvent, error) {
	// Use orchestrator API endpoint
	orchestratorURL := "https://api.kubiya.ai/api/orchestrate"

	// Set default values
	if options.Format == "" {
		options.Format = "sse"
	}
	options.Prompt = prompt

	// Add bearer token from client config
	if options.BearerToken == "" {
		options.BearerToken = wc.client.cfg.APIKey
	}

	// Create request body
	body, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, orchestratorURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Try both authentication methods
	if strings.HasPrefix(options.BearerToken, "ey") {
		// Looks like a JWT token, use Bearer
		req.Header.Set("Authorization", "Bearer "+options.BearerToken)
	} else {
		// Use UserKey format
		req.Header.Set("Authorization", "UserKey "+wc.client.cfg.APIKey)
	}

	// Execute request
	resp, err := wc.client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("orchestration failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Create channel for streaming events
	events := make(chan WorkflowSSEEvent)

	go func() {
		defer close(events)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Debug logging
			if wc.client.debug {
				fmt.Printf("[DEBUG] SSE Line: %s\n", line)
			}

			// Skip empty lines and retry messages
			if line == "" || strings.HasPrefix(line, "retry:") {
				continue
			}

			// Parse SSE format - the actual format is "2:{json}" or "d:{json}"
			if len(line) > 2 && line[1] == ':' {
				eventType := string(line[0])
				data := line[2:]

				switch eventType {
				case "2": // Data event
					events <- WorkflowSSEEvent{Type: "data", Data: data}
				case "d": // Done event
					events <- WorkflowSSEEvent{Type: "done", Data: data}
				case "e": // Error event
					events <- WorkflowSSEEvent{Type: "error", Data: data}
				default:
					// Unknown event type, treat as data
					events <- WorkflowSSEEvent{Type: "data", Data: data}
				}
			} else if strings.HasPrefix(line, "data: ") {
				// Standard SSE format
				data := strings.TrimPrefix(line, "data: ")
				events <- WorkflowSSEEvent{Type: "data", Data: data}
			} else if strings.HasPrefix(line, "event: ") {
				// Standard SSE event type
				eventType := strings.TrimPrefix(line, "event: ")
				events <- WorkflowSSEEvent{Type: eventType, Data: ""}
			}
		}

		if err := scanner.Err(); err != nil {
			if wc.client.debug {
				fmt.Printf("[DEBUG] Scanner error: %v\n", err)
			}
			events <- WorkflowSSEEvent{Type: "error", Data: err.Error()}
		}
	}()

	return events, nil
}

// ExecuteWorkflow executes a workflow directly using the Kubiya API (without orchestration)
func (wc *WorkflowClient) ExecuteWorkflow(ctx context.Context, req WorkflowExecutionRequest, runner string) (<-chan WorkflowSSEEvent, error) {
	// Default runner if not specified
	if runner == "" {
		runner = "core-testing-2"
	}

	// Build URL with query parameters
	params := url.Values{}
	params.Set("runner", runner)
	params.Set("operation", "execute_workflow")

	executeURL := fmt.Sprintf("%s/workflow?%s", wc.client.baseURL, params.Encode())

	// Create request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, executeURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Authorization", "UserKey "+wc.client.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	// Execute request
	resp, err := wc.client.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("workflow execution failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Create channel for streaming events
	events := make(chan WorkflowSSEEvent)

	go func() {
		defer close(events)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Debug logging
			if wc.client.debug {
				fmt.Printf("[DEBUG] SSE Line: %s\n", line)
			}

			// Skip empty lines and retry messages
			if line == "" || strings.HasPrefix(line, "retry:") {
				continue
			}

			// Parse SSE format - the actual format is "2:{json}" or "d:{json}"
			if len(line) > 2 && line[1] == ':' {
				eventType := string(line[0])
				data := line[2:]

				switch eventType {
				case "2": // Data event
					events <- WorkflowSSEEvent{Type: "data", Data: data}
				case "d": // Done event
					events <- WorkflowSSEEvent{Type: "done", Data: data}
				case "e": // Error event
					events <- WorkflowSSEEvent{Type: "error", Data: data}
				default:
					// Unknown event type, treat as data
					events <- WorkflowSSEEvent{Type: "data", Data: data}
				}
			} else if strings.HasPrefix(line, "data: ") {
				// Standard SSE format
				data := strings.TrimPrefix(line, "data: ")
				events <- WorkflowSSEEvent{Type: "data", Data: data}
			} else if strings.HasPrefix(line, "event: ") {
				// Standard SSE event type
				eventType := strings.TrimPrefix(line, "event: ")
				events <- WorkflowSSEEvent{Type: eventType, Data: ""}
			}
		}

		if err := scanner.Err(); err != nil {
			if wc.client.debug {
				fmt.Printf("[DEBUG] Scanner error: %v\n", err)
			}
			events <- WorkflowSSEEvent{Type: "error", Data: err.Error()}
		}
	}()

	return events, nil
}

// TestWorkflow tests a workflow by executing it in test mode
func (wc *WorkflowClient) TestWorkflow(ctx context.Context, req WorkflowExecutionRequest, runner string) (<-chan WorkflowSSEEvent, error) {
	// For testing, we use the same execution endpoint
	// You could add test-specific parameters here if needed
	return wc.ExecuteWorkflow(ctx, req, runner)
}

// ComposeAndExecute generates and executes a workflow from natural language using orchestration
func (wc *WorkflowClient) ComposeAndExecute(ctx context.Context, prompt string, options OrchestrateRequest) (<-chan WorkflowSSEEvent, error) {
	// This is the same as GenerateWorkflow but emphasizes the execution aspect
	// The orchestration API automatically executes the workflow based on the mode (act/plan)
	return wc.GenerateWorkflow(ctx, prompt, options)
}
