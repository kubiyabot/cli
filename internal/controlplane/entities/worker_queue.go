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
	Name          string  `json:"name"`
	EnvironmentID string  `json:"environment_id"`
	WorkerType    *string `json:"worker_type,omitempty"`
	MaxWorkers    *int    `json:"max_workers,omitempty"`
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
