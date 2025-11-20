package entities

// Skill represents a skill/toolset in the control plane
type Skill struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	SkillType     string                 `json:"skill_type"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Variant       *string                `json:"variant,omitempty"`
	Enabled       bool                   `json:"enabled,omitempty"`
	CreatedAt     *CustomTime            `json:"created_at,omitempty"`
	UpdatedAt     *CustomTime            `json:"updated_at,omitempty"`
}

// SkillCreateRequest represents the request to create a skill
type SkillCreateRequest struct {
	Name          string                 `json:"name"`
	Description   *string                `json:"description,omitempty"`
	SkillType     string                 `json:"skill_type"`
	Configuration map[string]interface{} `json:"configuration"`
	Variant       *string                `json:"variant,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
}

// SkillUpdateRequest represents the request to update a skill
type SkillUpdateRequest struct {
	Name          *string                `json:"name,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Variant       *string                `json:"variant,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
}

// SkillDefinition represents a skill type definition
type SkillDefinition struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Variants    []string               `json:"variants,omitempty"`
}

// SkillAssociation represents an association between a skill and an entity
type SkillAssociation struct {
	ID         string      `json:"id,omitempty"`
	EntityType string      `json:"entity_type"` // agent, team, environment
	EntityID   string      `json:"entity_id"`
	SkillID    string      `json:"skill_id"`
	CreatedAt  *CustomTime `json:"created_at,omitempty"`
}

// SkillAssociationRequest represents a request to associate a skill with an entity
type SkillAssociationRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	SkillID    string `json:"skill_id"`
}
