package entities

// Environment represents an environment in the control plane
type Environment struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Variables     map[string]string      `json:"variables,omitempty"`
	Secrets       []string               `json:"secrets,omitempty"`
	Integrations  []string               `json:"integrations,omitempty"`
	CreatedAt     *CustomTime            `json:"created_at,omitempty"`
	UpdatedAt     *CustomTime            `json:"updated_at,omitempty"`
}

// EnvironmentCreateRequest represents the request to create an environment
type EnvironmentCreateRequest struct {
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Variables     map[string]string      `json:"variables,omitempty"`
	Secrets       []string               `json:"secrets,omitempty"`
	Integrations  []string               `json:"integrations,omitempty"`
}

// EnvironmentUpdateRequest represents the request to update an environment
type EnvironmentUpdateRequest struct {
	Name          *string                `json:"name,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Variables     map[string]string      `json:"variables,omitempty"`
	Secrets       []string               `json:"secrets,omitempty"`
	Integrations  []string               `json:"integrations,omitempty"`
}
