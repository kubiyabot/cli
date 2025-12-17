package entities

// CodeFileMetadata represents metadata extracted from a code file
type CodeFileMetadata struct {
	FilePath     string   `json:"file_path"`
	Language     string   `json:"language"`
	SizeBytes    int      `json:"size_bytes"`
	LinesOfCode  int      `json:"lines_of_code"`
	Dependencies []string `json:"dependencies"`
	Exports      []string `json:"exports"`
	FileHash     string   `json:"file_hash"`
}

// CodeFileUpload represents a single code file with content and metadata
type CodeFileUpload struct {
	Content  string           `json:"content"`
	Metadata CodeFileMetadata `json:"metadata"`
}

// CodeIngestionConfig represents configuration for code ingestion
type CodeIngestionConfig struct {
	SourceType        string   `json:"source_type"` // "local" or "git"
	BasePath          string   `json:"base_path,omitempty"`
	IncludedPatterns  []string `json:"included_patterns,omitempty"`
	ExcludedPatterns  []string `json:"excluded_patterns,omitempty"`
	ExtractDependencies bool   `json:"extract_dependencies,omitempty"`
}

// CodeStreamSessionCreate represents request to start code ingestion session
type CodeStreamSessionCreate struct {
	SessionDurationMinutes int                 `json:"session_duration_minutes,omitempty"`
	Config                 CodeIngestionConfig `json:"config"`
}

// CodeStreamSession represents a streaming session response
type CodeStreamSession struct {
	ID         string      `json:"id"`
	DatasetID  string      `json:"dataset_id"`
	Status     string      `json:"status"` // "active", "committed", "expired"
	ExpiresAt  *CustomTime `json:"expires_at,omitempty"`
	CreatedAt  *CustomTime `json:"created_at,omitempty"`
}

// CodeStreamBatchRequest represents batch of code files to upload
type CodeStreamBatchRequest struct {
	SessionID string           `json:"session_id"`
	BatchID   string           `json:"batch_id"` // Idempotency key
	Files     []CodeFileUpload `json:"files"`
}

// BatchSummary represents summary of batch processing
type BatchSummary struct {
	Total     int `json:"total"`
	Processed int `json:"processed"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped,omitempty"`
}

// CodeBatchResponse represents response from batch upload
type CodeBatchResponse struct {
	Status  string       `json:"status"`
	Summary BatchSummary `json:"summary"`
	Message string       `json:"message,omitempty"`
}

// CodeCommitResponse represents response from session commit
type CodeCommitResponse struct {
	JobID            string `json:"job_id"`
	Status           string `json:"status"`
	ProcessedRecords int    `json:"processed_records,omitempty"`
	FailedRecords    int    `json:"failed_records,omitempty"`
	Message          string `json:"message,omitempty"`
}

// CodeJobStatus represents status of code ingestion job
type CodeJobStatus struct {
	JobID            string                 `json:"job_id"`
	Status           string                 `json:"status"` // "pending", "running", "completed", "failed", "partial"
	TotalFiles       int                    `json:"total_files,omitempty"`
	ProcessedFiles   int                    `json:"processed_files,omitempty"`
	FailedFiles      int                    `json:"failed_files,omitempty"`
	FilesByLanguage  map[string]int         `json:"files_by_language,omitempty"`
	CognifyStatus    string                 `json:"cognify_status,omitempty"`
	Errors           []map[string]interface{} `json:"errors,omitempty"`
	StartedAt        *CustomTime            `json:"started_at,omitempty"`
	CompletedAt      *CustomTime            `json:"completed_at,omitempty"`
}
