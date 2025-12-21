package entities

import (
	"encoding/json"
	"strings"
)

// ExecutionEnvironmentOverride represents execution environment overrides
type ExecutionEnvironmentOverride struct {
	WorkingDir string            `json:"working_dir,omitempty"`
	EnvVars    map[string]string `json:"env_vars,omitempty"`
	Secrets    []string          `json:"secrets,omitempty"` // Secret names to fetch server-side
	SkillDirs  []string          `json:"skill_dirs,omitempty"`
	Timeout    int               `json:"timeout,omitempty"`
}

// ExecuteAgentRequest represents a request to execute an agent
type ExecuteAgentRequest struct {
	Prompt               string                        `json:"prompt"`
	SystemPrompt         *string                       `json:"system_prompt,omitempty"`
	WorkerQueueID        *string                       `json:"worker_queue_id,omitempty"` // Optional: Worker queue UUID (auto-selected if not provided)
	ParentExecutionID    *string                       `json:"parent_execution_id,omitempty"` // Optional: Parent execution ID for conversation continuation
	Stream               *bool                         `json:"stream,omitempty"`
	UserMetadata         map[string]interface{}        `json:"user_metadata,omitempty"`
	ExecutionEnvironment *ExecutionEnvironmentOverride `json:"execution_environment,omitempty"`
}

// ExecuteTeamRequest represents a request to execute a team
type ExecuteTeamRequest struct {
	Prompt               string                        `json:"prompt"`
	SystemPrompt         *string                       `json:"system_prompt,omitempty"`
	WorkerQueueID        *string                       `json:"worker_queue_id,omitempty"` // Optional: Worker queue UUID (auto-selected if not provided)
	ParentExecutionID    *string                       `json:"parent_execution_id,omitempty"` // Optional: Parent execution ID for conversation continuation
	Stream               *bool                         `json:"stream,omitempty"`
	UserMetadata         map[string]interface{}        `json:"user_metadata,omitempty"`
	ExecutionEnvironment *ExecutionEnvironmentOverride `json:"execution_environment,omitempty"`
}

// AgentExecutionStatus represents the status of an agent execution
type AgentExecutionStatus string

const (
	ExecutionStatusPending         AgentExecutionStatus = "pending"
	ExecutionStatusRunning         AgentExecutionStatus = "running"
	ExecutionStatusWaitingForInput AgentExecutionStatus = "waiting_for_input"
	ExecutionStatusCompleted       AgentExecutionStatus = "completed"
	ExecutionStatusFailed          AgentExecutionStatus = "failed"
)

// UnmarshalJSON handles case-insensitive status unmarshaling
func (s *AgentExecutionStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	// Normalize to lowercase
	*s = AgentExecutionStatus(strings.ToLower(str))
	return nil
}

// AgentExecution represents an agent or team execution
type AgentExecution struct {
	ID                string                 `json:"id"`                    // Used by GET /executions/:id
	ExecutionID       string                 `json:"execution_id"`          // Used by POST /agents/:id/execute response
	WorkflowID        *string                `json:"workflow_id,omitempty"` // Returned in execute response
	Message           *string                `json:"message,omitempty"`     // Status message from execute response
	ExecutionType     string                 `json:"execution_type"`        // "agent" or "team"
	EntityID          string                 `json:"entity_id"`
	EntityName        *string                `json:"entity_name,omitempty"`
	Agent             *Agent                 `json:"agent,omitempty"`
	Team              *Team                  `json:"team,omitempty"`
	Prompt            string                 `json:"prompt"`
	Task              string                 `json:"task,omitempty"` // Alias for prompt
	SystemPrompt      *string                `json:"system_prompt,omitempty"`
	Status            AgentExecutionStatus   `json:"status"`
	Response          *string                `json:"response,omitempty"`  // The actual output from the execution
	ErrorMessage      *string                `json:"error_message,omitempty"`
	Error             *string                `json:"error,omitempty"`    // Alias for error_message
	Usage             map[string]interface{} `json:"usage,omitempty"`
	ExecutionMetadata map[string]interface{} `json:"execution_metadata,omitempty"`
	WorkerQueueID     *string                `json:"worker_queue_id,omitempty"`
	WorkerID          *string                `json:"worker_id,omitempty"`
	CreatedAt         *CustomTime            `json:"created_at,omitempty"`
	StartedAt         *CustomTime            `json:"started_at,omitempty"`
	CompletedAt       *CustomTime            `json:"completed_at,omitempty"`
	UpdatedAt         *CustomTime            `json:"updated_at,omitempty"`
}

// GetID returns the execution ID, checking both possible fields
func (e *AgentExecution) GetID() string {
	if e.ExecutionID != "" {
		return e.ExecutionID
	}
	return e.ID
}

// StreamEvent represents a streaming event from execution
type StreamEvent struct {
	Type      string                 `json:"type"`    // "chunk", "error", "complete", "status", "tool_started", "tool_completed", "message_chunk"
	Content   string                 `json:"content"` // The content chunk or error message
	Status    *AgentExecutionStatus  `json:"status,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp *CustomTime            `json:"timestamp,omitempty"`

	// Tool event fields (for tool_started, tool_completed events)
	ToolName    string                 `json:"tool_name,omitempty"`
	ToolInputs  map[string]interface{} `json:"tool_inputs,omitempty"`
	ToolOutputs map[string]interface{} `json:"tool_outputs,omitempty"`
	Duration    *float64               `json:"duration_seconds,omitempty"`
	Success     *bool                  `json:"success,omitempty"`

	// Message event fields (for message, message_chunk events)
	Role  string `json:"role,omitempty"`  // "assistant", "user", "system", "tool"
	Chunk *bool  `json:"chunk,omitempty"` // true for streaming chunks vs complete messages
}

// Stream event type constants
const (
	StreamEventTypeChunk         = "chunk"
	StreamEventTypeError         = "error"
	StreamEventTypeComplete      = "complete"
	StreamEventTypeStatus        = "status"
	StreamEventTypeToolStarted   = "tool_started"
	StreamEventTypeToolCompleted = "tool_completed"
	StreamEventTypeMessage       = "message"
	StreamEventTypeMessageChunk  = "message_chunk"
	StreamEventTypeConnected     = "connected"
	StreamEventTypeDone          = "done"
)

// IsToolEvent returns true if this is a tool-related event
func (e *StreamEvent) IsToolEvent() bool {
	return e.Type == StreamEventTypeToolStarted || e.Type == StreamEventTypeToolCompleted
}

// IsMessageEvent returns true if this is a message-related event
func (e *StreamEvent) IsMessageEvent() bool {
	return e.Type == StreamEventTypeMessage || e.Type == StreamEventTypeMessageChunk || e.Type == StreamEventTypeChunk
}

// IsTerminalEvent returns true if this event indicates the stream is ending
func (e *StreamEvent) IsTerminalEvent() bool {
	return e.Type == StreamEventTypeComplete || e.Type == StreamEventTypeDone || e.Type == StreamEventTypeError
}
