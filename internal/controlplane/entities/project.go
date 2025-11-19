package entities

// Project represents a project in the control plane
type Project struct {
	ID          string      `json:"id,omitempty"`
	Name        string      `json:"name"`
	Description *string     `json:"description,omitempty"`
	AgentIDs    []string    `json:"agent_ids,omitempty"`
	TeamIDs     []string    `json:"team_ids,omitempty"`
	CreatedAt   *CustomTime `json:"created_at,omitempty"`
	UpdatedAt   *CustomTime `json:"updated_at,omitempty"`
}

// ProjectCreateRequest represents the request to create a project
type ProjectCreateRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	AgentIDs    []string `json:"agent_ids,omitempty"`
	TeamIDs     []string `json:"team_ids,omitempty"`
}

// ProjectUpdateRequest represents the request to update a project
type ProjectUpdateRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	AgentIDs    []string `json:"agent_ids,omitempty"`
	TeamIDs     []string `json:"team_ids,omitempty"`
}
