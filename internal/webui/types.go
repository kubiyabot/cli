package webui

import "time"

// WorkerPoolOverview contains dashboard summary data
type WorkerPoolOverview struct {
	TotalWorkers    int           `json:"total_workers"`
	ActiveWorkers   int           `json:"active_workers"`
	IdleWorkers     int           `json:"idle_workers"`
	TasksProcessed  int64         `json:"tasks_processed"`
	TasksActive     int           `json:"tasks_active"`
	TasksFailed     int64         `json:"tasks_failed"`
	ErrorRate       float64       `json:"error_rate"`
	Uptime          time.Duration `json:"uptime"`
	UptimeFormatted string        `json:"uptime_formatted"`
	ControlPlaneOK  bool          `json:"control_plane_ok"`
	LiteLLMProxyOK  *bool         `json:"litellm_proxy_ok,omitempty"`
	StartTime       time.Time     `json:"start_time"`
}

// WorkerStatus represents the current state of a worker
type WorkerStatus string

const (
	WorkerStatusRunning      WorkerStatus = "running"
	WorkerStatusIdle         WorkerStatus = "idle"
	WorkerStatusBusy         WorkerStatus = "busy"
	WorkerStatusError        WorkerStatus = "error"
	WorkerStatusStarting     WorkerStatus = "starting"
	WorkerStatusStopping     WorkerStatus = "stopping"
	WorkerStatusDisconnected WorkerStatus = "disconnected"
)

// WorkerInfo contains information about a single worker
type WorkerInfo struct {
	ID            string       `json:"id"`
	QueueID       string       `json:"queue_id"`
	Status        WorkerStatus `json:"status"`
	PID           int          `json:"pid"`
	StartedAt     time.Time    `json:"started_at"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	TasksActive   int          `json:"tasks_active"`
	TasksTotal    int64        `json:"tasks_total"`
	Version       string       `json:"version"`
	Hostname      string       `json:"hostname"`
}

// WorkerDetail contains detailed worker information including metrics
type WorkerDetail struct {
	WorkerInfo
	Metrics     *WorkerMetrics `json:"metrics,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Capabilities []string         `json:"capabilities,omitempty"`
}

// WorkerMetrics contains resource usage metrics for a worker
type WorkerMetrics struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryMB      float64   `json:"memory_mb"`
	MemoryRSS     int64     `json:"memory_rss"`
	MemoryVMS     int64     `json:"memory_vms"`
	MemoryPercent float64   `json:"memory_percent"`
	OpenFiles     int       `json:"open_files"`
	Threads       int       `json:"threads"`
	CollectedAt   time.Time `json:"collected_at"`
}

// ControlPlaneStatus contains control plane connection information
type ControlPlaneStatus struct {
	Connected       bool          `json:"connected"`
	URL             string        `json:"url"`
	Latency         time.Duration `json:"latency"`
	LatencyMS       int64         `json:"latency_ms"`
	LastCheck       time.Time     `json:"last_check"`
	LastSuccess     time.Time     `json:"last_success"`
	AuthStatus      string        `json:"auth_status"` // valid, expired, error
	ConfigVersion   string        `json:"config_version,omitempty"`
	ErrorMessage    string        `json:"error_message,omitempty"`
	ReconnectCount  int           `json:"reconnect_count"`
}

// SessionType represents the type of an active session
type SessionType string

const (
	SessionTypeChat      SessionType = "chat"
	SessionTypeStreaming SessionType = "streaming"
	SessionTypeExecution SessionType = "execution"
)

// SessionStatus represents the current state of a session
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// SessionInfo contains information about an active session
type SessionInfo struct {
	ID            string        `json:"id"`
	Type          SessionType   `json:"type"`
	Status        SessionStatus `json:"status"`
	WorkerID      string        `json:"worker_id"`
	AgentID       string        `json:"agent_id,omitempty"`
	AgentName     string        `json:"agent_name,omitempty"`
	StartedAt     time.Time     `json:"started_at"`
	EndedAt       *time.Time    `json:"ended_at,omitempty"`
	Duration      time.Duration `json:"duration"`
	DurationStr   string        `json:"duration_str"`
	MessagesCount int           `json:"messages_count"`
	TokensUsed    int64         `json:"tokens_used,omitempty"`
}

