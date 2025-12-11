package entities

// WorkerQueue represents a worker queue in the control plane
type WorkerQueue struct {
	ID            string      `json:"id,omitempty"`
	Name          string      `json:"name"`
	EnvironmentID string      `json:"environment_id"`
	WorkerType    string      `json:"worker_type,omitempty"`
	MaxWorkers    int         `json:"max_workers,omitempty"`
	CreatedAt     *CustomTime `json:"created_at,omitempty"`
	UpdatedAt     *CustomTime `json:"updated_at,omitempty"`
}

// WorkerQueueCreateRequest represents the request to create a worker queue
type WorkerQueueCreateRequest struct {
	Name                    string  `json:"name"`
	EnvironmentID           string  `json:"environment_id"`
	WorkerType              *string `json:"worker_type,omitempty"`
	MaxWorkers              *int    `json:"max_workers,omitempty"`
	Ephemeral               *bool   `json:"ephemeral,omitempty"`
	SingleExecutionMode     *bool   `json:"single_execution_mode,omitempty"`
	AutoCleanupAfterSeconds *int    `json:"auto_cleanup_after_seconds,omitempty"`
	ParentExecutionID       *string `json:"parent_execution_id,omitempty"`
}

// WorkerQueueUpdateRequest represents the request to update a worker queue
type WorkerQueueUpdateRequest struct {
	Name       *string `json:"name,omitempty"`
	WorkerType *string `json:"worker_type,omitempty"`
	MaxWorkers *int    `json:"max_workers,omitempty"`
}

// Worker represents a worker in the control plane
type Worker struct {
	ID          string                 `json:"id,omitempty"`
	QueueID     string                 `json:"queue_id"`
	Status      string                 `json:"status"`
	LastSeen    *CustomTime            `json:"last_seen,omitempty"`
	WorkerInfo  map[string]interface{} `json:"worker_info,omitempty"`
	CreatedAt   *CustomTime            `json:"created_at,omitempty"`
}

// ============================================================================
// Auto-Update Entities
// ============================================================================

// WorkerQueueConfig represents worker queue configuration with version tracking
type WorkerQueueConfig struct {
	QueueID                    string   `json:"queue_id"`
	Name                       string   `json:"name"`
	DisplayName                *string  `json:"display_name,omitempty"`
	Description                *string  `json:"description,omitempty"`
	Status                     string   `json:"status"`
	MaxWorkers                 *int     `json:"max_workers,omitempty"`
	HeartbeatInterval          int      `json:"heartbeat_interval"`
	Tags                       []string `json:"tags"`
	Settings                   map[string]interface{} `json:"settings"`
	ConfigVersion              string   `json:"config_version"`               // SHA256 hash for change detection
	ConfigUpdatedAt            string   `json:"config_updated_at"`            // Timestamp of last config change
	RecommendedPackageVersion  *string  `json:"recommended_package_version"`  // Latest recommended worker package
	EnvironmentID              string   `json:"environment_id"`
	EnvironmentName            string   `json:"environment_name"`
}

// UpdateLockRequest represents a request to acquire an update lock
type UpdateLockRequest struct {
	WorkerID            string `json:"worker_id"`
	LockDurationSeconds int    `json:"lock_duration_seconds"`
}

// UpdateLock represents an acquired update lock
type UpdateLock struct {
	LockID     string `json:"lock_id"`
	WorkerID   string `json:"worker_id"`
	QueueID    string `json:"queue_id"`
	AcquiredAt string `json:"acquired_at"`
	ExpiresAt  string `json:"expires_at"`
	Locked     bool   `json:"locked"`
}

// UpdateLockStatus represents the current lock status for a queue
type UpdateLockStatus struct {
	Locked                    bool    `json:"locked"`
	QueueID                   string  `json:"queue_id,omitempty"`
	WorkerID                  *string `json:"worker_id,omitempty"`
	LockID                    *string `json:"lock_id,omitempty"`
	AcquiredAt                *string `json:"acquired_at,omitempty"`
	ExpiresAt                 *string `json:"expires_at,omitempty"`
	TTLSeconds                *int    `json:"ttl_seconds,omitempty"`
	LockCoordinationAvailable bool    `json:"lock_coordination_available"`
	Message                   *string `json:"message,omitempty"`
}
