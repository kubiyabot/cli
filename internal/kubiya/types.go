package kubiya

import "time"

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

// Tool represents a tool within a source
type Tool struct {
	UUID        string      `json:"uuid"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Content     string      `json:"content"`
	Image       string      `json:"image"`
	Env         []string    `json:"env"`
	Alias       string      `json:"alias"`
	Args        []ToolArg   `json:"args"`
	LongRunning bool        `json:"long_running"`
	Source      ToolSource  `json:"source"`
	WithFiles   interface{} `json:"with_files"`
	WithVolumes interface{} `json:"with_volumes"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
	Workflows   interface{} `json:"workflows"`
}

// ToolArg represents an argument for a tool
type ToolArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolSource represents the source information for a tool
type ToolSource struct {
	ID string `json:"id"`
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
