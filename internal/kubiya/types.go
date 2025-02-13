package kubiya

import (
	"fmt"
	"strings"
	"time"
)

// Agent represents a Kubiya teammate
type Agent struct {
	UUID           string            `json:"uuid"`
	Name           string            `json:"name"`
	Desc           string            `json:"description"`
	AIInstructions string            `json:"ai_instructions"`
	Environment    map[string]string `json:"environment"`
	Metadata       struct {
		CreatedAt   string `json:"created_at"`
		LastUpdated string `json:"last_updated"`
	} `json:"metadata"`
}

// Knowledge represents a knowledge base item
type Knowledge struct {
	UUID                 string            `json:"uuid,omitempty"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	Content              string            `json:"content"`
	ContentHash          string            `json:"content_hash,omitempty"`
	Labels               []string          `json:"labels"`
	Groups               []string          `json:"groups"`
	Properties           map[string]string `json:"properties"`
	Owner                string            `json:"owner"`
	Type                 string            `json:"type"`
	Source               string            `json:"source"`
	SupportedAgents      []string          `json:"supported_agents"`
	SupportedAgentGroups []string          `json:"supported_agents_groups"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
	ManagedBy            string            `json:"managed_by"`
	TaskID               string            `json:"task_id"`
}

// Source represents a tool source
type Source struct {
	UUID                    string         `json:"uuid"`
	URL                     string         `json:"url"`
	Name                    string         `json:"name"`
	Description             string         `json:"description"`
	TaskID                  string         `json:"task_id"`
	ManagedBy               string         `json:"managed_by"`
	ConnectedAgentsCount    int            `json:"connected_agents_count"`
	ConnectedToolsCount     int            `json:"connected_tools_count"`
	ConnectedWorkflowsCount int            `json:"connected_workflows_count"`
	KubiyaMetadata          KubiyaMetadata `json:"kubiya_metadata"`
	ErrorsCount             int            `json:"errors_count"`
	Tools                   []Tool         `json:"tools"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
}

// KubiyaMetadata represents metadata about source creation and updates
type KubiyaMetadata struct {
	CreatedAt       string `json:"created_at"`
	LastUpdated     string `json:"last_updated"`
	UserCreated     string `json:"user_created"`
	UserLastUpdated string `json:"user_last_updated"`
}

// ToolArg represents an argument for a tool
type ToolArg struct {
	Name        string       `yaml:"name" json:"name"`
	Type        string       `yaml:"type,omitempty" json:"type,omitempty"`
	Description string       `yaml:"description" json:"description"`
	Required    bool         `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string       `yaml:"default,omitempty" json:"default,omitempty"`
	Options     []string     `yaml:"options,omitempty" json:"options,omitempty"`
	OptionsFrom *OptionsFrom `yaml:"options_from,omitempty" json:"options_from,omitempty"`
}

type ToolSource struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// Tool represents a tool in a source
type Tool struct {
	Name        string     `json:"name" yaml:"name"`
	Source      ToolSource `json:"source"`
	Description string     `json:"description" yaml:"description"`
	Args        []ToolArg  `json:"args" yaml:"args,omitempty"`
	Env         []string   `json:"env" yaml:"env,omitempty"`
	Content     string     `json:"content" yaml:"content,omitempty"`
	FileName    string     `json:"file_name" yaml:"file_name,omitempty"`
	Secrets     []string   `json:"secrets,omitempty"`
	IconURL     string     `json:"icon_url,omitempty"`
	Type        string     `json:"type,omitempty"`
	Alias       string     `json:"alias,omitempty"`
	WithFiles   []string   `json:"with_files,omitempty"`
	WithVolumes []string   `json:"with_volumes,omitempty"`
	LongRunning bool       `json:"long_running,omitempty"`
	Metadata    []string   `json:"metadata,omitempty"`
	Mermaid     string     `json:"mermaid,omitempty"`
}

// ToolMetadata represents metadata for a tool
type ToolMetadata struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	LastUsed    time.Time `json:"last_used"`
	UsageCount  int       `json:"usage_count"`
	ErrorCount  int       `json:"error_count"`
	SuccessRate float64   `json:"success_rate"`
}

