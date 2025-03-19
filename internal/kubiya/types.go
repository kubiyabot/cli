package kubiya

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Integration struct {
	Name    string `json:"name"`
	Type    string `json:"integration_type"`
	Configs []struct {
		Name      string         `json:"name"`
		IsDefault bool           `json:"is_default"`
		Details   map[string]any `json:"vendor_specific"`
	} `json:"configs,omitempty"`
}

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
	UUID                    string                 `json:"uuid"`
	URL                     string                 `json:"url"`
	Name                    string                 `json:"name"`
	Description             string                 `json:"description"`
	Type                    string                 `json:"type,omitempty"` // "git", "inline", or empty for backwards compatibility
	TaskID                  string                 `json:"task_id"`
	ManagedBy               string                 `json:"managed_by"`
	ConnectedAgentsCount    int                    `json:"connected_agents_count"`
	ConnectedToolsCount     int                    `json:"connected_tools_count"`
	ConnectedWorkflowsCount int                    `json:"connected_workflows_count"`
	KubiyaMetadata          KubiyaMetadata         `json:"kubiya_metadata"`
	ErrorsCount             int                    `json:"errors_count"`
	Tools                   []Tool                 `json:"tools"`
	InlineTools             []Tool                 `json:"inline_tools,omitempty"`   // For inline sources
	DynamicConfig           map[string]interface{} `json:"dynamic_config,omitempty"` // Dynamic configuration
	Runner                  string                 `json:"runner,omitempty"`         // Runner name
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
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
	Name        string      `json:"name" yaml:"name"`
	Source      ToolSource  `json:"source"`
	Description string      `json:"description" yaml:"description"`
	Args        []ToolArg   `json:"args" yaml:"args,omitempty"`
	Env         []string    `json:"env" yaml:"env,omitempty"`
	Content     string      `json:"content" yaml:"content,omitempty"`
	FileName    string      `json:"file_name" yaml:"file_name,omitempty"`
	Secrets     []string    `json:"secrets,omitempty"`
	IconURL     string      `json:"icon_url,omitempty"`
	Type        string      `json:"type,omitempty"`
	Alias       string      `json:"alias,omitempty"`
	WithFiles   interface{} `json:"with_files,omitempty"`   // Can be []string or map[string]interface{}
	WithVolumes interface{} `json:"with_volumes,omitempty"` // Can be []string or map[string]interface{}
	LongRunning bool        `json:"long_running,omitempty"`
	Metadata    interface{} `json:"metadata,omitempty"` // Can be []string or other formats
	Mermaid     string      `json:"mermaid,omitempty"`
}

// GetToolFiles returns a list of files associated with this tool,
// handling both string array and object formats safely
func (t *Tool) GetToolFiles() []string {
	files := []string{}

	if t.WithFiles == nil {
		return files
	}

	// Try to convert different formats to string slice
	switch v := t.WithFiles.(type) {
	case []string:
		// Already a string slice
		return v
	case []interface{}:
		// Convert interface slice to string slice
		for _, item := range v {
			if str, ok := item.(string); ok {
				files = append(files, str)
			}
		}
	case map[string]interface{}:
		// Extract keys and values from map
		for key, value := range v {
			files = append(files, key)
			if str, ok := value.(string); ok {
				files = append(files, str)
			}
		}
	}

	return files
}

// GetVolumes returns a list of volumes associated with this tool,
// handling both string array and object formats safely
func (t *Tool) GetVolumes() []string {
	volumes := []string{}

	if t.WithVolumes == nil {
		return volumes
	}

	// Try to convert different formats to string slice
	switch v := t.WithVolumes.(type) {
	case []string:
		// Already a string slice
		return v
	case []interface{}:
		// Convert interface slice to string slice
		for _, item := range v {
			if str, ok := item.(string); ok {
				volumes = append(volumes, str)
			}
		}
	case map[string]interface{}:
		// Extract keys and values from map
		for key, value := range v {
			volumes = append(volumes, key)
			if str, ok := value.(string); ok {
				volumes = append(volumes, str)
			}
		}
	}

	return volumes
}