// SessionDetail contains detailed session information
type SessionDetail struct {
	SessionInfo
	Messages []SessionMessage `json:"messages,omitempty"`
	Events   []SessionEvent   `json:"events,omitempty"`
}

// SessionMessage represents a message in a session
type SessionMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // user, assistant, system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionEvent represents an event in a session
type SessionEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// LogLevel represents log severity
type LogLevel string

const (
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       LogLevel  `json:"level"`
	Component   string    `json:"component"`
	Message     string    `json:"message"`
	WorkerID    string    `json:"worker_id,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// LogFilter contains log filtering options
type LogFilter struct {
	Level     LogLevel `json:"level,omitempty"`
	Component string   `json:"component,omitempty"`
	WorkerID  string   `json:"worker_id,omitempty"`
	Search    string   `json:"search,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

// SSEEventType represents the type of SSE event
type SSEEventType string

const (
	SSEEventWorkerUpdate    SSEEventType = "worker_update"
	SSEEventMetrics         SSEEventType = "metrics"
	SSEEventLog             SSEEventType = "log"
	SSEEventSession         SSEEventType = "session"
	SSEEventControlPlane    SSEEventType = "control_plane"
	SSEEventOverview        SSEEventType = "overview"
	SSEEventHeartbeat       SSEEventType = "heartbeat"
)

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Type SSEEventType `json:"type"`
	Data interface{}  `json:"data"`
}

