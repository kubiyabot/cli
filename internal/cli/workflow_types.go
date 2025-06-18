package cli

import "time"

// WorkflowExecutionRequest represents a request to execute a workflow
type WorkflowExecutionRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Steps       []interface{}          `json:"steps"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Runner      string                 `json:"runner,omitempty"`
}

// WorkflowExecutionResponse represents the response from workflow execution
type WorkflowExecutionResponse struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// WorkflowStreamEvent represents a streaming event during workflow execution
type WorkflowStreamEvent struct {
	Type      string                 `json:"type"`
	Step      string                 `json:"step,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Output    string                 `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Progress  int                    `json:"progress,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
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
}

// MCPServerConfig represents MCP server configuration
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// WorkflowComposition represents multiple workflows to be composed
type WorkflowComposition struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description" json:"description"`
	Workflows   []Workflow             `yaml:"workflows" json:"workflows"`
	Variables   map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`
}