// GetMetadata returns metadata associated with this tool as strings,
// handling both string array and object formats safely
func (t *Tool) GetMetadata() []string {
	metadata := []string{}

	if t.Metadata == nil {
		return metadata
	}

	// Try to convert different formats to string slice
	switch v := t.Metadata.(type) {
	case []string:
		// Already a string slice
		return v
	case []interface{}:
		// Convert interface slice to string slice
		for _, item := range v {
			if str, ok := item.(string); ok {
				metadata = append(metadata, str)
			} else {
				// Try to marshal non-string items as JSON
				if jsonData, err := json.Marshal(item); err == nil {
					metadata = append(metadata, string(jsonData))
				}
			}
		}
	case map[string]interface{}:
		// Extract keys and values from map
		for key, value := range v {
			metadata = append(metadata, key)
			if str, ok := value.(string); ok {
				metadata = append(metadata, str)
			} else {
				// Try to marshal non-string items as JSON
				if jsonData, err := json.Marshal(value); err == nil {
					metadata = append(metadata, string(jsonData))
				}
			}
		}
	default:
		// Try to marshal any other type as JSON
		if jsonData, err := json.Marshal(v); err == nil {
			metadata = append(metadata, string(jsonData))
		}
	}

	return metadata
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
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	Source             string        `json:"source"`
	AgentID            string        `json:"agent_id"`
	Communication      Communication `json:"communication"`
	CreatedAt          string        `json:"created_at,omitempty"`
	CreatedBy          string        `json:"created_by,omitempty"`
	Filter             string        `json:"filter"`
	ManagedBy          string        `json:"managed_by,omitempty"`
	Org                string        `json:"org,omitempty"`
	Prompt             string        `json:"prompt"`
	TaskID             string        `json:"task_id,omitempty"`
	UpdatedAt          string        `json:"updated_at,omitempty"`
	WebhookURL         string        `json:"webhook_url,omitempty"`
	HideWebhookHeaders bool          `json:"hide_webhook_headers,omitempty"`
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

// ProjectTemplate represents a template for creating projects
type ProjectTemplate struct {
	UUID          string             `json:"uuid,omitempty"`
	ID            string             `json:"id,omitempty"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	Version       string             `json:"version,omitempty"`
	RepositoryURL string             `json:"repository_url,omitempty"`
	URL           string             `json:"url,omitempty"`
	Repository    string             `json:"repository,omitempty"`
	Readme        string             `json:"readme,omitempty"`
	Icons         []interface{}      `json:"icons,omitempty"`
	Variables     []TemplateVariable `json:"variables,omitempty"`
	Secrets       []TemplateSecret   `json:"secrets,omitempty"`
	Providers     []TemplateProvider `json:"providers,omitempty"`
	Resources     []TemplateResource `json:"resources,omitempty"`
}

// TemplateProvider represents a provider used by a template
type TemplateProvider struct {
	Name string `json:"name"`
}

// TemplateResource represents a resource in a template
type TemplateResource struct {
	Name      string                `json:"name"`
	Type      string                `json:"type"`
	Provider  string                `json:"provider"`
	Variables []TemplateRscVariable `json:"variables,omitempty"`
}

// TemplateRscVariable represents a variable for a resource in a template
type TemplateRscVariable struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Value        interface{} `json:"value,omitempty"`
	Default      interface{} `json:"default,omitempty"`
	HasError     bool        `json:"has_error,omitempty"`
	Description  string      `json:"description,omitempty"`
	ErrorDetail  string      `json:"error_detail,omitempty"`
	ErrorSummary string      `json:"error_summary,omitempty"`
}

// TemplateSecret represents a secret required by a template
type TemplateSecret struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ToEnv       string `json:"to_env,omitempty"`
	Value       string `json:"value,omitempty"`
}

// TemplateVariable defines a variable expected by a template
type TemplateVariable struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Description  string      `json:"description,omitempty"`
	Default      interface{} `json:"default,omitempty"`
	Value        interface{} `json:"value,omitempty"`
	Required     bool        `json:"required,omitempty"`
	Sensitive    bool        `json:"sensitive,omitempty"`
	HasError     bool        `json:"has_error,omitempty"`
	ErrorDetail  string      `json:"error_detail,omitempty"`
	ErrorSummary string      `json:"error_summary,omitempty"`
}

// Variable represents a variable value in a project
type Variable struct {
	Name      string      `json:"name"`
	Value     interface{} `json:"value"`
	Type      string      `json:"type,omitempty"`
	Sensitive bool        `json:"sensitive,omitempty"`
}

// Project represents a project instance
type Project struct {
	ID          string        `json:"id,omitempty"`
	UUID        string        `json:"uuid,omitempty"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Status      string        `json:"status,omitempty"`
	UsecaseID   string        `json:"usecase_id,omitempty"`
	CreatedAt   string        `json:"created_at,omitempty"`
	UpdatedAt   string        `json:"updated_at,omitempty"`
	Variables   []Variable    `json:"variables,omitempty"`
	URL         string        `json:"url,omitempty"`
	Repository  string        `json:"repository,omitempty"`
	Readme      string        `json:"readme,omitempty"`
	Icons       []interface{} `json:"icons,omitempty"`
}

// ProjectResource represents a resource in a project
type ProjectResource struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	Properties  map[string]interface{} `json:"properties"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	Provisioner string                 `json:"provisioner,omitempty"`
}

// ProjectPlan represents a plan for project changes
type ProjectPlan struct {
	ProjectID  string                  `json:"project_id"`
	PlanID     string                  `json:"plan_id"`
	Status     string                  `json:"status"`
	Changes    []ProjectResourceChange `json:"changes"`
	CreatedAt  time.Time               `json:"created_at"`
	ApprovedAt *time.Time              `json:"approved_at,omitempty"`
}

// ProjectResourceChange represents a change to a resource in a project plan
type ProjectResourceChange struct {
	ResourceID string `json:"resource_id"`
	Action     string `json:"action"` // create, update, delete
	Before     string `json:"before,omitempty"`
	After      string `json:"after,omitempty"`
}

// ProjectExecution represents the execution of a project plan
type ProjectExecution struct {
	ProjectID   string     `json:"project_id"`
	PlanID      string     `json:"plan_id"`
	ExecutionID string     `json:"execution_id"`
	Status      string     `json:"status"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
}
