package entities

// Runner represents a runner in the control plane
type Runner struct {
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Health    string                 `json:"health,omitempty"`
	LastSeen  *CustomTime            `json:"last_seen,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// RunnerHealth represents health information for a runner
type RunnerHealth struct {
	Healthy  bool   `json:"healthy"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}
