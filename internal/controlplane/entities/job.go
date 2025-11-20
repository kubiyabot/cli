package entities

import "time"

// Job represents a scheduled or recurring task
type Job struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`

	// Entity association - either agent_id or team_id
	AgentID     *string    `json:"agent_id,omitempty"`
	TeamID      *string    `json:"team_id,omitempty"`

	// Schedule configuration
	Schedule    *string    `json:"schedule,omitempty"`      // Cron expression
	Timezone    *string    `json:"timezone,omitempty"`       // Timezone for schedule

	// Execution configuration
	Prompt      string     `json:"prompt"`                   // Task to execute
	Enabled     bool       `json:"enabled"`                  // Whether job is active

	// Metadata
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	CreatedBy   *string    `json:"created_by,omitempty"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty"`

	// Webhook trigger
	WebhookPath *string    `json:"webhook_path,omitempty"`
}

// CreateJobRequest represents the request to create a new job
type CreateJobRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`

	// Entity association - either agent_id or team_id (required)
	AgentID     *string `json:"agent_id,omitempty"`
	TeamID      *string `json:"team_id,omitempty"`

	// Schedule configuration (optional - if not provided, job can only be triggered manually/webhook)
	Schedule    *string `json:"schedule,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`

	// Execution configuration
	Prompt      string  `json:"prompt"`
	Enabled     *bool   `json:"enabled,omitempty"` // Default to true if not specified

	// Webhook configuration (optional)
	WebhookPath *string `json:"webhook_path,omitempty"`
}

// UpdateJobRequest represents the request to update a job
type UpdateJobRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`

	// Schedule configuration
	Schedule    *string `json:"schedule,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`

	// Execution configuration
	Prompt      *string `json:"prompt,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`

	// Entity association update
	AgentID     *string `json:"agent_id,omitempty"`
	TeamID      *string `json:"team_id,omitempty"`

	// Webhook configuration
	WebhookPath *string `json:"webhook_path,omitempty"`
}

// TriggerJobResponse represents the response from triggering a job
type TriggerJobResponse struct {
	ExecutionID string `json:"execution_id"`
	JobID       string `json:"job_id"`
	Status      string `json:"status"`
}

// JobExecutionsResponse represents the list of executions for a job
type JobExecutionsResponse struct {
	Executions []*AgentExecution `json:"executions"`
	Total      int               `json:"total"`
}
