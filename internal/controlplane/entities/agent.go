package entities

// AgentStatus represents the status of an agent
type AgentStatus string

const (
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusPaused    AgentStatus = "paused"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusFailed    AgentStatus = "failed"
	AgentStatusStopped   AgentStatus = "stopped"
)

// RuntimeType represents the agent runtime type
type RuntimeType string

const (
	RuntimeDefault    RuntimeType = "default"
	RuntimeClaudeCode RuntimeType = "claude_code"
)

// Agent represents an agent in the control plane
type Agent struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	Status        AgentStatus            `json:"status,omitempty"`
	Capabilities  []string               `json:"capabilities,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	ModelID       *string                `json:"model_id,omitempty"`
	Model         *string                `json:"model,omitempty"` // Model value (alternative to model_id)
	LLMConfig     map[string]interface{} `json:"llm_config,omitempty"`
	Runtime       RuntimeType            `json:"runtime,omitempty"`
	SystemPrompt  *string                `json:"system_prompt,omitempty"`
	TeamID        *string                `json:"team_id,omitempty"`
	EnvironmentIDs []string              `json:"environment_ids,omitempty"`
	ExecutionEnvironment *ExecutionEnvironment `json:"execution_environment,omitempty"`
	CreatedAt     *CustomTime            `json:"created_at,omitempty"`
	UpdatedAt     *CustomTime            `json:"updated_at,omitempty"`
	LastActiveAt  *CustomTime            `json:"last_active_at,omitempty"`
	State         map[string]interface{} `json:"state,omitempty"`
	ErrorMessage  *string                `json:"error_message,omitempty"`
}

// ExecutionEnvironment represents the execution environment for an agent
type ExecutionEnvironment struct {
	EnvVars      map[string]string `json:"env_vars,omitempty"`
	Secrets      []string          `json:"secrets,omitempty"`
	Integrations []string          `json:"integrations,omitempty"`
}

// AgentCreateRequest represents the request to create an agent
type AgentCreateRequest struct {
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	Capabilities  []string               `json:"capabilities,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	ModelID       *string                `json:"model_id,omitempty"`
	Model         *string                `json:"model,omitempty"`
	LLMConfig     map[string]interface{} `json:"llm_config,omitempty"`
	Runtime       *RuntimeType           `json:"runtime,omitempty"`
	SystemPrompt  *string                `json:"system_prompt,omitempty"`
	TeamID        *string                `json:"team_id,omitempty"`
	EnvironmentIDs []string              `json:"environment_ids,omitempty"`
	ExecutionEnvironment *ExecutionEnvironment `json:"execution_environment,omitempty"`
}

// AgentUpdateRequest represents the request to update an agent
type AgentUpdateRequest struct {
	Name          *string                `json:"name,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Status        *AgentStatus           `json:"status,omitempty"`
	Capabilities  []string               `json:"capabilities,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	State         map[string]interface{} `json:"state,omitempty"`
	ModelID       *string                `json:"model_id,omitempty"`
	Model         *string                `json:"model,omitempty"`
	LLMConfig     map[string]interface{} `json:"llm_config,omitempty"`
	Runtime       *RuntimeType           `json:"runtime,omitempty"`
	SystemPrompt  *string                `json:"system_prompt,omitempty"`
	TeamID        *string                `json:"team_id,omitempty"`
	EnvironmentIDs []string              `json:"environment_ids,omitempty"`
	ExecutionEnvironment *ExecutionEnvironment `json:"execution_environment,omitempty"`
}

// AgentExecuteRequest represents the request to execute an agent
type AgentExecuteRequest struct {
	Input      string                 `json:"input"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// AgentExecuteResponse represents the response from executing an agent
type AgentExecuteResponse struct {
	ExecutionID string `json:"execution_id"`
	Status      string `json:"status"`
}
