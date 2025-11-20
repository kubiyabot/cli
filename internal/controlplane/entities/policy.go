package entities

// Policy represents an OPA policy in the control plane
type Policy struct {
	ID          string      `json:"id,omitempty"`
	Name        string      `json:"name"`
	Description *string     `json:"description,omitempty"`
	Rego        string      `json:"rego"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   *CustomTime `json:"created_at,omitempty"`
	UpdatedAt   *CustomTime `json:"updated_at,omitempty"`
}

// PolicyCreateRequest represents the request to create a policy
type PolicyCreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Rego        string  `json:"rego"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// PolicyUpdateRequest represents the request to update a policy
type PolicyUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Rego        *string `json:"rego,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// PolicyAssociation represents an association between a policy and an entity
type PolicyAssociation struct {
	ID         string      `json:"id,omitempty"`
	EntityType string      `json:"entity_type"` // agent, team, environment
	EntityID   string      `json:"entity_id"`
	PolicyID   string      `json:"policy_id"`
	Priority   int         `json:"priority,omitempty"`
	CreatedAt  *CustomTime `json:"created_at,omitempty"`
	UpdatedAt  *CustomTime `json:"updated_at,omitempty"`
}

// PolicyAssociationRequest represents a request to create a policy association
type PolicyAssociationRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	PolicyID   string `json:"policy_id"`
	Priority   *int   `json:"priority,omitempty"`
}

// PolicyAssociationUpdateRequest represents a request to update a policy association
type PolicyAssociationUpdateRequest struct {
	Priority *int `json:"priority,omitempty"`
}

// PolicyEvaluationRequest represents a request to evaluate policies
type PolicyEvaluationRequest struct {
	Input map[string]interface{} `json:"input"`
}

// PolicyEvaluationResponse represents the response from policy evaluation
type PolicyEvaluationResponse struct {
	Allowed bool                   `json:"allowed"`
	Reasons []string               `json:"reasons,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}
