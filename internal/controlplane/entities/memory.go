package entities

// Memory represents a stored memory in the context graph
type Memory struct {
	MemoryID  string                 `json:"memory_id"`
	Context   MemoryContext          `json:"context"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt *CustomTime            `json:"created_at,omitempty"`
	UpdatedAt *CustomTime            `json:"updated_at,omitempty"`
}

// MemoryContext represents the context of a memory
type MemoryContext struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}

// MemoryStoreRequest represents the request to store a new memory
type MemoryStoreRequest struct {
	DatasetID string                 `json:"dataset_id"`
	Context   MemoryContext          `json:"context"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryStoreResponse represents the response from storing a memory
type MemoryStoreResponse struct {
	MemoryID string  `json:"memory_id"`
	Status   string  `json:"status"`
	JobID    *string `json:"job_id,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// MemoryRecallRequest represents the request to recall memories
type MemoryRecallRequest struct {
	Query    string   `json:"query"`
	TopK     int      `json:"top_k,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	MinScore float64  `json:"min_score,omitempty"`
}

// MemorySearchResult represents a single memory search result
type MemorySearchResult struct {
	Memory Memory  `json:"memory"`
	Score  float64 `json:"score"`
}

// MemoryRecallResponse represents the response from recalling memories
type MemoryRecallResponse struct {
	Results []MemorySearchResult `json:"results"`
	Query   string               `json:"query,omitempty"`
	Count   int                  `json:"count"`
}

// MemoryJobStatus represents the status of an async memory job
type MemoryJobStatus struct {
	JobID      string  `json:"job_id"`
	Status     string  `json:"status"`
	Progress   float64 `json:"progress,omitempty"`
	Message    string  `json:"message,omitempty"`
	Error      string  `json:"error,omitempty"`
	CompletedAt *CustomTime `json:"completed_at,omitempty"`
}

// Dataset represents a cognitive dataset
type Dataset struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description,omitempty"`
	Scope        string      `json:"scope"` // user, org, role
	AllowedRoles []string    `json:"allowed_roles,omitempty"`
	CreatedBy    string      `json:"created_by,omitempty"`
	CreatedAt    *CustomTime `json:"created_at,omitempty"`
	UpdatedAt    *CustomTime `json:"updated_at,omitempty"`
}

// DatasetCreateRequest represents the request to create a dataset
type DatasetCreateRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Scope        string   `json:"scope"`
	AllowedRoles []string `json:"allowed_roles,omitempty"`
}

// DatasetDataEntry represents a data entry in a dataset
type DatasetDataEntry struct {
	ID        string                 `json:"id"`
	DatasetID string                 `json:"dataset_id"`
	Content   map[string]interface{} `json:"content"`
	CreatedAt *CustomTime            `json:"created_at,omitempty"`
}

// DatasetListResponse represents the response from listing datasets
type DatasetListResponse struct {
	Datasets []*Dataset `json:"datasets"`
	Total    int        `json:"total"`
}

// DatasetDataResponse represents the response from getting dataset data
type DatasetDataResponse struct {
	DatasetID string             `json:"dataset_id"`
	Data      []DatasetDataEntry `json:"data"`
	Count     int                `json:"count"`
}
