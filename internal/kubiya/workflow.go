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
	"os"
	"strings"
	"time"
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
	Command     string                 `json:"command"` // Required: should be "execute_workflow"
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Steps       []interface{}          `json:"steps"`
	Params      map[string]interface{} `json:"params,omitempty"`  // Use 'params' instead of 'variables'
	Secrets     map[string]interface{} `json:"secrets,omitempty"` // Secrets passed in request body
	Env         map[string]interface{} `json:"env,omitempty"`     // Environment variables
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
	// Use orchestrator API endpoint - check for custom orchestrator URL
	orchestratorURL := os.Getenv("KUBIYA_ORCHESTRATOR_URL")
	if orchestratorURL == "" {
		// Check if we should use the same base URL as the main API
		if os.Getenv("KUBIYA_USE_SAME_API") == "true" {
			orchestratorURL = strings.TrimSuffix(wc.client.baseURL, "/api/v1") + "/api/orchestrate"
		} else {
			// Default to the orchestrator service URL
			orchestratorURL = "https://orchestrator.kubiya.ai/api/orchestrate"
		}
	}

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
	req.Header.Set("x-vercel-ai-data-stream", "v1") // protocol flag
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Always use UserKey format for API key authentication
	// The orchestrator expects UserKey format for Kubiya API keys (even if they are JWTs)
	req.Header.Set("Authorization", "UserKey "+options.BearerToken)

	// Execute request
	// Create a custom client with longer timeout for orchestration
	orchestrationClient := &http.Client{
		Timeout: 5 * time.Minute, // Longer timeout for orchestration
	}
	resp, err := orchestrationClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, fmt.Errorf("orchestration API timeout. The service might be unavailable or slow. You can:\n1. Try again later\n2. Use 'kubiya workflow execute' for direct workflow execution\n3. The generate command will create a local template instead")
		}
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, fmt.Errorf("orchestration API not found at %s. You may need to:\n1. Set KUBIYA_ORCHESTRATOR_URL environment variable\n2. Enable orchestration features in your Kubiya account\n3. Use 'kubiya workflow execute' for direct workflow execution instead", orchestratorURL)
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
		// Increase scanner buffer size for large events
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			// Check if context was cancelled
			select {
			case <-ctx.Done():
				select {
				case events <- WorkflowSSEEvent{Type: "error", Data: fmt.Sprintf("context canceled: %v", ctx.Err())}:
				case <-time.After(time.Second):
					// Don't block if channel is full during cancellation
				}
				return
			default:
			}

			line := scanner.Text()

			// Debug logging
			if wc.client.debug {
				fmt.Printf("[DEBUG] SSE Line: %s\n", line)
			}

			// Skip empty lines, retry messages, and heartbeat events
			if line == "" || strings.HasPrefix(line, "retry:") || line == ": heartbeat" || strings.HasPrefix(line, ": ") {
				continue
			}

			// Parse SSE format - the actual format is "2:{json}" or "d:{json}"
			if len(line) > 2 && line[1] == ':' {
				eventType := string(line[0])
				data := line[2:]

				switch eventType {
				case "2": // Data event
					select {
					case events <- WorkflowSSEEvent{Type: "data", Data: data}:
					case <-ctx.Done():
						return
					}
				case "3": // Error event with details
					select {
					case events <- WorkflowSSEEvent{Type: "data", Data: data}: // Process as data to get the error details
					case <-ctx.Done():
						return
					}
				case "d": // Done event
					select {
					case events <- WorkflowSSEEvent{Type: "done", Data: data}:
					case <-ctx.Done():
						return
					}
				case "e": // Error event
					select {
					case events <- WorkflowSSEEvent{Type: "error", Data: data}:
					case <-ctx.Done():
						return
					}
				default:
					// Unknown event type, treat as data
					select {
					case events <- WorkflowSSEEvent{Type: "data", Data: data}:
					case <-ctx.Done():
						return
					}
				}
			} else if strings.HasPrefix(line, "data: ") {
				// Standard SSE format
				data := strings.TrimPrefix(line, "data: ")
				select {
				case events <- WorkflowSSEEvent{Type: "data", Data: data}:
				case <-ctx.Done():
					return
				}
			} else if strings.HasPrefix(line, "event: ") {
				// Standard SSE event type
				eventType := strings.TrimPrefix(line, "event: ")
				select {
				case events <- WorkflowSSEEvent{Type: eventType, Data: ""}:
				case <-ctx.Done():
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			if wc.client.debug {
				fmt.Printf("[DEBUG] Scanner error: %v\n", err)
			}
			select {
			case events <- WorkflowSSEEvent{Type: "error", Data: fmt.Sprintf("stream reading error: %v", err)}:
			case <-ctx.Done():
				return
			}
		} else {
			// Scanner finished without error - send done event
			if wc.client.debug {
				fmt.Printf("[DEBUG] Stream ended normally\n")
			}
			select {
			case events <- WorkflowSSEEvent{Type: "done", Data: "stream completed"}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}

// ExecuteWorkflow executes a workflow directly using the Kubiya API (without orchestration)
func (wc *WorkflowClient) ExecuteWorkflow(ctx context.Context, req WorkflowExecutionRequest, runner string) (<-chan WorkflowSSEEvent, error) {
	// Default runner if not specified
	if runner == "" {
		if defaultRunner := os.Getenv("KUBIYA_DEFAULT_RUNNER"); defaultRunner != "" {
			runner = defaultRunner
		} else {
			runner = "kubiya-hosted" // Default to kubiya-hosted as mentioned
		}
	}

	// Build URL with query parameters
	params := url.Values{}
	params.Set("runner", runner)
	params.Set("operation", "execute_workflow")

	// Handle base URL to avoid double /api/v1
	baseURL := strings.TrimSuffix(wc.client.baseURL, "/")
	if strings.HasSuffix(baseURL, "/api/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/api/v1")
	}
	executeURL := fmt.Sprintf("%s/api/v1/workflow?%s", baseURL, params.Encode())

	// Create request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request with user context for proper cancellation handling
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, executeURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - use UserKey format like other CLI commands
	httpReq.Header.Set("Authorization", "UserKey "+wc.client.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-vercel-ai-data-stream", "v1") // protocol flag
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	// Execute request with no timeout for streaming connections
	streamingClient := &http.Client{
		Timeout: 0, // No timeout for streaming connections
	}
	resp, err := streamingClient.Do(httpReq)
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
		// Increase scanner buffer size for large events
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			// Check if context was cancelled
			select {
			case <-ctx.Done():
				select {
				case events <- WorkflowSSEEvent{Type: "error", Data: fmt.Sprintf("context canceled: %v", ctx.Err())}:
				case <-time.After(time.Second):
					// Don't block if channel is full during cancellation
				}
				return
			default:
			}

			line := scanner.Text()

			// Debug logging
			if wc.client.debug {
				fmt.Printf("[DEBUG] SSE Line: %s\n", line)
			}

			// Skip empty lines, retry messages, and heartbeat events
			if line == "" || strings.HasPrefix(line, "retry:") || line == ": heartbeat" || strings.HasPrefix(line, ": ") {
				continue
			}

			// Parse SSE format - the actual format is "2:{json}" or "d:{json}"
			if len(line) > 2 && line[1] == ':' {
				eventType := string(line[0])
				data := line[2:]

				switch eventType {
				case "2": // Data event
					select {
					case events <- WorkflowSSEEvent{Type: "data", Data: data}:
					case <-ctx.Done():
						return
					}
				case "3": // Error event with details
					select {
					case events <- WorkflowSSEEvent{Type: "data", Data: data}: // Process as data to get the error details
					case <-ctx.Done():
						return
					}
				case "d": // Done event
					select {
					case events <- WorkflowSSEEvent{Type: "done", Data: data}:
					case <-ctx.Done():
						return
					}
				case "e": // Error event
					select {
					case events <- WorkflowSSEEvent{Type: "error", Data: data}:
					case <-ctx.Done():
						return
					}
				default:
					// Unknown event type, treat as data
					select {
					case events <- WorkflowSSEEvent{Type: "data", Data: data}:
					case <-ctx.Done():
						return
					}
				}
			} else if strings.HasPrefix(line, "data: ") {
				// Standard SSE format
				data := strings.TrimPrefix(line, "data: ")
				select {
				case events <- WorkflowSSEEvent{Type: "data", Data: data}:
				case <-ctx.Done():
					return
				}
			} else if strings.HasPrefix(line, "event: ") {
				// Standard SSE event type
				eventType := strings.TrimPrefix(line, "event: ")
				select {
				case events <- WorkflowSSEEvent{Type: eventType, Data: ""}:
				case <-ctx.Done():
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			if wc.client.debug {
				fmt.Printf("[DEBUG] Scanner error: %v\n", err)
			}
			select {
			case events <- WorkflowSSEEvent{Type: "error", Data: fmt.Sprintf("stream reading error: %v", err)}:
			case <-ctx.Done():
				return
			}
		} else {
			// Scanner finished without error - send done event
			if wc.client.debug {
				fmt.Printf("[DEBUG] Stream ended normally\n")
			}
			select {
			case events <- WorkflowSSEEvent{Type: "done", Data: "stream completed"}:
			case <-ctx.Done():
				return
			}
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
