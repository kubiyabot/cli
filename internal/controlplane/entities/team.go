package entities

// Team represents a team in the control plane
type Team struct {
	ID             string      `json:"id,omitempty"`
	Name           string      `json:"name"`
	Description    *string     `json:"description,omitempty"`
	Members        []string    `json:"members,omitempty"` // Agent IDs
	EnvironmentIDs []string    `json:"environment_ids,omitempty"`
	TeamType       *string     `json:"team_type,omitempty"`
	CreatedAt      *CustomTime `json:"created_at,omitempty"`
	UpdatedAt      *CustomTime `json:"updated_at,omitempty"`
}

// TeamCreateRequest represents the request to create a team
type TeamCreateRequest struct {
	Name           string   `json:"name"`
	Description    *string  `json:"description,omitempty"`
	Members        []string `json:"members,omitempty"`
	EnvironmentIDs []string `json:"environment_ids,omitempty"`
	TeamType       *string  `json:"team_type,omitempty"`
}

// TeamUpdateRequest represents the request to update a team
type TeamUpdateRequest struct {
	Name           *string  `json:"name,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Members        []string `json:"members,omitempty"`
	EnvironmentIDs []string `json:"environment_ids,omitempty"`
	TeamType       *string  `json:"team_type,omitempty"`
}