// HealthStatus represents the health of a component
type HealthStatus struct {
	Status    string    `json:"status"` // ok, degraded, error
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// HealthResponse contains overall health information
type HealthResponse struct {
	Status     string                  `json:"status"`
	Uptime     time.Duration           `json:"uptime"`
	Components map[string]HealthStatus `json:"components"`
}

// WorkerConfig contains current worker configuration
type WorkerConfig struct {
	QueueID            string            `json:"queue_id"`
	QueueName          string            `json:"queue_name,omitempty"`
	DeploymentType     string            `json:"deployment_type"`
	ControlPlaneURL    string            `json:"control_plane_url"`
	EnableLocalProxy   bool              `json:"enable_local_proxy"`
	ProxyPort          int               `json:"proxy_port,omitempty"`
	ModelOverride      string            `json:"model_override,omitempty"`
	AutoUpdate         bool              `json:"auto_update"`
	DaemonMode         bool              `json:"daemon_mode"`
	WorkerDir          string            `json:"worker_dir"`
	Environment        map[string]string `json:"environment,omitempty"`
	// System info for UI display
	Version            string            `json:"version,omitempty"`
	BuildCommit        string            `json:"build_commit,omitempty"`
	BuildDate          string            `json:"build_date,omitempty"`
	GoVersion          string            `json:"go_version,omitempty"`
	OS                 string            `json:"os,omitempty"`
	Arch               string            `json:"arch,omitempty"`
}

// ActionResponse is returned by action endpoints
type ActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// MetricsSnapshot contains a point-in-time metrics snapshot
type MetricsSnapshot struct {
	Timestamp       time.Time      `json:"timestamp"`
	Workers         []WorkerMetrics `json:"workers"`
	TotalCPU        float64        `json:"total_cpu"`
	TotalMemoryMB   float64        `json:"total_memory_mb"`
	TasksPerSecond  float64        `json:"tasks_per_second"`
	ErrorsPerSecond float64        `json:"errors_per_second"`
}

// RecentActivity represents recent activity for the dashboard
type RecentActivity struct {
	Type        string    `json:"type"` // task_completed, task_failed, session_started, etc.
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	WorkerID    string    `json:"worker_id,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
}

// ============================================================================
// Doctor/Diagnostics Types
// ============================================================================

// DiagnosticStatus represents the result status of a diagnostic check
type DiagnosticStatus string

const (
	DiagnosticStatusPass    DiagnosticStatus = "pass"
	DiagnosticStatusFail    DiagnosticStatus = "fail"
	DiagnosticStatusWarning DiagnosticStatus = "warning"
	DiagnosticStatusSkip    DiagnosticStatus = "skip"
)

// DiagnosticCategory represents the category of diagnostic checks
type DiagnosticCategory string

const (
	DiagnosticCategoryPython       DiagnosticCategory = "python"
	DiagnosticCategoryPackages     DiagnosticCategory = "packages"
	DiagnosticCategoryConnectivity DiagnosticCategory = "connectivity"
	DiagnosticCategoryConfig       DiagnosticCategory = "config"
	DiagnosticCategoryProcess      DiagnosticCategory = "process"
)

// DiagnosticCheck represents a single diagnostic check result
type DiagnosticCheck struct {
	Name        string             `json:"name"`
	Category    DiagnosticCategory `json:"category"`
	Status      DiagnosticStatus   `json:"status"`
	Message     string             `json:"message"`
	Details     interface{}        `json:"details,omitempty"`
	DurationMS  int64              `json:"duration_ms"`
	Remediation string             `json:"remediation,omitempty"`
}

// DiagnosticSummary contains aggregated counts for diagnostics
type DiagnosticSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Warnings int `json:"warnings"`
	Skipped  int `json:"skipped"`
}

// DiagnosticsReport represents a complete diagnostics report
type DiagnosticsReport struct {
	Timestamp time.Time         `json:"timestamp"`
	Overall   string            `json:"overall"` // healthy, degraded, unhealthy
	Checks    []DiagnosticCheck `json:"checks"`
	Summary   DiagnosticSummary `json:"summary"`
}

// PythonInfo contains Python environment information
type PythonInfo struct {
	Version    string `json:"version"`
	Path       string `json:"path"`
	VenvPath   string `json:"venv_path,omitempty"`
	VenvActive bool   `json:"venv_active"`
	PipVersion string `json:"pip_version"`
}

// PackageInfo contains package version information
type PackageInfo struct {
	Name             string `json:"name"`
	InstalledVersion string `json:"installed_version,omitempty"`
	LatestVersion    string `json:"latest_version,omitempty"`
	RequiredVersion  string `json:"required_version,omitempty"`
	IsInstalled      bool   `json:"is_installed"`
	IsOutdated       bool   `json:"is_outdated"`
}

// ============================================================================
// LiteLLM Proxy Types
// ============================================================================

// ProxyStatus represents the current state of LiteLLM proxy
type ProxyStatus struct {
	Running      bool       `json:"running"`
	PID          int        `json:"pid,omitempty"`
	Port         int        `json:"port,omitempty"`
	BaseURL      string     `json:"base_url,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	HealthStatus string     `json:"health_status"` // healthy, unhealthy, unknown
	ConfigPath   string     `json:"config_path,omitempty"`
	Models       []string   `json:"models,omitempty"`
	LogFile      string     `json:"log_file,omitempty"`
}

// LangfuseConfig represents Langfuse integration settings
type LangfuseConfig struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"` // masked in responses
	Host      string `json:"host,omitempty"`
}

// ProxyControlRequest represents a proxy control action
type ProxyControlRequest struct {
	Action   string          `json:"action"` // start, stop, restart
	Config   interface{}     `json:"config,omitempty"`
	Langfuse *LangfuseConfig `json:"langfuse,omitempty"`
}

// ProxyLogsRequest contains proxy log request options
type ProxyLogsRequest struct {
	Lines  int    `json:"lines,omitempty"`
	Search string `json:"search,omitempty"`
	Level  string `json:"level,omitempty"`
}

// ============================================================================
// Environment Variables Types
// ============================================================================

// EnvVariableSource indicates where an env variable comes from
type EnvVariableSource string

const (
	EnvSourceCustom    EnvVariableSource = "custom"    // User-defined in .env
	EnvSourceInherited EnvVariableSource = "inherited" // From parent process
	EnvSourceSystem    EnvVariableSource = "system"    // System-level
	EnvSourceWorker    EnvVariableSource = "worker"    // Worker-specific
)

// EnvVariable represents an environment variable with metadata
type EnvVariable struct {
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Source    EnvVariableSource `json:"source"`
	Sensitive bool              `json:"sensitive"`
	Editable  bool              `json:"editable"`
}

// EnvUpdateRequest represents a request to update environment variables
type EnvUpdateRequest struct {
	Variables  map[string]string `json:"variables"`
	SaveToFile bool              `json:"save_to_file"`
}