// SourceMetadata represents detailed source information
type SourceMetadata struct {
	UUID                 string                 `json:"uuid"`
	URL                  string                 `json:"url"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	Type                 string                 `json:"type"`
	Config               map[string]interface{} `json:"config"`
	Status               string                 `json:"status"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	Tools                []Tool                 `json:"tools"`
	Stats                SourceStats            `json:"stats"`
	ManagedBy            string                 `json:"managed_by"`
	Owner                string                 `json:"owner"`
	Properties           map[string]string      `json:"properties"`
	Labels               []string               `json:"labels"`
	Groups               []string               `json:"groups"`
	ConnectedAgentsCount int                    `json:"connected_agents_count"`
	ConnectedToolsCount  int                    `json:"connected_tools_count"`
}

// SourceStats represents statistics for a source
type SourceStats struct {
	TotalTools          int       `json:"total_tools"`
	ActiveTools         int       `json:"active_tools"`
	ErrorCount          int       `json:"error_count"`
	SuccessRate         float64   `json:"success_rate"`
	AverageResponseTime float64   `json:"average_response_time"`
	LastActivity        time.Time `json:"last_activity"`
}

// ChatResponse represents a response from the chat API
type ChatResponse struct {
	Content string `json:"content"`
}

// ChatMessage represents a message in a chat session
type ChatMessage struct {
	Content    string `json:"content"`
	Type       string `json:"type"`
	MessageID  string `json:"message_id"`
	Timestamp  string `json:"timestamp"`
	SenderName string `json:"sender_name"`
	Final      bool   `json:"final"`
	SessionID  string `json:"session_id"`
	Error      string `json:"error"`
}

// Add these new types to support Tool
type ToolParameter struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Required     bool     `json:"required"`
	Default      string   `json:"default,omitempty"`
	Enum         []string `json:"enum,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`
	MinLength    *int     `json:"min_length,omitempty"`
	MaxLength    *int     `json:"max_length,omitempty"`
	MinValue     *float64 `json:"min_value,omitempty"`
	MaxValue     *float64 `json:"max_value,omitempty"`
	MultipleOf   *float64 `json:"multiple_of,omitempty"`
	Format       string   `json:"format,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Examples     []string `json:"examples,omitempty"`
}

type ToolOutput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Runner represents a Kubiya runner
type Runner struct {
	Name                string       `json:"name"`
	WssURL              string       `json:"wss_url"`
	TaskID              string       `json:"task_id"`
	ManagedBy           string       `json:"managed_by"`
	Description         string       `json:"description"`
	AuthenticationType  string       `json:"authentication_type"`
	Version             string       `json:"version"`
	RunnerType          string       `json:"runner_type"`
	GatewayURL          *string      `json:"gateway_url"`
	GatewayPassword     *string      `json:"gateway_password"`
	Namespace           string       `json:"namespace"`
	Subject             string       `json:"subject"`
	UserKeyID           string       `json:"user_key_id"`
	RunnerHealth        HealthStatus `json:"runner_health"`
	ToolManagerHealth   HealthStatus `json:"tool_manager_health"`
	AgentManagerHealth  HealthStatus `json:"agent_manager_health"`
	KubernetesNamespace string       `json:"kubernetes_namespace"`
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status  string `json:"status"`
	Health  string `json:"health"`
	Error   string `json:"error"`
	Version string `json:"version"`
}

// RunnerManifest represents the response for runner manifest request
type RunnerManifest struct {
	URL string `json:"url"`
}

// Webhook represents a Kubiya webhook
type Webhook struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Source        string        `json:"source"`
	AgentID       string        `json:"agent_id"`
	Communication Communication `json:"communication"`
	CreatedAt     string        `json:"created_at"`
	CreatedBy     string        `json:"created_by"`
	Filter        string        `json:"filter"`
	ManagedBy     string        `json:"managed_by"`
	Org           string        `json:"org"`
	Prompt        string        `json:"prompt"`
	TaskID        string        `json:"task_id"`
	UpdatedAt     string        `json:"updated_at"`
	WebhookURL    string        `json:"webhook_url"`
}

// Communication represents webhook communication settings
type Communication struct {
	Method      string `json:"method"`
	Destination string `json:"destination"`
}

// Add this to types.go
type Secret struct {
	Name        string `json:"name"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description"`
}

type SecretValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Add these types
type EnvVarSource struct {
	Type     string // "teammate", "local", "aws", "manual"
	Value    string
	Icon     string
	Label    string
	Teammate *Teammate // if from teammate
}

// EnvVarStatus represents the status of an environment variable
type EnvVarStatus struct {
	Value string
	// ... other fields if needed
}

// Add these types to your existing types.go file

type Teammate struct {
	UUID            string            `json:"uuid"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	InstructionType string            `json:"instruction_type"`
	LLMModel        string            `json:"llm_model"`
	Sources         []string          `json:"sources"`
	Environment     map[string]string `json:"environment_variables"`
	Secrets         []string          `json:"secrets"`
	AllowedGroups   []string          `json:"allowed_groups"`
	AllowedUsers    []string          `json:"allowed_users"`
	Owners          []string          `json:"owners"`
	Runners         []string          `json:"runners"`
	IsDebugMode     bool              `json:"is_debug_mode"`
	AIInstructions  string            `json:"ai_instructions"`
	Image           string            `json:"image"`
	ManagedBy       string            `json:"managed_by"`
	Integrations    []string          `json:"integrations"`
	Links           []string          `json:"links"`
	Tools           []string          `json:"tools"`
	Tasks           []string          `json:"tasks"`
	Starters        []string          `json:"starters"`
	Metadata        struct {
		CreatedAt   string `json:"created_at"`
		LastUpdated string `json:"last_updated"`
	} `json:"metadata"`
}

// SourceError represents an error in source discovery
type SourceError struct {
	File    string `json:"file"`
	Type    string `json:"type"`
	Error   string `json:"error"`
	Details string `json:"details"`
}

// SourceDiscoveryResponse represents the response from source discovery
type SourceDiscoveryResponse struct {
	Name   string `json:"name"`
	Source struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		Commit    string `json:"commit"`
		Committer string `json:"committer"`
		Branch    string `json:"branch"`
	} `json:"source"`
	Tools    []Tool        `json:"tools"`
	Errors   []SourceError `json:"errors"`
	errorMsg string        // private field to store error message
}

// Error implements the error interface
func (s *SourceDiscoveryResponse) Error() string {
	if s.errorMsg != "" {
		return s.errorMsg
	}
	if len(s.Errors) > 0 {
		var errMsgs []string
		for _, e := range s.Errors {
			msg := fmt.Sprintf("%s in %s: %s", e.Type, e.File, e.Error)
			if e.Details != "" {
				msg += "\nDetails: " + e.Details
			}
			errMsgs = append(errMsgs, msg)
		}
		return fmt.Sprintf("source discovery found errors:\n%s", strings.Join(errMsgs, "\n"))
	}
	return "unknown error in source discovery"
}

// SetError sets the error message
func (s *SourceDiscoveryResponse) SetError(msg string) {
	s.errorMsg = msg
}

// SyncOptions represents options for syncing a source
type SyncOptions struct {
	Mode       string `json:"mode,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Force      bool   `json:"force,omitempty"`
	AutoCommit bool   `json:"auto_commit,omitempty"`
	NoDiff     bool   `json:"no_diff,omitempty"`
}

// Arg represents a tool argument
type Arg struct {
	Name        string       `yaml:"name" json:"name"`
	Type        string       `yaml:"type,omitempty" json:"type,omitempty"`
	Description string       `yaml:"description" json:"description"`
	Required    bool         `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string       `yaml:"default,omitempty" json:"default,omitempty"`
	Options     []string     `yaml:"options,omitempty" json:"options,omitempty"`
	OptionsFrom *OptionsFrom `yaml:"options_from,omitempty" json:"options_from,omitempty"`
}

// OptionsFrom defines where to get argument options from
type OptionsFrom struct {
	Image  string `yaml:"image" json:"image"`
	Script string `yaml:"script" json:"script"`
}

// GeneratedToolContent represents the structure of tool generation SSE events
type GeneratedToolContent struct {
	Content  string `json:"content"`
	FileName string `json:"file_name"`
}

// GeneratedToolResponse represents the wrapper response for tool generation
type GeneratedToolResponse struct {
	GeneratedToolContent []GeneratedToolContent `json:"generated_tool_content"`
}

type ToolGenerationChatMessage struct {
	Type                 string                 `json:"type"`
	GeneratedToolContent []GeneratedToolContent `json:"generated_tool_content"`
}
