package cli

import (
	"time"
)

// TriggerProvider represents different external trigger providers
type TriggerProvider string

const (
	ProviderDatadog TriggerProvider = "datadog"
	ProviderGitHub  TriggerProvider = "github"
)

// Trigger represents a workflow trigger configuration
type Trigger struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Provider    TriggerProvider        `json:"provider"`
	WorkflowRef string                 `json:"workflow_ref"` // Path to workflow file or inline workflow
	Config      map[string]interface{} `json:"config"`       // Provider-specific configuration
	Status      TriggerStatus          `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedBy   string                 `json:"created_by"`
	WebhookURL  string                 `json:"webhook_url,omitempty"` // Generated webhook URL
}

// TriggerStatus represents the status of a trigger
type TriggerStatus string

const (
	StatusActive   TriggerStatus = "active"
	StatusInactive TriggerStatus = "inactive"
	StatusError    TriggerStatus = "error"
)

// DatadogTriggerConfig holds Datadog-specific configuration
type DatadogTriggerConfig struct {
	WebhookName   string            `json:"webhook_name"`
	CustomHeaders string            `json:"custom_headers,omitempty"`
	Payload       string            `json:"payload,omitempty"`
	EncodeAs      string            `json:"encode_as,omitempty"`
	APIEndpoint   string            `json:"api_endpoint,omitempty"` // Allow custom DD endpoints
	Environment   map[string]string `json:"environment,omitempty"`  // Environment variables for workflow
}

// GitHubTriggerConfig holds GitHub-specific configuration
type GitHubTriggerConfig struct {
	Repository string   `json:"repository"`    // Format: "owner/repo"
	Events     []string `json:"events"`       // GitHub webhook events
	Secret     string   `json:"secret,omitempty"` // Webhook secret for verification
}

// TriggerProvider interface defines the methods each provider must implement
type TriggerProviderInterface interface {
	// CreateTrigger creates a new trigger with the provider
	CreateTrigger(trigger *Trigger) error

	// UpdateTrigger updates an existing trigger
	UpdateTrigger(trigger *Trigger) error

	// DeleteTrigger removes a trigger from the provider
	DeleteTrigger(triggerID string) error

	// TestTrigger sends a test event to verify the trigger works
	TestTrigger(triggerID string) error

	// ValidateConfig validates the provider-specific configuration
	ValidateConfig(config map[string]interface{}) error

	// GetRequiredEnvVars returns a list of required environment variables
	GetRequiredEnvVars() []string
}

// TriggerTestResult represents the result of testing a trigger
type TriggerTestResult struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// WebhookInfo represents information about a webhook from a provider
type WebhookInfo struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Provider string `json:"provider"`
	IsKubiya bool   `json:"is_kubiya"`
	Events   []string `json:"events,omitempty"`
}