// EnvUpdateResponse represents the result of an environment update
type EnvUpdateResponse struct {
	Success       bool     `json:"success"`
	Message       string   `json:"message,omitempty"`
	RestartNeeded bool     `json:"restart_needed"`
	UpdatedKeys   []string `json:"updated_keys,omitempty"`
	EnvFilePath   string   `json:"env_file_path,omitempty"`
}

// ============================================================================
// LLM Models/Providers Types
// ============================================================================

// ModelInfo represents an LLM model with capabilities
type ModelInfo struct {
	ID                 string                 `json:"id"`
	Value              string                 `json:"value"`
	Label              string                 `json:"label"`
	Provider           string                 `json:"provider"`
	Logo               string                 `json:"logo,omitempty"`
	Enabled            bool                   `json:"enabled"`
	Recommended        bool                   `json:"recommended"`
	Capabilities       map[string]interface{} `json:"capabilities,omitempty"`
	Pricing            map[string]interface{} `json:"pricing,omitempty"`
	CompatibleRuntimes []string               `json:"compatible_runtimes,omitempty"`
}

// ProviderStatus represents a provider's connectivity status
type ProviderStatus struct {
	Name       string `json:"name"`
	Connected  bool   `json:"connected"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
	Error      string `json:"error,omitempty"`
	ModelCount int    `json:"model_count"`
	Logo       string `json:"logo,omitempty"`
}

// LLMInsights represents combined LLM provider information
type LLMInsights struct {
	Providers    []ProviderStatus `json:"providers"`
	Models       []ModelInfo      `json:"models"`
	DefaultModel *ModelInfo       `json:"default_model,omitempty"`
	LastUpdated  time.Time        `json:"last_updated"`
	CachedUntil  time.Time        `json:"cached_until"`
}

// ModelTestRequest represents a request to test model connectivity
type ModelTestRequest struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt,omitempty"`
}

// ModelTestResponse represents the result of a model connectivity test
type ModelTestResponse struct {
	Success   bool   `json:"success"`
	ModelID   string `json:"model_id"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
	Response  string `json:"response,omitempty"`
}

// ============================================================================
// Multi-Worker Pool Types
// ============================================================================

// PoolStatus represents the overall worker pool status
type PoolStatus struct {
	TotalWorkers   int          `json:"total_workers"`
	ActiveWorkers  int          `json:"active_workers"`
	IdleWorkers    int          `json:"idle_workers"`
	SpawningCount  int          `json:"spawning_count"`
	MaxWorkers     int          `json:"max_workers"`
	CanScale       bool         `json:"can_scale"`
	ScaleLimit     int          `json:"scale_limit"`
	Workers        []WorkerInfo `json:"workers"`
}

// WorkerSpawnRequest represents a request to spawn a new worker
type WorkerSpawnRequest struct {
	QueueID     string            `json:"queue_id,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
}

// WorkerScaleRequest represents a pool scaling request
type WorkerScaleRequest struct {
	TargetCount int `json:"target_count"`
}

// ============================================================================
// Directory Types
// ============================================================================

// DirectoryInfo represents directory information
type DirectoryInfo struct {
	Path      string   `json:"path"`
	Exists    bool     `json:"exists"`
	Writable  bool     `json:"writable"`
	Size      int64    `json:"size"`
	FileCount int      `json:"file_count"`
	Files     []string `json:"files,omitempty"`
}

// DirectoryEntry represents a single directory entry
type DirectoryEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// DirectoryBrowseResponse represents directory contents
type DirectoryBrowseResponse struct {
	CurrentPath string           `json:"current_path"`
	ParentPath  string           `json:"parent_path"`
	Entries     []DirectoryEntry `json:"entries"`
}

// DirectoryUpdateRequest represents a directory change request
type DirectoryUpdateRequest struct {
	Path string `json:"path"`
}

// ============================================================================
// Additional SSE Event Types
// ============================================================================

const (
	SSEEventDiagnostic   SSEEventType = "diagnostic"
	SSEEventProxyStatus  SSEEventType = "proxy_status"
	SSEEventEnvUpdate    SSEEventType = "env_update"
	SSEEventPoolUpdate   SSEEventType = "pool_update"
	SSEEventModelUpdate  SSEEventType = "model_update"
)
